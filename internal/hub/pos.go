package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// ledgerEntry is one change to what a customer owes.
type ledgerEntry struct {
	CustomerID, Kind, RefID, RefNo, Note string
	UserID, UserName, TerminalID         string
	Amount, At                           int64
}

// postLedger changes what a customer owes on account, records it on the
// customer's statement and publishes the new balance to all tills.
func postLedger(ctx context.Context, tx *sql.Tx, e ledgerEntry) error {
	if e.Amount == 0 {
		return nil
	}
	if e.CustomerID == "" {
		audit(ctx, tx, e.UserID, e.TerminalID, e.Kind+".credit_without_customer", e.Kind, e.RefID, map[string]any{"amount": e.Amount})
		return nil
	}
	res, err := tx.ExecContext(ctx, "UPDATE customers SET balance_minor=balance_minor+? WHERE id=?", e.Amount, e.CustomerID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		audit(ctx, tx, e.UserID, e.TerminalID, e.Kind+".unknown_customer", e.Kind, e.RefID, map[string]any{"customer_id": e.CustomerID, "amount": e.Amount})
		return nil
	}
	if e.At == 0 {
		e.At = ids.NowMs()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO customer_ledger(id,customer_id,at,kind,ref_id,ref_no,amount_minor,note,user_name) VALUES(?,?,?,?,?,?,?,?,?)`,
		ids.New(), e.CustomerID, e.At, e.Kind, e.RefID, e.RefNo, e.Amount, e.Note, e.UserName); err != nil {
		return err
	}
	return recordChange(ctx, tx, protocol.EntityCustomer, e.CustomerID)
}

// applyReturn records a refund. Like sales, a return that reaches the hub
// has already happened (money left the drawer), so it is never refused for
// business reasons; over-returns are flagged in the audit log.
func (h *Hub) applyReturn(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var r protocol.Return
	if err := json.Unmarshal(op.Payload, &r); err != nil {
		return invalid("bad return payload: %v", err)
	}
	if r.ID == "" || r.ReturnNo == "" || len(r.Lines) == 0 {
		return invalid("return is missing id, number or lines")
	}
	if !strings.HasPrefix(r.ReturnNo, t.ReceiptPrefix+"-") {
		return invalid("return number %q does not belong to terminal %s", r.ReturnNo, t.ReceiptPrefix)
	}
	var exists int
	tx.QueryRowContext(ctx, "SELECT count(*) FROM returns WHERE id=?", r.ID).Scan(&exists)
	if exists > 0 {
		return nil
	}
	now := ids.NowMs()
	loc := r.LocationID
	var ok int
	tx.QueryRowContext(ctx, "SELECT count(*) FROM locations WHERE id=?", loc).Scan(&ok)
	if ok == 0 {
		loc = t.LocationID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO returns(id,return_no,sale_id,sale_receipt_no,terminal_id,location_id,user_id,user_name,approved_by,
		shift_id,customer_id,reason,tax_minor,total_minor,created_at,received_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.ReturnNo, r.SaleID, r.SaleReceiptNo, t.ID, loc, r.UserID, r.UserName, r.ApprovedBy, r.ShiftID, r.CustomerID,
		r.Reason, r.TaxMinor, r.TotalMinor, r.CreatedAt, now); err != nil {
		return err
	}
	for _, l := range r.Lines {
		if l.ID == "" {
			l.ID = ids.New()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO return_lines(id,return_id,sale_line_id,variant_id,product_name,variant_name,sku,qty_milli,tax_minor,total_minor,restock)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, l.ID, r.ID, l.SaleLineID, l.VariantID, l.ProductName, l.VariantName, l.SKU, l.QtyMilli, l.TaxMinor, l.TotalMinor, l.Restock); err != nil {
			return err
		}
		// Flag returns exceeding what was sold on that line.
		if l.SaleLineID != "" {
			var sold, returned int64
			tx.QueryRowContext(ctx, "SELECT qty_milli FROM sale_lines WHERE id=?", l.SaleLineID).Scan(&sold)
			tx.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_milli),0) FROM return_lines WHERE sale_line_id=?", l.SaleLineID).Scan(&returned)
			if sold > 0 && returned > sold {
				audit(ctx, tx, r.UserID, t.ID, "return.over_returned", "sale_line", l.SaleLineID, map[string]any{"sold": sold, "returned": returned, "return": r.ReturnNo})
			}
		}
		if !l.Restock {
			continue
		}
		var track bool
		err := tx.QueryRowContext(ctx, `SELECT p.track_stock FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, l.VariantID).Scan(&track)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !track) {
			continue
		}
		if err != nil {
			return err
		}
		if err := addStock(ctx, tx, stockMove{VariantID: l.VariantID, LocationID: loc, DeltaMilli: l.QtyMilli, Reason: "return",
			RefType: "return", RefID: r.ID, UserID: r.UserID, TerminalID: t.ID, CreatedAt: r.CreatedAt}); err != nil {
			return err
		}
	}
	var onAccount int64
	for _, p := range r.Refunds {
		if p.ID == "" {
			p.ID = ids.New()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO return_refunds(id,return_id,method_id,method_name,kind,amount_minor,reference) VALUES(?,?,?,?,?,?,?)`,
			p.ID, r.ID, p.MethodID, p.MethodName, p.Kind, p.AmountMinor, p.Reference); err != nil {
			return err
		}
		if p.Kind == protocol.PayCredit {
			onAccount += p.AmountMinor
		}
	}
	return postLedger(ctx, tx, ledgerEntry{CustomerID: r.CustomerID, Kind: "return", RefID: r.ID, RefNo: r.ReturnNo, Amount: -onAccount,
		At: r.CreatedAt, UserID: r.UserID, UserName: r.UserName, TerminalID: t.ID})
}

func (h *Hub) applyShiftOpen(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var s protocol.Shift
	if err := json.Unmarshal(op.Payload, &s); err != nil || s.ID == "" {
		return invalid("bad shift payload")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO shifts(id,terminal_id,location_id,status,opened_by,opened_by_name,opened_at,opening_float_minor,received_at)
		VALUES(?,?,?,'open',?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		s.ID, t.ID, t.LocationID, s.OpenedBy, s.OpenedByName, s.OpenedAt, s.OpeningFloatMinor, ids.NowMs())
	return err
}

func (h *Hub) applyShiftClose(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var s protocol.Shift
	if err := json.Unmarshal(op.Payload, &s); err != nil || s.ID == "" {
		return invalid("bad shift payload")
	}
	summary, _ := json.Marshal(s.Summary)
	var expected, variance int64
	if s.Summary != nil {
		expected, variance = s.Summary.ExpectedCash, s.Summary.Variance
	}
	// Upsert: the open op may have been lost if the till was reinstalled.
	_, err := tx.ExecContext(ctx, `INSERT INTO shifts(id,terminal_id,location_id,status,opened_by,opened_by_name,opened_at,opening_float_minor,
		closed_by,closed_by_name,closed_at,counted_cash_minor,expected_cash_minor,variance_minor,note,summary,received_at)
		VALUES(?,?,?,'closed',?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET status='closed', closed_by=excluded.closed_by, closed_by_name=excluded.closed_by_name,
		closed_at=excluded.closed_at, counted_cash_minor=excluded.counted_cash_minor, expected_cash_minor=excluded.expected_cash_minor,
		variance_minor=excluded.variance_minor, note=excluded.note, summary=excluded.summary`,
		s.ID, t.ID, t.LocationID, s.OpenedBy, s.OpenedByName, s.OpenedAt, s.OpeningFloatMinor,
		s.ClosedBy, s.ClosedByName, s.ClosedAt, s.CountedCashMinor, expected, variance, s.Note, string(summary), ids.NowMs())
	if err == nil && variance != 0 {
		audit(ctx, tx, s.ClosedBy, t.ID, "shift.cash_variance", "shift", s.ID, map[string]any{"expected": expected, "counted": s.CountedCashMinor, "variance": variance})
	}
	return err
}

func (h *Hub) applyCashMovement(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var m protocol.CashMovement
	if err := json.Unmarshal(op.Payload, &m); err != nil || m.ID == "" {
		return invalid("bad cash movement payload")
	}
	if m.Kind != protocol.CashPaidIn && m.Kind != protocol.CashPaidOut {
		return invalid("unknown cash movement %q", m.Kind)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO cash_movements(id,shift_id,terminal_id,kind,amount_minor,reason,user_id,user_name,approved_by,created_at,received_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		m.ID, m.ShiftID, t.ID, m.Kind, m.AmountMinor, m.Reason, m.UserID, m.UserName, m.ApprovedBy, m.CreatedAt, ids.NowMs())
	return err
}

// applyCustomer upserts customer details, last write wins by updated_at.
// The balance is never taken from a till.
func (h *Hub) applyCustomer(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var c protocol.Customer
	if err := json.Unmarshal(op.Payload, &c); err != nil || c.ID == "" {
		return invalid("bad customer payload")
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return invalid("customer name is required")
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = ids.NowMs()
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO customers(id,name,phone,email,notes,active,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, phone=excluded.phone, email=excluded.email, notes=excluded.notes,
		active=excluded.active, updated_at=excluded.updated_at WHERE excluded.updated_at >= customers.updated_at`,
		c.ID, c.Name, c.Phone, c.Email, c.Notes, c.Active, c.UpdatedAt, c.UpdatedAt)
	if err != nil {
		return err
	}
	_ = res // a stale edit changes nothing; publishing anyway lets the sender converge
	return recordChange(ctx, tx, protocol.EntityCustomer, c.ID)
}

// LookupSale finds a sale by receipt number (from any till) with the
// quantities already returned per line.
func (h *Hub) LookupSale(ctx context.Context, receiptNo string) (*protocol.SaleLookup, error) {
	r := h.db.R()
	var s protocol.Sale
	err := r.QueryRowContext(ctx, `SELECT s.id,s.receipt_no,s.terminal_id,s.location_id,s.user_id,s.user_name,s.customer_id,COALESCE(c.name,''),s.status,
		s.subtotal_minor,s.discount_minor,s.tax_minor,s.total_minor,s.paid_minor,s.change_minor,s.note,s.shift_id,s.approved_by,s.tax_mode,s.created_at
		FROM sales s LEFT JOIN customers c ON c.id=s.customer_id WHERE s.receipt_no=? OR s.id=?`, strings.TrimSpace(receiptNo), strings.TrimSpace(receiptNo)).
		Scan(&s.ID, &s.ReceiptNo, &s.TerminalID, &s.LocationID, &s.UserID, &s.UserName, &s.CustomerID, &s.CustomerName, &s.Status,
			&s.SubtotalMinor, &s.DiscountMinor, &s.TaxMinor, &s.TotalMinor, &s.PaidMinor, &s.ChangeMinor, &s.Note, &s.ShiftID, &s.ApprovedBy, &s.TaxMode, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("no sale with receipt number %q", receiptNo)
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.QueryContext(ctx, `SELECT l.id,l.line_no,l.variant_id,l.product_name,l.variant_name,l.sku,l.qty_milli,l.unit_price_minor,l.discount_minor,
		l.tax_minor,l.total_minor,l.original_price_minor,l.tax_rate_bp,COALESCE(p.track_stock,0)
		FROM sale_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE l.sale_id=? ORDER BY l.line_no`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.SaleLine
		if err := rows.Scan(&l.ID, &l.LineNo, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli, &l.UnitPriceMinor, &l.DiscountMinor,
			&l.TaxMinor, &l.TotalMinor, &l.OriginalPriceMinor, &l.TaxRateBP, &l.TrackStock); err != nil {
			rows.Close()
			return nil, err
		}
		s.Lines = append(s.Lines, l)
	}
	rows.Close()
	rows, err = r.QueryContext(ctx, `SELECT id,method_id,method_name,kind,amount_minor,reference FROM sale_payments WHERE sale_id=?`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p protocol.SalePayment
		rows.Scan(&p.ID, &p.MethodID, &p.MethodName, &p.Kind, &p.AmountMinor, &p.Reference)
		s.Payments = append(s.Payments, p)
	}
	rows.Close()

	out := &protocol.SaleLookup{Sale: s, Returned: map[string]int64{}}
	rows, err = r.QueryContext(ctx, `SELECT rl.sale_line_id, sum(rl.qty_milli) FROM return_lines rl JOIN returns rt ON rt.id=rl.return_id
		WHERE rt.sale_id=? GROUP BY rl.sale_line_id`, s.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var q int64
		rows.Scan(&id, &q)
		out.Returned[id] = q
	}
	return out, rows.Err()
}
