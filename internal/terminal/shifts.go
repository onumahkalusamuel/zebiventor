package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// A shift is one till session: opened with a cash float, closed with a
// cash count. Selling and refunds require an open shift so that every cash
// movement belongs to exactly one drawer reconciliation.

// CurrentShift returns the open shift, or nil.
func (t *Terminal) CurrentShift(ctx context.Context) (*protocol.Shift, error) {
	id := t.kv(ctx, "shift_id")
	if id == "" {
		return nil, nil
	}
	return t.GetShift(ctx, id)
}

// GetShift loads a shift with its (live or final) summary.
func (t *Terminal) GetShift(ctx context.Context, id string) (*protocol.Shift, error) {
	s := &protocol.Shift{}
	var summary string
	err := t.db.R().QueryRowContext(ctx, `SELECT id,status,opened_by,opened_by_name,opened_at,opening_float_minor,closed_by,closed_by_name,closed_at,
		counted_cash_minor,note,summary FROM shifts WHERE id=?`, id).
		Scan(&s.ID, &s.Status, &s.OpenedBy, &s.OpenedByName, &s.OpenedAt, &s.OpeningFloatMinor, &s.ClosedBy, &s.ClosedByName, &s.ClosedAt,
			&s.CountedCashMinor, &s.Note, &summary)
	if err != nil {
		return nil, err
	}
	p, _ := t.Pairing(ctx)
	s.TerminalID, s.LocationID = p.TerminalID, p.LocationID
	if s.Status == "closed" && summary != "" {
		s.Summary = &protocol.ShiftSummary{}
		_ = json.Unmarshal([]byte(summary), s.Summary)
	} else {
		s.Summary, err = t.shiftSummary(ctx, t.db.R(), s)
	}
	return s, err
}

// OpenShift starts a shift with the cash already in the drawer.
func (t *Terminal) OpenShift(ctx context.Context, u User, floatMinor int64) (*protocol.Shift, error) {
	if floatMinor < 0 {
		return nil, saleErr("the float cannot be negative")
	}
	s := &protocol.Shift{ID: ids.New(), Status: "open", OpenedBy: u.ID, OpenedByName: u.Name, OpeningFloatMinor: floatMinor}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var cur string
		tx.QueryRowContext(ctx, "SELECT value FROM kv WHERE key='shift_id'").Scan(&cur)
		if cur != "" {
			return saleErr("a shift is already open on this till")
		}
		s.OpenedAt = ids.NowMs()
		if _, err := tx.ExecContext(ctx, `INSERT INTO shifts(id,status,opened_by,opened_by_name,opened_at,opening_float_minor) VALUES(?,?,?,?,?,?)`,
			s.ID, s.Status, s.OpenedBy, s.OpenedByName, s.OpenedAt, s.OpeningFloatMinor); err != nil {
			return err
		}
		if err := setKV(ctx, tx, "shift_id", s.ID); err != nil {
			return err
		}
		return enqueue(ctx, tx, protocol.OpShiftOpen, s.ID, s, s.OpenedAt)
	})
	if err != nil {
		return nil, err
	}
	t.Nudge()
	return t.GetShift(ctx, s.ID)
}

// AddCashMovement records cash put into (paid in) or taken out of
// (paid out, e.g. petty cash, bank drop) the drawer.
func (t *Terminal) AddCashMovement(ctx context.Context, u User, approver *User, kind string, amount int64, reason string) (*protocol.CashMovement, error) {
	if kind != protocol.CashPaidIn && kind != protocol.CashPaidOut {
		return nil, saleErr("unknown cash movement")
	}
	if amount <= 0 {
		return nil, saleErr("enter an amount")
	}
	if strings.TrimSpace(reason) == "" {
		return nil, saleErr("give a reason")
	}
	m := &protocol.CashMovement{ID: ids.New(), Kind: kind, AmountMinor: amount, Reason: strings.TrimSpace(reason), UserID: u.ID, UserName: u.Name}
	if kind == protocol.CashPaidOut {
		by, err := t.authorise(ctx, u, approver, ApprovePaidOut)
		if err != nil {
			return nil, err
		}
		m.ApprovedBy = by
	}
	m.ShiftID = t.kv(ctx, "shift_id")
	if m.ShiftID == "" {
		return nil, saleErr("open a shift first")
	}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		m.CreatedAt = ids.NowMs()
		if _, err := tx.ExecContext(ctx, `INSERT INTO cash_movements(id,shift_id,kind,amount_minor,reason,user_id,user_name,approved_by,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
			m.ID, m.ShiftID, m.Kind, m.AmountMinor, m.Reason, m.UserID, m.UserName, m.ApprovedBy, m.CreatedAt); err != nil {
			return err
		}
		return enqueue(ctx, tx, protocol.OpCashMovement, m.ID, m, m.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	t.Nudge()
	return m, nil
}

// CashMovements lists the movements of a shift.
func (t *Terminal) CashMovements(ctx context.Context, shiftID string) ([]protocol.CashMovement, error) {
	rows, err := t.db.R().QueryContext(ctx, `SELECT id,shift_id,kind,amount_minor,reason,user_id,user_name,approved_by,created_at FROM cash_movements
		WHERE shift_id=? ORDER BY created_at`, shiftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.CashMovement{}
	for rows.Next() {
		var m protocol.CashMovement
		if err := rows.Scan(&m.ID, &m.ShiftID, &m.Kind, &m.AmountMinor, &m.Reason, &m.UserID, &m.UserName, &m.ApprovedBy, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CloseShift counts the drawer, freezes the Z report and ends the shift.
func (t *Terminal) CloseShift(ctx context.Context, u User, countedMinor int64, note string) (*protocol.Shift, error) {
	if countedMinor < 0 {
		return nil, saleErr("the counted cash cannot be negative")
	}
	id := t.kv(ctx, "shift_id")
	if id == "" {
		return nil, saleErr("there is no open shift")
	}
	var closed *protocol.Shift
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		s := &protocol.Shift{}
		if err := tx.QueryRowContext(ctx, `SELECT id,status,opened_by,opened_by_name,opened_at,opening_float_minor FROM shifts WHERE id=?`, id).
			Scan(&s.ID, &s.Status, &s.OpenedBy, &s.OpenedByName, &s.OpenedAt, &s.OpeningFloatMinor); err != nil {
			return err
		}
		sum, err := t.shiftSummary(ctx, tx, s)
		if err != nil {
			return err
		}
		sum.Closed = true
		sum.CountedCash = countedMinor
		sum.Variance = countedMinor - sum.ExpectedCash
		s.Status, s.ClosedBy, s.ClosedByName, s.ClosedAt = "closed", u.ID, u.Name, ids.NowMs()
		s.CountedCashMinor, s.Note, s.Summary = countedMinor, strings.TrimSpace(note), sum
		b, _ := json.Marshal(sum)
		if _, err := tx.ExecContext(ctx, `UPDATE shifts SET status='closed', closed_by=?, closed_by_name=?, closed_at=?, counted_cash_minor=?, note=?, summary=? WHERE id=?`,
			s.ClosedBy, s.ClosedByName, s.ClosedAt, s.CountedCashMinor, s.Note, string(b), s.ID); err != nil {
			return err
		}
		if err := setKV(ctx, tx, "shift_id", ""); err != nil {
			return err
		}
		closed = s
		return enqueue(ctx, tx, protocol.OpShiftClose, s.ID, s, s.ClosedAt)
	})
	if err != nil {
		return nil, err
	}
	t.Nudge()
	p, _ := t.Pairing(ctx)
	closed.TerminalID, closed.LocationID = p.TerminalID, p.LocationID
	return closed, nil
}

// ListShifts returns recent shifts, newest first.
func (t *Terminal) ListShifts(ctx context.Context, limit int) ([]protocol.Shift, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT id FROM shifts ORDER BY opened_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var idList []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		idList = append(idList, id)
	}
	rows.Close()
	out := []protocol.Shift{}
	for _, id := range idList {
		s, err := t.GetShift(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, nil
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// shiftSummary computes the X report of a shift from local data. Every
// figure here comes from this till only, so it is exact even offline.
func (t *Terminal) shiftSummary(ctx context.Context, q querier, s *protocol.Shift) (*protocol.ShiftSummary, error) {
	sum := &protocol.ShiftSummary{OpeningFloat: s.OpeningFloatMinor}
	var change int64
	err := q.QueryRowContext(ctx, `SELECT count(*), COALESCE(sum(subtotal_minor),0), COALESCE(sum(discount_minor),0), COALESCE(sum(tax_minor),0),
		COALESCE(sum(total_minor),0), COALESCE(sum(change_minor),0) FROM sales WHERE shift_id=?`, s.ID).
		Scan(&sum.SalesCount, &sum.GrossMinor, &sum.DiscountMinor, &sum.TaxMinor, &sum.NetSalesMinor, &change)
	if err != nil {
		return nil, err
	}
	q.QueryRowContext(ctx, `SELECT COALESCE(sum(l.qty_milli),0) FROM sale_lines l JOIN sales s ON s.id=l.sale_id WHERE s.shift_id=?`, s.ID).Scan(&sum.ItemsMilli)
	q.QueryRowContext(ctx, `SELECT count(*), COALESCE(sum(total_minor),0) FROM returns WHERE shift_id=?`, s.ID).Scan(&sum.ReturnsCount, &sum.ReturnsMinor)

	methods := map[string]*protocol.MethodTotal{}
	get := func(name, kind string) *protocol.MethodTotal {
		m, ok := methods[name]
		if !ok {
			m = &protocol.MethodTotal{MethodName: name, Kind: kind}
			methods[name] = m
		}
		return m
	}
	rows, err := q.QueryContext(ctx, `SELECT p.method_name, p.kind, sum(p.amount_minor) FROM sale_payments p JOIN sales s ON s.id=p.sale_id
		WHERE s.shift_id=? GROUP BY p.method_name, p.kind`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, kind string
		var amt int64
		rows.Scan(&name, &kind, &amt)
		get(name, kind).SalesMinor += amt
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT f.method_name, f.kind, sum(f.amount_minor) FROM return_refunds f JOIN returns r ON r.id=f.return_id
		WHERE r.shift_id=? GROUP BY f.method_name, f.kind`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, kind string
		var amt int64
		rows.Scan(&name, &kind, &amt)
		get(name, kind).RefundsMinor += amt
	}
	rows.Close()

	rows, err = q.QueryContext(ctx, `SELECT method_name, kind, count(*), sum(amount_minor) FROM account_payments WHERE shift_id=? GROUP BY method_name, kind`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, kind string
		var n, amt int64
		rows.Scan(&name, &kind, &n, &amt)
		get(name, kind).PaymentsMinor += amt
		sum.AccountCount += n
		sum.AccountMinor += amt
		if kind == protocol.PayCash {
			sum.CashAccount += amt
		}
	}
	rows.Close()

	// Change is handed out in cash: net it off the (first) cash method.
	if change > 0 {
		var cashName string
		for name, m := range methods {
			if m.Kind == protocol.PayCash && (cashName == "" || name < cashName) {
				cashName = name
			}
		}
		if cashName == "" {
			cashName = "Cash"
		}
		get(cashName, protocol.PayCash).SalesMinor -= change
	}
	for _, m := range methods {
		m.NetMinor = m.SalesMinor - m.RefundsMinor + m.PaymentsMinor
		if m.Kind == protocol.PayCash {
			sum.CashSales += m.SalesMinor
			sum.CashRefunds += m.RefundsMinor
		}
		sum.ByMethod = append(sum.ByMethod, *m)
	}
	sort.Slice(sum.ByMethod, func(i, j int) bool { return sum.ByMethod[i].NetMinor > sum.ByMethod[j].NetMinor })

	q.QueryRowContext(ctx, `SELECT COALESCE(sum(CASE WHEN kind='paid_in' THEN amount_minor END),0), COALESCE(sum(CASE WHEN kind='paid_out' THEN amount_minor END),0)
		FROM cash_movements WHERE shift_id=?`, s.ID).Scan(&sum.PaidIn, &sum.PaidOut)
	sum.ExpectedCash = sum.OpeningFloat + sum.CashSales - sum.CashRefunds + sum.CashAccount + sum.PaidIn - sum.PaidOut
	return sum, nil
}

// ---------- held (parked) sales ----------

// HeldSale is a parked cart.
type HeldSale struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	UserName   string `json:"user_name"`
	ItemCount  int64  `json:"item_count"`
	TotalMinor int64  `json:"total_minor"`
	Payload    string `json:"payload"` // opaque UI cart state
	CreatedAt  int64  `json:"created_at"`
}

// HoldSale parks a cart so the till can serve someone else.
func (t *Terminal) HoldSale(ctx context.Context, u User, h HeldSale) (string, error) {
	if h.Payload == "" {
		return "", saleErr("nothing to hold")
	}
	h.ID, h.UserName, h.CreatedAt = ids.New(), u.Name, ids.NowMs()
	return h.ID, t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO held_sales(id,label,user_name,item_count,total_minor,payload,created_at) VALUES(?,?,?,?,?,?,?)`,
			h.ID, strings.TrimSpace(h.Label), h.UserName, h.ItemCount, h.TotalMinor, h.Payload, h.CreatedAt)
		return err
	})
}

// HeldSales lists parked carts, oldest first.
func (t *Terminal) HeldSales(ctx context.Context) ([]HeldSale, error) {
	rows, err := t.db.R().QueryContext(ctx, `SELECT id,label,user_name,item_count,total_minor,payload,created_at FROM held_sales ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HeldSale{}
	for rows.Next() {
		var h HeldSale
		if err := rows.Scan(&h.ID, &h.Label, &h.UserName, &h.ItemCount, &h.TotalMinor, &h.Payload, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// TakeHeldSale removes a parked cart and returns it for recall.
func (t *Terminal) TakeHeldSale(ctx context.Context, id string) (*HeldSale, error) {
	h := &HeldSale{}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT id,label,user_name,item_count,total_minor,payload,created_at FROM held_sales WHERE id=?`, id).
			Scan(&h.ID, &h.Label, &h.UserName, &h.ItemCount, &h.TotalMinor, &h.Payload, &h.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return saleErr("that held sale was already recalled")
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM held_sales WHERE id=?", id)
		return err
	})
	return h, err
}

// ---------- customers ----------

// SearchCustomers finds customers by name, phone or email.
func (t *Terminal) SearchCustomers(ctx context.Context, query string, limit int) ([]protocol.Customer, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	where := []string{"1=1"}
	args := []any{}
	for _, w := range strings.Fields(strings.ToLower(query)) {
		where = append(where, `search LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(w)+"%")
	}
	args = append(args, limit)
	rows, err := t.db.R().QueryContext(ctx, `SELECT id,name,phone,email,notes,active,balance_minor+`+pendingBalance+`,updated_at FROM customers WHERE `+
		strings.Join(where, " AND ")+` ORDER BY active DESC, name COLLATE NOCASE LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.Customer{}
	for rows.Next() {
		var c protocol.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.Active, &c.BalanceMinor, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func customerSearch(c protocol.Customer) string {
	return strings.ToLower(strings.Join([]string{c.Name, c.Phone, c.Email}, " "))
}

// SaveCustomer creates or edits a customer locally (works offline) and
// queues it for the hub, which shares it with every till.
func (t *Terminal) SaveCustomer(ctx context.Context, c protocol.Customer) (*protocol.Customer, error) {
	c.Name, c.Phone, c.Email = strings.TrimSpace(c.Name), strings.TrimSpace(c.Phone), strings.TrimSpace(c.Email)
	if c.Name == "" {
		return nil, saleErr("customer name is required")
	}
	if c.ID == "" {
		c.ID, c.Active = ids.New(), true
	}
	c.UpdatedAt = ids.NowMs()
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if c.Phone != "" {
			var other string
			tx.QueryRowContext(ctx, "SELECT name FROM customers WHERE phone=? AND id<>? AND active=1", c.Phone, c.ID).Scan(&other)
			if other != "" {
				return saleErr("%s already has phone number %s", other, c.Phone)
			}
		}
		tx.QueryRowContext(ctx, "SELECT balance_minor FROM customers WHERE id=?", c.ID).Scan(&c.BalanceMinor)
		if _, err := tx.ExecContext(ctx, `INSERT INTO customers(id,name,phone,email,notes,active,balance_minor,updated_at,search) VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, phone=excluded.phone, email=excluded.email, notes=excluded.notes,
			active=excluded.active, updated_at=excluded.updated_at, search=excluded.search`,
			c.ID, c.Name, c.Phone, c.Email, c.Notes, boolInt(c.Active), c.BalanceMinor, c.UpdatedAt, customerSearch(c)); err != nil {
			return err
		}
		return enqueue(ctx, tx, protocol.OpCustomerUpsert, c.ID, c, c.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	t.Nudge()
	return &c, nil
}
