package terminal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// ReturnableLine is a sold line with how much can still be returned.
type ReturnableLine struct {
	protocol.SaleLine
	ReturnedMilli   int64 `json:"returned_milli"`
	ReturnableMilli int64 `json:"returnable_milli"`
	PaidMinor       int64 `json:"paid_minor"` // what the customer paid for the whole line
}

// ReturnableSale is a sale found for a return.
type ReturnableSale struct {
	Sale   protocol.Sale    `json:"sale"`
	Lines  []ReturnableLine `json:"lines"`
	Source string           `json:"source"` // local | hub
	// Partial is true when the hub could not be asked, so returns made on
	// other tills may not be counted yet.
	Partial bool `json:"partial"`
}

// LookupForReturn finds a sale by receipt number. Sales from this till are
// found offline; sales from other tills need the hub.
func (t *Terminal) LookupForReturn(ctx context.Context, receiptNo string) (*ReturnableSale, error) {
	receiptNo = strings.ToUpper(strings.TrimSpace(receiptNo))
	if receiptNo == "" {
		return nil, saleErr("enter or scan the receipt number")
	}
	out := &ReturnableSale{}
	var hubReturned map[string]int64

	local, err := t.GetSale(ctx, receiptNo)
	if err == nil {
		out.Sale, out.Source = *local, "local"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Ask the hub: it knows sales from all tills and returns made anywhere.
	if p, ok := t.Pairing(ctx); ok {
		var lk protocol.SaleLookup
		herr := t.client.do(ctx, 5*time.Second, http.MethodGet, p.HubURL+"/api/sales/lookup?receipt="+url.QueryEscape(receiptNo), p.token, "", nil, &lk)
		switch {
		case herr == nil:
			if out.Source == "" {
				out.Sale, out.Source = lk.Sale, "hub"
			}
			hubReturned = lk.Returned
		case out.Source == "" && IsOffline(herr):
			return nil, saleErr("receipt %s isn't from this till and the hub can't be reached to look it up", receiptNo)
		case out.Source == "":
			return nil, saleErr("%s", herr.Error())
		default:
			out.Partial = true // local sale, hub offline or unaware yet
		}
	} else if out.Source == "" {
		return nil, saleErr("no sale with receipt number %s", receiptNo)
	}

	// Returned so far = hub's count + this till's returns the hub hasn't seen.
	// Without the hub, count all of this till's returns.
	q := `SELECT rl.sale_line_id, sum(rl.qty_milli) FROM return_lines rl JOIN returns r ON r.id=rl.return_id WHERE r.sale_id=?`
	if hubReturned != nil {
		q += ` AND r.synced_at=0`
	}
	rows, err := t.db.R().QueryContext(ctx, q+` GROUP BY rl.sale_line_id`, out.Sale.ID)
	if err != nil {
		return nil, err
	}
	returned := map[string]int64{}
	for k, v := range hubReturned {
		returned[k] = v
	}
	for rows.Next() {
		var id string
		var n int64
		rows.Scan(&id, &n)
		returned[id] += n
	}
	rows.Close()

	for _, l := range out.Sale.Lines {
		rl := ReturnableLine{SaleLine: l, ReturnedMilli: returned[l.ID], PaidMinor: l.PaidMinor(out.Sale.TaxMode)}
		rl.ReturnableMilli = max(0, l.QtyMilli-rl.ReturnedMilli)
		out.Lines = append(out.Lines, rl)
	}
	return out, nil
}

// ReturnLineInput selects what comes back.
type ReturnLineInput struct {
	SaleLineID string `json:"sale_line_id"`
	QtyMilli   int64  `json:"qty_milli"`
	Restock    bool   `json:"restock"`
}

// ReturnInput is a refund request from the UI.
type ReturnInput struct {
	ReceiptNo     string            `json:"receipt_no"`
	Lines         []ReturnLineInput `json:"lines"`
	Refunds       []PaymentInput    `json:"refunds"`
	Reason        string            `json:"reason"`
	ApprovalToken string            `json:"approval_token"`
}

// RecordReturn refunds part or all of an earlier sale. Amounts are computed
// from what the customer actually paid per line (after discounts, incl. tax).
func (t *Terminal) RecordReturn(ctx context.Context, u User, approver *User, in ReturnInput) (*protocol.Return, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired with a hub")
	}
	approved, err := t.authorise(ctx, u, approver, ApproveRefund)
	if err != nil {
		return nil, err
	}
	shiftID := t.kv(ctx, "shift_id")
	if shiftID == "" {
		return nil, saleErr("open a shift before giving refunds")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, saleErr("give a reason for the return")
	}
	rs, err := t.LookupForReturn(ctx, in.ReceiptNo)
	if err != nil {
		return nil, err
	}
	byID := map[string]ReturnableLine{}
	for _, l := range rs.Lines {
		byID[l.ID] = l
	}

	r := &protocol.Return{
		ID: ids.New(), SaleID: rs.Sale.ID, SaleReceiptNo: rs.Sale.ReceiptNo, TerminalID: p.TerminalID, LocationID: p.LocationID,
		UserID: u.ID, UserName: u.Name, ApprovedBy: approved, ShiftID: shiftID, CustomerID: rs.Sale.CustomerID,
		CustomerName: rs.Sale.CustomerName, Reason: strings.TrimSpace(in.Reason),
	}
	for _, li := range in.Lines {
		if li.QtyMilli <= 0 {
			continue
		}
		sl, ok := byID[li.SaleLineID]
		if !ok {
			return nil, saleErr("that item is not on receipt %s", rs.Sale.ReceiptNo)
		}
		if li.QtyMilli > sl.ReturnableMilli {
			return nil, saleErr("only %s of %s can still be returned", fmtQty(sl.ReturnableMilli), sl.ProductName)
		}
		rl := protocol.ReturnLine{
			ID: ids.New(), SaleLineID: sl.ID, VariantID: sl.VariantID, ProductName: sl.ProductName, VariantName: sl.VariantName, SKU: sl.SKU,
			QtyMilli: li.QtyMilli, Restock: li.Restock, TrackStock: sl.TrackStock,
		}
		// Refund the same proportion of what was paid; the last unit takes any rounding remainder.
		if li.QtyMilli == sl.ReturnableMilli {
			paidBefore := mulDivRound(sl.PaidMinor, sl.ReturnedMilli, sl.QtyMilli)
			taxBefore := mulDivRound(sl.TaxMinor, sl.ReturnedMilli, sl.QtyMilli)
			rl.TotalMinor = sl.PaidMinor - paidBefore
			rl.TaxMinor = sl.TaxMinor - taxBefore
		} else {
			rl.TotalMinor = mulDivRound(sl.PaidMinor, li.QtyMilli, sl.QtyMilli)
			rl.TaxMinor = mulDivRound(sl.TaxMinor, li.QtyMilli, sl.QtyMilli)
		}
		r.TotalMinor += rl.TotalMinor
		r.TaxMinor += rl.TaxMinor
		r.Lines = append(r.Lines, rl)
	}
	if len(r.Lines) == 0 {
		return nil, saleErr("choose at least one item to return")
	}

	return t.storeReturn(ctx, p, r, in.Refunds)
}

// storeReturn checks the refunds add up, numbers the return and queues it.
func (t *Terminal) storeReturn(ctx context.Context, p Pairing, r *protocol.Return, refunds []PaymentInput) (*protocol.Return, error) {
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		r.CreatedAt = now
		r.Refunds = nil
		var refunded int64
		for _, pi := range refunds {
			if pi.AmountMinor <= 0 {
				continue
			}
			var pm protocol.SalePayment
			if pi.MethodID == StoreCreditMethodID {
				if r.CustomerID == "" {
					return saleErr("choose the customer to give store credit to")
				}
				pm.MethodID, pm.MethodName, pm.Kind = StoreCreditMethodID, StoreCreditName, protocol.PayCredit
			} else {
				var active bool
				err := tx.QueryRowContext(ctx, "SELECT id,name,kind,active FROM payment_methods WHERE id=?", pi.MethodID).Scan(&pm.MethodID, &pm.MethodName, &pm.Kind, &active)
				if errors.Is(err, sql.ErrNoRows) || !active {
					return saleErr("that refund method is not available")
				}
				if err != nil {
					return err
				}
			}
			if pm.Kind == protocol.PayCredit && r.CustomerID == "" {
				return saleErr("this sale had no customer, so it can't be refunded to an account")
			}
			pm.ID, pm.AmountMinor, pm.Reference = ids.New(), pi.AmountMinor, pi.Reference
			refunded += pm.AmountMinor
			r.Refunds = append(r.Refunds, pm)
		}
		if refunded != r.TotalMinor {
			cur := t.Currency(ctx)
			return saleErr("refund %s in total (you entered %s)", cur.FormatSymbol(r.TotalMinor), cur.FormatSymbol(refunded))
		}
		seq, err := nextSeq(ctx, tx, "return_seq")
		if err != nil {
			return err
		}
		r.ReturnNo = fmt.Sprintf("%s-R%05d", p.ReceiptPrefix, seq)

		if _, err := tx.ExecContext(ctx, `INSERT INTO returns(id,return_no,sale_id,sale_receipt_no,location_id,user_id,user_name,approved_by,shift_id,
			customer_id,customer_name,reason,tax_minor,total_minor,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			r.ID, r.ReturnNo, r.SaleID, r.SaleReceiptNo, r.LocationID, r.UserID, r.UserName, r.ApprovedBy, r.ShiftID,
			r.CustomerID, r.CustomerName, r.Reason, r.TaxMinor, r.TotalMinor, r.CreatedAt); err != nil {
			return err
		}
		for _, l := range r.Lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO return_lines(id,return_id,sale_line_id,variant_id,product_name,variant_name,sku,qty_milli,tax_minor,total_minor,restock,track_stock)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, l.ID, r.ID, l.SaleLineID, l.VariantID, l.ProductName, l.VariantName, l.SKU, l.QtyMilli,
				l.TaxMinor, l.TotalMinor, boolInt(l.Restock), boolInt(l.TrackStock)); err != nil {
				return err
			}
		}
		for _, pm := range r.Refunds {
			if _, err := tx.ExecContext(ctx, `INSERT INTO return_refunds(id,return_id,method_id,method_name,kind,amount_minor,reference) VALUES(?,?,?,?,?,?,?)`,
				pm.ID, r.ID, pm.MethodID, pm.MethodName, pm.Kind, pm.AmountMinor, pm.Reference); err != nil {
				return err
			}
		}
		return enqueue(ctx, tx, protocol.OpReturnCreate, r.ID, r, now)
	})
	if err != nil {
		return nil, err
	}
	t.refreshCounts(ctx)
	t.Nudge()
	return r, nil
}

// GetReturn loads a return made on this till.
func (t *Terminal) GetReturn(ctx context.Context, id string) (*protocol.Return, error) {
	r := &protocol.Return{}
	err := t.db.R().QueryRowContext(ctx, `SELECT id,return_no,sale_id,sale_receipt_no,location_id,user_id,user_name,approved_by,shift_id,customer_id,
		customer_name,reason,tax_minor,total_minor,created_at,synced_at FROM returns WHERE id=? OR return_no=?`, id, id).
		Scan(&r.ID, &r.ReturnNo, &r.SaleID, &r.SaleReceiptNo, &r.LocationID, &r.UserID, &r.UserName, &r.ApprovedBy, &r.ShiftID, &r.CustomerID,
			&r.CustomerName, &r.Reason, &r.TaxMinor, &r.TotalMinor, &r.CreatedAt, &r.SyncedAt)
	if err != nil {
		return nil, err
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT id,sale_line_id,variant_id,product_name,variant_name,sku,qty_milli,tax_minor,total_minor,restock,track_stock
		FROM return_lines WHERE return_id=?`, r.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.ReturnLine
		if err := rows.Scan(&l.ID, &l.SaleLineID, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli, &l.TaxMinor, &l.TotalMinor, &l.Restock, &l.TrackStock); err != nil {
			rows.Close()
			return nil, err
		}
		r.Lines = append(r.Lines, l)
	}
	rows.Close()
	r.Refunds, err = t.payments(ctx, "return_refunds", "return_id", r.ID)
	return r, err
}

func fmtQty(milli int64) string {
	if milli%1000 == 0 {
		return fmt.Sprint(milli / 1000)
	}
	return strings.TrimRight(fmt.Sprintf("%.3f", float64(milli)/1000), "0")
}

// Store credit: a refund kept on the customer's account (their balance goes
// below zero) and spent on later sales. It needs no payment method set up.
const (
	StoreCreditMethodID = "store_credit"
	StoreCreditName     = "Store credit"
)

// NoReceiptLine is an item brought back without a receipt.
type NoReceiptLine struct {
	VariantID string `json:"variant_id"`
	QtyMilli  int64  `json:"qty_milli"`
	// UnitRefundMinor is what is given back per unit, tax included; 0 means
	// today's price. It can be lower than today's price, never higher.
	UnitRefundMinor int64 `json:"unit_refund_minor"`
	Restock         bool  `json:"restock"`
}

// NoReceiptInput is a return without a receipt.
type NoReceiptInput struct {
	Lines         []NoReceiptLine `json:"lines"`
	CustomerID    string          `json:"customer_id"`
	Refunds       []PaymentInput  `json:"refunds"`
	Reason        string          `json:"reason"`
	ApprovalToken string          `json:"approval_token"`
}

// NoReceiptPrice is today's refund price per unit (tax included) for an item.
func (t *Terminal) NoReceiptPrice(ctx context.Context, variantID string) (int64, error) {
	var price, bp int64
	err := t.db.R().QueryRowContext(ctx, `SELECT v.price_minor, p.tax_rate_bp FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, variantID).Scan(&price, &bp)
	if err != nil {
		return 0, saleErr("item not found")
	}
	if t.Settings(ctx)["tax_mode"] == protocol.TaxExclusive {
		price += lineTax(protocol.TaxExclusive, price, bp)
	}
	return price, nil
}

// RecordReturnNoReceipt takes goods back without a receipt. A manager must
// always approve it (whatever the approval settings), and the refund can't
// exceed today's price.
func (t *Terminal) RecordReturnNoReceipt(ctx context.Context, u User, approver *User, in NoReceiptInput) (*protocol.Return, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired with a hub")
	}
	approved, err := approvedBy(u, approver)
	if err != nil {
		return nil, err
	}
	shiftID := t.kv(ctx, "shift_id")
	if shiftID == "" {
		return nil, saleErr("open a shift before giving refunds")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, saleErr("give a reason for the return")
	}
	r := &protocol.Return{ID: ids.New(), TerminalID: p.TerminalID, LocationID: p.LocationID, UserID: u.ID, UserName: u.Name,
		ApprovedBy: approved, ShiftID: shiftID, Reason: "No receipt: " + strings.TrimSpace(in.Reason)}
	if in.CustomerID != "" {
		err := t.db.R().QueryRowContext(ctx, "SELECT id,name FROM customers WHERE id=?", in.CustomerID).Scan(&r.CustomerID, &r.CustomerName)
		if err != nil {
			return nil, saleErr("customer not found")
		}
	}
	mode := t.Settings(ctx)["tax_mode"]
	seen := map[string]bool{}
	for _, li := range in.Lines {
		if li.QtyMilli <= 0 {
			continue
		}
		if seen[li.VariantID] {
			return nil, saleErr("an item is listed twice")
		}
		seen[li.VariantID] = true
		var l protocol.ReturnLine
		var bp int64
		var fraction bool
		err := t.db.R().QueryRowContext(ctx, `SELECT v.id,p.name,v.name,v.sku,p.track_stock,p.tax_rate_bp,p.allow_fraction
			FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, li.VariantID).
			Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.TrackStock, &bp, &fraction)
		if err != nil {
			return nil, saleErr("item not found")
		}
		if !fraction && li.QtyMilli%1000 != 0 {
			return nil, saleErr("%s is returned in whole units", l.ProductName)
		}
		today, _ := t.NoReceiptPrice(ctx, li.VariantID)
		unit := li.UnitRefundMinor
		if unit == 0 {
			unit = today
		}
		if unit < 0 || unit > today {
			return nil, saleErr("the refund for %s can't be more than today's price", l.ProductName)
		}
		l.ID, l.QtyMilli, l.Restock = ids.New(), li.QtyMilli, li.Restock
		l.TotalMinor = mulDivRound(unit, li.QtyMilli, 1000)
		// Tax inside the refund, at the item's rate.
		if mode == protocol.TaxInclusive || mode == protocol.TaxExclusive {
			l.TaxMinor = lineTax(protocol.TaxInclusive, l.TotalMinor, bp)
		}
		r.TotalMinor += l.TotalMinor
		r.TaxMinor += l.TaxMinor
		r.Lines = append(r.Lines, l)
	}
	if len(r.Lines) == 0 {
		return nil, saleErr("add the items being returned")
	}
	return t.storeReturn(ctx, p, r, in.Refunds)
}
