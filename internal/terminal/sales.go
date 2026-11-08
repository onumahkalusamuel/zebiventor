package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// CartLine is one line of a sale as entered at the till.
type CartLine struct {
	VariantID string `json:"variant_id"`
	QtyMilli  int64  `json:"qty_milli"`
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
	Note          string         `json:"note"`
}

// SaleError is a problem the cashier must fix before the sale can complete.
type SaleError struct{ Msg string }

func (e SaleError) Error() string { return e.Msg }

func saleErr(format string, a ...any) error { return SaleError{fmt.Sprintf(format, a...)} }

var discountRoles = map[string]bool{protocol.RoleAdmin: true, protocol.RoleManager: true}

// RecordSale validates and commits a sale locally, queues it for the hub and
// returns the stored sale. Prices come from the local catalogue, never from
// the UI, and totals are computed here.
func (t *Terminal) RecordSale(ctx context.Context, u User, in SaleInput) (*protocol.Sale, error) {
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
	if in.DiscountMinor > 0 && !discountRoles[u.Role] {
		return nil, saleErr("only a manager can give a discount")
	}

	sale := &protocol.Sale{
		ID:         ids.New(),
		TerminalID: p.TerminalID,
		LocationID: p.LocationID,
		UserID:     u.ID,
		UserName:   u.Name,
		Status:     "completed",
		Note:       in.Note,
	}

	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		sale.CreatedAt = now
		sale.Lines = sale.Lines[:0]
		sale.Payments = sale.Payments[:0]
		sale.SubtotalMinor = 0

		for i, cl := range in.Lines {
			if cl.QtyMilli <= 0 {
				return saleErr("quantity must be more than zero")
			}
			var l protocol.SaleLine
			var allowFraction, active bool
			err := tx.QueryRowContext(ctx, `SELECT v.id, p.name, v.name, v.sku, v.price_minor, p.track_stock, p.allow_fraction, (p.active AND v.active)
				FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, cl.VariantID).
				Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.UnitPriceMinor, &l.TrackStock, &allowFraction, &active)
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
		sale.TotalMinor = sale.SubtotalMinor - sale.DiscountMinor + sale.TaxMinor

		var paid, cash int64
		sale.PaidMinor = 0
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
			pm.ID = ids.New()
			pm.AmountMinor = pi.AmountMinor
			pm.Reference = pi.Reference
			paid += pm.AmountMinor
			if pm.Kind == protocol.PayCash {
				cash += pm.AmountMinor
			}
			sale.Payments = append(sale.Payments, pm)
		}
		if paid < sale.TotalMinor {
			return saleErr("payment is short by %s", t.Currency(ctx).FormatSymbol(sale.TotalMinor-paid))
		}
		change := paid - sale.TotalMinor
		if change > cash {
			return saleErr("only cash can be overpaid; reduce the non-cash amount")
		}
		sale.PaidMinor, sale.ChangeMinor = paid, change

		// Receipt number: terminal prefix + local sequence, unique offline.
		var seqStr string
		tx.QueryRowContext(ctx, "SELECT value FROM kv WHERE key='receipt_seq'").Scan(&seqStr)
		seq, _ := strconv.ParseInt(seqStr, 10, 64)
		seq++
		if err := setKV(ctx, tx, "receipt_seq", strconv.FormatInt(seq, 10)); err != nil {
			return err
		}
		sale.ReceiptNo = fmt.Sprintf("%s-%06d", p.ReceiptPrefix, seq)

		if _, err := tx.ExecContext(ctx, `INSERT INTO sales(id,receipt_no,location_id,user_id,user_name,customer_id,status,subtotal_minor,discount_minor,
			tax_minor,total_minor,paid_minor,change_minor,note,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			sale.ID, sale.ReceiptNo, sale.LocationID, sale.UserID, sale.UserName, sale.CustomerID, sale.Status, sale.SubtotalMinor,
			sale.DiscountMinor, sale.TaxMinor, sale.TotalMinor, sale.PaidMinor, sale.ChangeMinor, sale.Note, sale.CreatedAt); err != nil {
			return err
		}
		for _, l := range sale.Lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sale_lines(id,sale_id,line_no,variant_id,product_name,variant_name,sku,qty_milli,unit_price_minor,
				discount_minor,tax_minor,total_minor,track_stock) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				l.ID, sale.ID, l.LineNo, l.VariantID, l.ProductName, l.VariantName, l.SKU, l.QtyMilli, l.UnitPriceMinor,
				l.DiscountMinor, l.TaxMinor, l.TotalMinor, boolInt(l.TrackStock)); err != nil {
				return err
			}
		}
		for _, pm := range sale.Payments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sale_payments(id,sale_id,method_id,method_name,kind,amount_minor,reference) VALUES(?,?,?,?,?,?,?)`,
				pm.ID, sale.ID, pm.MethodID, pm.MethodName, pm.Kind, pm.AmountMinor, pm.Reference); err != nil {
				return err
			}
		}
		payload, err := json.Marshal(sale)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO outbox(id,kind,ref_id,payload,created_at) VALUES(?,?,?,?,?)",
			ids.New(), protocol.OpSaleCreate, sale.ID, string(payload), now)
		return err
	})
	if err != nil {
		return nil, err
	}
	t.refreshCounts(ctx)
	t.Nudge()
	return sale, nil
}

// GetSale loads a sale recorded on this till.
func (t *Terminal) GetSale(ctx context.Context, id string) (*protocol.Sale, error) {
	s := &protocol.Sale{}
	err := t.db.R().QueryRowContext(ctx, `SELECT id,receipt_no,location_id,user_id,user_name,customer_id,status,subtotal_minor,discount_minor,
		tax_minor,total_minor,paid_minor,change_minor,note,created_at,synced_at FROM sales WHERE id=? OR receipt_no=?`, id, id).
		Scan(&s.ID, &s.ReceiptNo, &s.LocationID, &s.UserID, &s.UserName, &s.CustomerID, &s.Status, &s.SubtotalMinor, &s.DiscountMinor,
			&s.TaxMinor, &s.TotalMinor, &s.PaidMinor, &s.ChangeMinor, &s.Note, &s.CreatedAt, &s.SyncedAt)
	if err != nil {
		return nil, err
	}
	p, _ := t.Pairing(ctx)
	s.TerminalID = p.TerminalID

	rows, err := t.db.R().QueryContext(ctx, `SELECT id,line_no,variant_id,product_name,variant_name,sku,qty_milli,unit_price_minor,discount_minor,
		tax_minor,total_minor,track_stock FROM sale_lines WHERE sale_id=? ORDER BY line_no`, s.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.SaleLine
		if err := rows.Scan(&l.ID, &l.LineNo, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli, &l.UnitPriceMinor,
			&l.DiscountMinor, &l.TaxMinor, &l.TotalMinor, &l.TrackStock); err != nil {
			rows.Close()
			return nil, err
		}
		s.Lines = append(s.Lines, l)
	}
	rows.Close()

	rows, err = t.db.R().QueryContext(ctx, `SELECT id,method_id,method_name,kind,amount_minor,reference FROM sale_payments WHERE sale_id=?`, s.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pm protocol.SalePayment
		if err := rows.Scan(&pm.ID, &pm.MethodID, &pm.MethodName, &pm.Kind, &pm.AmountMinor, &pm.Reference); err != nil {
			return nil, err
		}
		s.Payments = append(s.Payments, pm)
	}
	return s, rows.Err()
}

// SaleSummary is a row in the sales history list.
type SaleSummary struct {
	ID         string `json:"id"`
	ReceiptNo  string `json:"receipt_no"`
	UserName   string `json:"user_name"`
	TotalMinor int64  `json:"total_minor"`
	Items      int64  `json:"items"`
	Methods    string `json:"methods"`
	CreatedAt  int64  `json:"created_at"`
	Synced     bool   `json:"synced"`
}

// RecentSales lists this till's sales, newest first.
func (t *Terminal) RecentSales(ctx context.Context, query string, limit int) ([]SaleSummary, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT s.id, s.receipt_no, s.user_name, s.total_minor,
		(SELECT COALESCE(sum(qty_milli),0) FROM sale_lines WHERE sale_id=s.id),
		(SELECT COALESCE(group_concat(method_name, ', '),'') FROM sale_payments WHERE sale_id=s.id),
		s.created_at, s.synced_at > 0
		FROM sales s WHERE (?='' OR s.receipt_no LIKE '%'||?||'%') ORDER BY s.created_at DESC LIMIT ?`, query, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SaleSummary{}
	for rows.Next() {
		var s SaleSummary
		if err := rows.Scan(&s.ID, &s.ReceiptNo, &s.UserName, &s.TotalMinor, &s.Items, &s.Methods, &s.CreatedAt, &s.Synced); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DayTotals summarises today's sales on this till.
type DayTotals struct {
	Count    int64            `json:"count"`
	Total    int64            `json:"total_minor"`
	ByMethod map[string]int64 `json:"by_method"`
	Unsynced int64            `json:"unsynced"`
}

// TodayTotals returns today's totals for this till.
func (t *Terminal) TodayTotals(ctx context.Context, startOfDayMs int64) (DayTotals, error) {
	d := DayTotals{ByMethod: map[string]int64{}}
	err := t.db.R().QueryRowContext(ctx, `SELECT count(*), COALESCE(sum(total_minor),0), COALESCE(sum(synced_at=0),0) FROM sales WHERE created_at>=?`, startOfDayMs).
		Scan(&d.Count, &d.Total, &d.Unsynced)
	if err != nil {
		return d, err
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT p.method_name, sum(p.amount_minor)
		FROM sale_payments p JOIN sales s ON s.id=p.sale_id WHERE s.created_at>=? GROUP BY p.method_name`, startOfDayMs)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var amt int64
		if err := rows.Scan(&name, &amt); err != nil {
			return d, err
		}
		d.ByMethod[name] = amt
	}
	// Change is given in cash, so net it off the cash tender.
	var change int64
	t.db.R().QueryRowContext(ctx, `SELECT COALESCE(sum(change_minor),0) FROM sales WHERE created_at>=?`, startOfDayMs).Scan(&change)
	if change > 0 {
		var cashName string
		t.db.R().QueryRowContext(ctx, `SELECT name FROM payment_methods WHERE kind='cash' ORDER BY sort_order LIMIT 1`).Scan(&cashName)
		if cashName != "" {
			d.ByMethod[cashName] -= change
		}
	}
	return d, rows.Err()
}
