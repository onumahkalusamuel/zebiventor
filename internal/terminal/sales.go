package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// CartLine is one line of a sale as entered at the till.
type CartLine struct {
	VariantID string `json:"variant_id"`
	QtyMilli  int64  `json:"qty_milli"`
	// PriceOverride replaces the catalogue unit price (needs a manager).
	PriceOverride *int64 `json:"price_override,omitempty"`
}

// PaymentInput is one tender.
type PaymentInput struct {
	MethodID    string `json:"method_id"`
	AmountMinor int64  `json:"amount_minor"`
	Reference   string `json:"reference"`
}

// SaleInput is what the POS screen submits.
type SaleInput struct {
	Lines         []CartLine     `json:"lines"`
	Payments      []PaymentInput `json:"payments"`
	DiscountMinor int64          `json:"discount_minor"`
	CustomerID    string         `json:"customer_id"`
	Note          string         `json:"note"`
	ApprovalToken string         `json:"approval_token"`
}

// SaleError is a problem the cashier must fix before the sale can complete.
type SaleError struct{ Msg string }

func (e SaleError) Error() string { return e.Msg }

func saleErr(format string, a ...any) error { return SaleError{fmt.Sprintf(format, a...)} }

// ErrApprovalRequired means a manager must authorise the action.
var ErrApprovalRequired = SaleError{"manager approval required"}

// IsManager reports whether the role can authorise discounts, overrides and refunds.
func IsManager(role string) bool { return role == protocol.RoleAdmin || role == protocol.RoleManager }

// Actions that may need a manager, each switchable in store settings.
const (
	ApproveDiscount      = "discount"
	ApprovePriceOverride = "price_override"
	ApproveRefund        = "refund"
	ApprovePaidOut       = "paid_out"
	ApproveNoSale        = "no_sale"
)

// NeedsApproval reports whether cashiers need a manager for an action.
// Rules come from store settings; anything not set to "0" is required.
func (t *Terminal) NeedsApproval(ctx context.Context, action string) bool {
	var v string
	t.db.R().QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", "approval_"+action).Scan(&v)
	return v != "0"
}

// authorise returns who authorised an action: the manager (or the user
// themselves when they are one), "" when no approval is needed, or an error.
func (t *Terminal) authorise(ctx context.Context, u User, approver *User, actions ...string) (string, error) {
	need := false
	for _, a := range actions {
		need = need || t.NeedsApproval(ctx, a)
	}
	by, err := approvedBy(u, approver)
	if err != nil && !need {
		return "", nil
	}
	return by, err
}

// approvedBy decides who authorised a sensitive action, or fails.
func approvedBy(u User, approver *User) (string, error) {
	if IsManager(u.Role) {
		return u.ID, nil
	}
	if approver != nil && IsManager(approver.Role) {
		return approver.ID, nil
	}
	return "", ErrApprovalRequired
}

// mulDivRound returns a*b/c rounded half up (a, b, c >= 0).
func mulDivRound(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	return (a*b + c/2) / c
}

// lineTax computes the tax contained in (inclusive) or added to (exclusive) net.
func lineTax(mode string, net, bp int64) int64 {
	if bp <= 0 || net <= 0 {
		return 0
	}
	switch mode {
	case protocol.TaxInclusive:
		return net - mulDivRound(net, 10000, 10000+bp)
	case protocol.TaxExclusive:
		return mulDivRound(net, bp, 10000)
	}
	return 0
}

// allocate splits amount across weights proportionally; shares sum exactly to amount.
func allocate(amount int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	var total int64
	for _, w := range weights {
		total += w
	}
	if amount == 0 || total == 0 {
		return out
	}
	type rem struct {
		i int
		r int64
	}
	var given int64
	rems := make([]rem, len(weights))
	for i, w := range weights {
		out[i] = amount * w / total
		given += out[i]
		rems[i] = rem{i, amount * w % total}
	}
	sort.SliceStable(rems, func(a, b int) bool { return rems[a].r > rems[b].r })
	for k := 0; given < amount; k++ {
		out[rems[k%len(rems)].i]++
		given++
	}
	return out
}

func (t *Terminal) taxMode(ctx context.Context) string {
	switch m := t.Settings(ctx)["tax_mode"]; m {
	case protocol.TaxInclusive, protocol.TaxExclusive:
		return m
	}
	return protocol.TaxNone
}

// RecordSale validates and commits a sale locally, queues it for the hub and
// returns the stored sale. Prices come from the local catalogue (or an
// approved override), never blindly from the UI, and totals are computed here.
func (t *Terminal) RecordSale(ctx context.Context, u User, approver *User, in SaleInput) (*protocol.Sale, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired with a hub")
	}
	if len(in.Lines) == 0 {
		return nil, saleErr("the cart is empty")
	}
	if in.DiscountMinor < 0 {
		return nil, saleErr("discount cannot be negative")
	}
	var actions []string
	if in.DiscountMinor > 0 {
		actions = append(actions, ApproveDiscount)
	}
	for _, l := range in.Lines {
		if l.PriceOverride != nil {
			if *l.PriceOverride < 0 {
				return nil, saleErr("price cannot be negative")
			}
			actions = append(actions, ApprovePriceOverride)
		}
	}
	approved := ""
	if len(actions) > 0 {
		by, err := t.authorise(ctx, u, approver, actions...)
		if err != nil {
			return nil, err
		}
		approved = by
	}
	shiftID := t.kv(ctx, "shift_id")
	if shiftID == "" {
		return nil, saleErr("open a shift before selling")
	}
	mode := t.taxMode(ctx)

	sale := &protocol.Sale{
		ID: ids.New(), TerminalID: p.TerminalID, LocationID: p.LocationID, UserID: u.ID, UserName: u.Name,
		Status: "completed", Note: in.Note, ShiftID: shiftID, ApprovedBy: approved, TaxMode: mode, CustomerID: in.CustomerID,
	}

	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		sale.CreatedAt = now
		sale.Lines, sale.Payments = nil, nil
		sale.SubtotalMinor, sale.TaxMinor = 0, 0

		if sale.CustomerID != "" {
			if err := tx.QueryRowContext(ctx, "SELECT name FROM customers WHERE id=?", sale.CustomerID).Scan(&sale.CustomerName); err != nil {
				return saleErr("that customer is not known on this till")
			}
		}

		for i, cl := range in.Lines {
			if cl.QtyMilli <= 0 {
				return saleErr("quantity must be more than zero")
			}
			var l protocol.SaleLine
			var allowFraction, active bool
			err := tx.QueryRowContext(ctx, `SELECT v.id, p.name, v.name, v.sku, v.price_minor, p.track_stock, p.allow_fraction, (p.active AND v.active), p.tax_rate_bp
				FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, cl.VariantID).
				Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.UnitPriceMinor, &l.TrackStock, &allowFraction, &active, &l.TaxRateBP)
			if errors.Is(err, sql.ErrNoRows) {
				return saleErr("an item in the cart is no longer in the catalogue")
			}
			if err != nil {
				return err
			}
			if !active {
				return saleErr("%s is no longer for sale", l.ProductName)
			}
			if !allowFraction && cl.QtyMilli%money.QtyScale != 0 {
				return saleErr("%s is sold in whole units", l.ProductName)
			}
			if cl.PriceOverride != nil && *cl.PriceOverride != l.UnitPriceMinor {
				l.OriginalPriceMinor = l.UnitPriceMinor
				l.UnitPriceMinor = *cl.PriceOverride
			}
			if mode == protocol.TaxNone {
				l.TaxRateBP = 0
			}
			l.ID = ids.New()
			l.LineNo = i + 1
			l.QtyMilli = cl.QtyMilli
			l.TotalMinor = money.LineTotal(l.QtyMilli, l.UnitPriceMinor)
			sale.SubtotalMinor += l.TotalMinor
			sale.Lines = append(sale.Lines, l)
		}
		if in.DiscountMinor > sale.SubtotalMinor {
			return saleErr("discount is larger than the sale")
		}
		sale.DiscountMinor = in.DiscountMinor

		// Spread the cart discount over lines, then compute tax on what's left.
		weights := make([]int64, len(sale.Lines))
		for i, l := range sale.Lines {
			weights[i] = l.TotalMinor
		}
		shares := allocate(sale.DiscountMinor, weights)
		var net int64
		for i := range sale.Lines {
			l := &sale.Lines[i]
			l.DiscountMinor = shares[i]
			lineNet := l.TotalMinor - l.DiscountMinor
			l.TaxMinor = lineTax(mode, lineNet, l.TaxRateBP)
			sale.TaxMinor += l.TaxMinor
			net += lineNet
		}
		sale.TotalMinor = net
		if mode == protocol.TaxExclusive {
			sale.TotalMinor += sale.TaxMinor
		}

		var paid, cash, credit int64
		for _, pi := range in.Payments {
			if pi.AmountMinor <= 0 {
				continue
			}
			var pm protocol.SalePayment
			var requiresRef, active bool
			err := tx.QueryRowContext(ctx, "SELECT id,name,kind,requires_reference,active FROM payment_methods WHERE id=?", pi.MethodID).
				Scan(&pm.MethodID, &pm.MethodName, &pm.Kind, &requiresRef, &active)
			if errors.Is(err, sql.ErrNoRows) || !active {
				return saleErr("that payment method is not available")
			}
			if err != nil {
				return err
			}
			if requiresRef && pi.Reference == "" {
				return saleErr("enter the reference for the %s payment", pm.MethodName)
			}
			pm.ID, pm.AmountMinor, pm.Reference = ids.New(), pi.AmountMinor, pi.Reference
			paid += pm.AmountMinor
			switch pm.Kind {
			case protocol.PayCash:
				cash += pm.AmountMinor
			case protocol.PayCredit:
				credit += pm.AmountMinor
			}
			sale.Payments = append(sale.Payments, pm)
		}
		if credit > 0 && sale.CustomerID == "" {
			return saleErr("choose the customer to put this on account")
		}
		if paid < sale.TotalMinor {
			return saleErr("payment is short by %s", t.Currency(ctx).FormatSymbol(sale.TotalMinor-paid))
		}
		change := paid - sale.TotalMinor
		if change > cash {
			return saleErr("only cash can be overpaid; reduce the non-cash amount")
		}
		sale.PaidMinor, sale.ChangeMinor = paid, change

		seq, err := nextSeq(ctx, tx, "receipt_seq")
		if err != nil {
			return err
		}
		sale.ReceiptNo = fmt.Sprintf("%s-%06d", p.ReceiptPrefix, seq)

		if _, err := tx.ExecContext(ctx, `INSERT INTO sales(id,receipt_no,location_id,user_id,user_name,customer_id,customer_name,status,subtotal_minor,discount_minor,
			tax_minor,total_minor,paid_minor,change_minor,note,created_at,shift_id,approved_by,tax_mode) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			sale.ID, sale.ReceiptNo, sale.LocationID, sale.UserID, sale.UserName, sale.CustomerID, sale.CustomerName, sale.Status, sale.SubtotalMinor,
			sale.DiscountMinor, sale.TaxMinor, sale.TotalMinor, sale.PaidMinor, sale.ChangeMinor, sale.Note, sale.CreatedAt,
			sale.ShiftID, sale.ApprovedBy, sale.TaxMode); err != nil {
			return err
		}
		for _, l := range sale.Lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sale_lines(id,sale_id,line_no,variant_id,product_name,variant_name,sku,qty_milli,unit_price_minor,
				discount_minor,tax_minor,total_minor,track_stock,original_price_minor,tax_rate_bp) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				l.ID, sale.ID, l.LineNo, l.VariantID, l.ProductName, l.VariantName, l.SKU, l.QtyMilli, l.UnitPriceMinor,
				l.DiscountMinor, l.TaxMinor, l.TotalMinor, boolInt(l.TrackStock), l.OriginalPriceMinor, l.TaxRateBP); err != nil {
				return err
			}
		}
		for _, pm := range sale.Payments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sale_payments(id,sale_id,method_id,method_name,kind,amount_minor,reference) VALUES(?,?,?,?,?,?,?)`,
				pm.ID, sale.ID, pm.MethodID, pm.MethodName, pm.Kind, pm.AmountMinor, pm.Reference); err != nil {
				return err
			}
		}
		return enqueue(ctx, tx, protocol.OpSaleCreate, sale.ID, sale, now)
	})
	if err != nil {
		return nil, err
	}
	t.refreshCounts(ctx)
	t.Nudge()
	return sale, nil
}

// nextSeq increments and returns a local counter.
func nextSeq(ctx context.Context, tx *sql.Tx, key string) (int64, error) {
	var cur string
	tx.QueryRowContext(ctx, "SELECT value FROM kv WHERE key=?", key).Scan(&cur)
	n, _ := strconv.ParseInt(cur, 10, 64)
	n++
	return n, setKV(ctx, tx, key, strconv.FormatInt(n, 10))
}

// enqueue adds an op to the outbox in the caller's transaction.
func enqueue(ctx context.Context, tx *sql.Tx, kind, refID string, payload any, now int64) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO outbox(id,kind,ref_id,payload,created_at) VALUES(?,?,?,?,?)", ids.New(), kind, refID, string(b), now)
	return err
}

// GetSale loads a sale recorded on this till.
func (t *Terminal) GetSale(ctx context.Context, id string) (*protocol.Sale, error) {
	s := &protocol.Sale{}
	err := t.db.R().QueryRowContext(ctx, `SELECT id,receipt_no,location_id,user_id,user_name,customer_id,customer_name,status,subtotal_minor,discount_minor,
		tax_minor,total_minor,paid_minor,change_minor,note,created_at,synced_at,shift_id,approved_by,tax_mode FROM sales WHERE id=? OR receipt_no=?`, id, id).
		Scan(&s.ID, &s.ReceiptNo, &s.LocationID, &s.UserID, &s.UserName, &s.CustomerID, &s.CustomerName, &s.Status, &s.SubtotalMinor, &s.DiscountMinor,
			&s.TaxMinor, &s.TotalMinor, &s.PaidMinor, &s.ChangeMinor, &s.Note, &s.CreatedAt, &s.SyncedAt, &s.ShiftID, &s.ApprovedBy, &s.TaxMode)
	if err != nil {
		return nil, err
	}
	p, _ := t.Pairing(ctx)
	s.TerminalID = p.TerminalID

	rows, err := t.db.R().QueryContext(ctx, `SELECT id,line_no,variant_id,product_name,variant_name,sku,qty_milli,unit_price_minor,discount_minor,
		tax_minor,total_minor,track_stock,original_price_minor,tax_rate_bp FROM sale_lines WHERE sale_id=? ORDER BY line_no`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.SaleLine
		if err := rows.Scan(&l.ID, &l.LineNo, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli, &l.UnitPriceMinor,
			&l.DiscountMinor, &l.TaxMinor, &l.TotalMinor, &l.TrackStock, &l.OriginalPriceMinor, &l.TaxRateBP); err != nil {
			rows.Close()
			return nil, err
		}
		s.Lines = append(s.Lines, l)
	}
	rows.Close()

	s.Payments, err = t.payments(ctx, "sale_payments", "sale_id", s.ID)
	return s, err
}

func (t *Terminal) payments(ctx context.Context, table, col, id string) ([]protocol.SalePayment, error) {
	rows, err := t.db.R().QueryContext(ctx, `SELECT id,method_id,method_name,kind,amount_minor,reference FROM `+table+` WHERE `+col+`=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []protocol.SalePayment
	for rows.Next() {
		var pm protocol.SalePayment
		if err := rows.Scan(&pm.ID, &pm.MethodID, &pm.MethodName, &pm.Kind, &pm.AmountMinor, &pm.Reference); err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, rows.Err()
}

// SaleSummary is a row in the sales history list (sales and returns).
type SaleSummary struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // sale | return | payment
	ReceiptNo  string `json:"receipt_no"`
	UserName   string `json:"user_name"`
	Customer   string `json:"customer"`
	TotalMinor int64  `json:"total_minor"` // negative for returns
	Items      int64  `json:"items"`
	Methods    string `json:"methods"`
	CreatedAt  int64  `json:"created_at"`
	Synced     bool   `json:"synced"`
}

// RecentSales lists this till's sales and returns, newest first.
func (t *Terminal) RecentSales(ctx context.Context, query string, limit int) ([]SaleSummary, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT * FROM (
		SELECT s.id, 'sale', s.receipt_no, s.user_name, s.customer_name, s.total_minor,
			(SELECT COALESCE(sum(qty_milli),0) FROM sale_lines WHERE sale_id=s.id),
			(SELECT COALESCE(group_concat(method_name, ', '),'') FROM sale_payments WHERE sale_id=s.id),
			s.created_at, s.synced_at > 0
		FROM sales s WHERE (?1='' OR s.receipt_no LIKE '%'||?1||'%' OR s.customer_name LIKE '%'||?1||'%')
		UNION ALL
		SELECT r.id, 'return', r.return_no, r.user_name, r.customer_name, -r.total_minor,
			(SELECT COALESCE(sum(qty_milli),0) FROM return_lines WHERE return_id=r.id),
			(SELECT COALESCE(group_concat(method_name, ', '),'') FROM return_refunds WHERE return_id=r.id),
			r.created_at, r.synced_at > 0
		FROM returns r WHERE (?1='' OR r.return_no LIKE '%'||?1||'%' OR r.sale_receipt_no LIKE '%'||?1||'%' OR r.customer_name LIKE '%'||?1||'%')
		UNION ALL
		SELECT a.id, 'payment', a.payment_no, a.user_name, a.customer_name, a.amount_minor, 0, a.method_name, a.created_at, a.synced_at > 0
		FROM account_payments a WHERE (?1='' OR a.payment_no LIKE '%'||?1||'%' OR a.customer_name LIKE '%'||?1||'%')
		) ORDER BY 9 DESC LIMIT ?2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SaleSummary{}
	for rows.Next() {
		var s SaleSummary
		if err := rows.Scan(&s.ID, &s.Kind, &s.ReceiptNo, &s.UserName, &s.Customer, &s.TotalMinor, &s.Items, &s.Methods, &s.CreatedAt, &s.Synced); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
