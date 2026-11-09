package hub

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Money owed to suppliers. A bill (the supplier's invoice) raises what we
// owe; a payment lowers it and is allocated to bills so each bill shows
// whether it is paid. Payments larger than the open bills leave a credit
// that is used up by the next bill.

const dayMs = int64(24 * time.Hour / time.Millisecond)

func startOfDay(t time.Time) int64 {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location()).UnixMilli()
}

func startOfToday() int64 { return startOfDay(ids.Now()) }

// SupplierMethods are the ways a shop pays its suppliers.
var SupplierMethods = []string{"Cash", "Bank transfer", "Cheque", "Card", "Other"}

type supplierLedgerEntry struct {
	SupplierID, Kind, RefID, RefNo, Note, UserName string
	Amount, At                                     int64
}

// postSupplierLedger changes what we owe a supplier and records why.
func postSupplierLedger(ctx context.Context, tx *sql.Tx, e supplierLedgerEntry) error {
	if e.At == 0 {
		e.At = ids.NowMs()
	}
	if _, err := tx.ExecContext(ctx, "UPDATE suppliers SET balance_minor=balance_minor+?, updated_at=? WHERE id=?", e.Amount, ids.NowMs(), e.SupplierID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO supplier_ledger(id,supplier_id,at,kind,ref_id,ref_no,amount_minor,note,user_name) VALUES(?,?,?,?,?,?,?,?,?)`,
		ids.New(), e.SupplierID, e.At, e.Kind, e.RefID, e.RefNo, e.Amount, e.Note, e.UserName)
	return err
}

func supplierName(ctx context.Context, q rowQuerier, id string) (string, bool) {
	var name string
	err := q.QueryRowContext(ctx, "SELECT name FROM suppliers WHERE id=?", id).Scan(&name)
	return name, err == nil
}

// ---------- bills ----------

const billSelect = `SELECT b.id,b.bill_no,b.supplier_id,COALESCE(s.name,''),b.supplier_ref,b.grn_id,COALESCE(g.grn_no,''),b.bill_date,b.due_date,
	b.amount_minor,b.paid_minor,b.status,b.note,COALESCE(u.name,b.created_by),b.created_at
	FROM supplier_bills b LEFT JOIN suppliers s ON s.id=b.supplier_id LEFT JOIN goods_receipts g ON g.id=b.grn_id
	LEFT JOIN users u ON u.id=b.created_by`

func scanBill(r interface{ Scan(...any) error }) (protocol.SupplierBill, error) {
	var b protocol.SupplierBill
	err := r.Scan(&b.ID, &b.BillNo, &b.SupplierID, &b.SupplierName, &b.SupplierRef, &b.GRNID, &b.GRNNo, &b.BillDate, &b.DueDate,
		&b.AmountMinor, &b.PaidMinor, &b.Status, &b.Note, &b.CreatedBy, &b.CreatedAt)
	return b, err
}

func queryBills(ctx context.Context, q rowQuerier, where string, args ...any) ([]protocol.SupplierBill, error) {
	rows, err := q.QueryContext(ctx, billSelect+" WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.SupplierBill{}
	for rows.Next() {
		b, err := scanBill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListBills lists supplier bills. status is "open", "overdue", "paid",
// "void" or empty for all.
func (h *Hub) ListBills(ctx context.Context, by *Session, supplierID, status string) ([]protocol.SupplierBill, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	where, args := "1=1", []any{}
	if supplierID != "" {
		where += " AND b.supplier_id=?"
		args = append(args, supplierID)
	}
	switch status {
	case "":
	case "overdue":
		where += " AND b.status='open' AND b.due_date<?"
		args = append(args, startOfToday())
	case protocol.BillOpen, protocol.BillPaid, protocol.BillVoid:
		where += " AND b.status=?"
		args = append(args, status)
	default:
		return nil, invalid("unknown bill status %q", status)
	}
	order := " ORDER BY b.bill_date DESC, b.created_at DESC LIMIT 1000"
	if status == protocol.BillOpen || status == "overdue" {
		order = " ORDER BY b.due_date, b.bill_date LIMIT 1000"
	}
	return queryBills(ctx, h.db.R(), where+order, args...)
}

// GetBill loads one bill and the payments made against it.
func (h *Hub) GetBill(ctx context.Context, by *Session, id string) (*protocol.SupplierBill, []protocol.SupplierPayment, error) {
	if !by.Can(managerRoles...) {
		return nil, nil, ErrForbidden
	}
	list, err := queryBills(ctx, h.db.R(), "b.id=? OR b.bill_no=?", id, id)
	if err != nil {
		return nil, nil, err
	}
	if len(list) == 0 {
		return nil, nil, invalid("bill not found")
	}
	pays, err := h.queryPayments(ctx, h.db.R(), "p.id IN (SELECT payment_id FROM supplier_allocations WHERE bill_id=?) ORDER BY p.paid_at", list[0].ID)
	return &list[0], pays, err
}

// CreateBill records a supplier's invoice, optionally for one delivery.
func (h *Hub) CreateBill(ctx context.Context, by *Session, in protocol.SupplierBillInput) (string, error) {
	if !by.Can(managerRoles...) {
		return "", ErrForbidden
	}
	if in.AmountMinor < 0 {
		return "", invalid("amount cannot be negative")
	}
	id := ids.New()
	return id, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if in.GRNID != "" {
			var grnSupplier, grnNo string
			var total int64
			err := tx.QueryRowContext(ctx, "SELECT supplier_id,grn_no,total_minor FROM goods_receipts WHERE id=?", in.GRNID).Scan(&grnSupplier, &grnNo, &total)
			if errors.Is(err, sql.ErrNoRows) {
				return invalid("delivery not found")
			}
			if err != nil {
				return err
			}
			if in.SupplierID == "" {
				in.SupplierID = grnSupplier
			}
			if grnSupplier != "" && grnSupplier != in.SupplierID {
				return invalid("%s came from a different supplier", grnNo)
			}
			var billed string
			tx.QueryRowContext(ctx, "SELECT bill_no FROM supplier_bills WHERE grn_id=? AND status<>'void'", in.GRNID).Scan(&billed)
			if billed != "" {
				return invalid("%s is already on bill %s", grnNo, billed)
			}
			if in.AmountMinor == 0 {
				in.AmountMinor = total
			}
		}
		if in.SupplierID == "" {
			return invalid("choose a supplier")
		}
		var terms int64
		if err := tx.QueryRowContext(ctx, "SELECT terms_days FROM suppliers WHERE id=?", in.SupplierID).Scan(&terms); errors.Is(err, sql.ErrNoRows) {
			return invalid("supplier not found")
		} else if err != nil {
			return err
		}
		if in.AmountMinor <= 0 {
			return invalid("enter the bill amount")
		}
		now := ids.NowMs()
		if in.BillDate == 0 {
			in.BillDate = startOfToday()
		}
		if in.DueDate == 0 {
			in.DueDate = in.BillDate + terms*dayMs
		}
		if in.DueDate < in.BillDate {
			return invalid("the due date is before the bill date")
		}
		ref := trim(in.SupplierRef)
		if ref != "" {
			var dup string
			tx.QueryRowContext(ctx, "SELECT bill_no FROM supplier_bills WHERE supplier_id=? AND supplier_ref=? COLLATE NOCASE AND status<>'void'", in.SupplierID, ref).Scan(&dup)
			if dup != "" {
				return invalid("invoice %s from this supplier is already recorded as %s", ref, dup)
			}
		}
		no, err := nextDocNo(ctx, tx, "bill", "BILL")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO supplier_bills(id,bill_no,supplier_id,supplier_ref,grn_id,bill_date,due_date,amount_minor,paid_minor,status,note,created_by,created_at)
			VALUES(?,?,?,?,?,?,?,?,0,'open',?,?,?)`, id, no, in.SupplierID, ref, in.GRNID, in.BillDate, in.DueDate, in.AmountMinor, trim(in.Note), by.UserID, now); err != nil {
			return err
		}
		note := ref
		if note != "" {
			note = "Invoice " + note
		}
		if err := postSupplierLedger(ctx, tx, supplierLedgerEntry{SupplierID: in.SupplierID, Kind: "bill", RefID: id, RefNo: no, Amount: in.AmountMinor,
			At: in.BillDate, Note: note, UserName: by.Name}); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "supplier.bill", "supplier_bill", id, map[string]any{"no": no, "amount": in.AmountMinor, "ref": ref})
		// Use up any credit left by earlier overpayments.
		return applySupplierCredit(ctx, tx, in.SupplierID)
	})
}

// VoidBill cancels a bill entered by mistake. A bill with payments against
// it cannot be voided.
func (h *Hub) VoidBill(ctx context.Context, by *Session, id, reason string) error {
	if !by.Can(managerRoles...) {
		return ErrForbidden
	}
	if trim(reason) == "" {
		return invalid("give a reason for voiding the bill")
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		list, err := queryBills(ctx, tx, "b.id=?", id)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return invalid("bill not found")
		}
		b := list[0]
		if b.Status == protocol.BillVoid {
			return invalid("%s is already void", b.BillNo)
		}
		if b.PaidMinor > 0 {
			return invalid("%s has payments against it and cannot be voided", b.BillNo)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE supplier_bills SET status='void', note=? WHERE id=?", trim(b.Note+"\nVoid: "+reason), id); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "supplier.bill_void", "supplier_bill", id, map[string]any{"no": b.BillNo, "reason": reason})
		return postSupplierLedger(ctx, tx, supplierLedgerEntry{SupplierID: b.SupplierID, Kind: "void", RefID: id, RefNo: b.BillNo,
			Amount: -b.AmountMinor, Note: trim(reason), UserName: by.Name})
	})
}

// ---------- payments ----------

const paymentSelect = `SELECT p.id,p.payment_no,p.supplier_id,COALESCE(s.name,''),p.method,p.reference,p.amount_minor,p.paid_at,p.note,
	COALESCE(u.name,p.created_by),p.created_at
	FROM supplier_payments p LEFT JOIN suppliers s ON s.id=p.supplier_id LEFT JOIN users u ON u.id=p.created_by`

func (h *Hub) queryPayments(ctx context.Context, q rowQuerier, where string, args ...any) ([]protocol.SupplierPayment, error) {
	rows, err := q.QueryContext(ctx, paymentSelect+" WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.SupplierPayment{}
	for rows.Next() {
		var p protocol.SupplierPayment
		if err := rows.Scan(&p.ID, &p.PaymentNo, &p.SupplierID, &p.SupplierName, &p.Method, &p.Reference, &p.AmountMinor, &p.PaidAt,
			&p.Note, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListSupplierPayments lists payments made, newest first.
func (h *Hub) ListSupplierPayments(ctx context.Context, by *Session, supplierID string) ([]protocol.SupplierPayment, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	if supplierID != "" {
		return h.queryPayments(ctx, h.db.R(), "p.supplier_id=? ORDER BY p.paid_at DESC, p.created_at DESC LIMIT 1000", supplierID)
	}
	return h.queryPayments(ctx, h.db.R(), "1=1 ORDER BY p.paid_at DESC, p.created_at DESC LIMIT 1000")
}

// GetSupplierPayment loads a payment and the bills it paid.
func (h *Hub) GetSupplierPayment(ctx context.Context, by *Session, id string) (*protocol.SupplierPayment, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	list, err := h.queryPayments(ctx, h.db.R(), "p.id=? OR p.payment_no=?", id, id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, invalid("payment not found")
	}
	p := list[0]
	rows, err := h.db.R().QueryContext(ctx, `SELECT a.bill_id,b.bill_no,b.supplier_ref,a.amount_minor FROM supplier_allocations a
		JOIN supplier_bills b ON b.id=a.bill_id WHERE a.payment_id=? ORDER BY b.due_date`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a protocol.SupplierAllocation
		if err := rows.Scan(&a.BillID, &a.BillNo, &a.SupplierRef, &a.AmountMinor); err != nil {
			return nil, err
		}
		p.Allocations = append(p.Allocations, a)
	}
	return &p, rows.Err()
}

// PaySupplier records money paid to a supplier and allocates it to bills.
func (h *Hub) PaySupplier(ctx context.Context, by *Session, in protocol.SupplierPaymentInput) (string, error) {
	if !by.Can(managerRoles...) {
		return "", ErrForbidden
	}
	if in.AmountMinor <= 0 {
		return "", invalid("enter the amount paid")
	}
	if trim(in.Method) == "" {
		return "", invalid("choose how the supplier was paid")
	}
	id := ids.New()
	return id, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, ok := supplierName(ctx, tx, in.SupplierID); !ok {
			return invalid("supplier not found")
		}
		now := ids.NowMs()
		if in.PaidAt == 0 {
			in.PaidAt = now
		}
		if in.PaidAt > now+dayMs {
			return invalid("the payment date is in the future")
		}
		no, err := nextDocNo(ctx, tx, "supplier_payment", "SP")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO supplier_payments(id,payment_no,supplier_id,method,reference,amount_minor,paid_at,note,created_by,created_at)
			VALUES(?,?,?,?,?,?,?,?,?,?)`, id, no, in.SupplierID, trim(in.Method), trim(in.Reference), in.AmountMinor, in.PaidAt, trim(in.Note), by.UserID, now); err != nil {
			return err
		}
		if err := postSupplierLedger(ctx, tx, supplierLedgerEntry{SupplierID: in.SupplierID, Kind: "payment", RefID: id, RefNo: no,
			Amount: -in.AmountMinor, At: in.PaidAt, Note: trim(in.Method + " " + in.Reference), UserName: by.Name}); err != nil {
			return err
		}
		for _, billID := range in.BillIDs {
			var sup, status string
			if err := tx.QueryRowContext(ctx, "SELECT supplier_id,status FROM supplier_bills WHERE id=?", billID).Scan(&sup, &status); err != nil || sup != in.SupplierID {
				return invalid("that bill is not from this supplier")
			}
			if status != protocol.BillOpen {
				return invalid("that bill is already %s", status)
			}
		}
		audit(ctx, tx, by.UserID, "", "supplier.payment", "supplier_payment", id, map[string]any{"no": no, "amount": in.AmountMinor, "method": in.Method})
		return allocatePayment(ctx, tx, id, in.SupplierID, in.AmountMinor, in.BillIDs)
	})
}

// allocatePayment spreads amount over the chosen bills, then the oldest
// open bills. It returns nothing left over: unallocated money is a credit,
// worked out later as payments minus allocations.
func allocatePayment(ctx context.Context, tx *sql.Tx, paymentID, supplierID string, amount int64, first []string) error {
	order := append([]string{}, first...)
	rows, err := tx.QueryContext(ctx, "SELECT id FROM supplier_bills WHERE supplier_id=? AND status='open' ORDER BY due_date, bill_date, created_at", supplierID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		order = append(order, id)
	}
	rows.Close()
	done := map[string]bool{}
	for _, billID := range order {
		if amount <= 0 {
			break
		}
		if done[billID] {
			continue
		}
		done[billID] = true
		var owed int64
		if err := tx.QueryRowContext(ctx, "SELECT amount_minor-paid_minor FROM supplier_bills WHERE id=? AND status='open'", billID).Scan(&owed); err != nil || owed <= 0 {
			continue
		}
		pay := min(owed, amount)
		amount -= pay
		if _, err := tx.ExecContext(ctx, `INSERT INTO supplier_allocations(payment_id,bill_id,amount_minor) VALUES(?,?,?)
			ON CONFLICT(payment_id,bill_id) DO UPDATE SET amount_minor=amount_minor+excluded.amount_minor`, paymentID, billID, pay); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE supplier_bills SET paid_minor=paid_minor+?,
			status=CASE WHEN paid_minor+?>=amount_minor THEN 'paid' ELSE 'open' END WHERE id=?`, pay, pay, billID); err != nil {
			return err
		}
	}
	return nil
}

// applySupplierCredit allocates unallocated payment money (oldest payment
// first) to the supplier's open bills.
func applySupplierCredit(ctx context.Context, tx *sql.Tx, supplierID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT p.id, p.amount_minor-COALESCE((SELECT sum(amount_minor) FROM supplier_allocations a WHERE a.payment_id=p.id),0)
		FROM supplier_payments p WHERE p.supplier_id=? ORDER BY p.paid_at, p.created_at`, supplierID)
	if err != nil {
		return err
	}
	type credit struct {
		id     string
		amount int64
	}
	var credits []credit
	for rows.Next() {
		var c credit
		if err := rows.Scan(&c.id, &c.amount); err != nil {
			rows.Close()
			return err
		}
		if c.amount > 0 {
			credits = append(credits, c)
		}
	}
	rows.Close()
	for _, c := range credits {
		if err := allocatePayment(ctx, tx, c.id, supplierID, c.amount, nil); err != nil {
			return err
		}
	}
	return nil
}

// ---------- statement & ageing ----------

// SupplierStatement lists every change to what we owe a supplier, oldest
// first, with the bills still open.
func (h *Hub) SupplierStatement(ctx context.Context, by *Session, supplierID string) (*protocol.SupplierStatement, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	sups, err := h.ListSuppliers(ctx, by)
	if err != nil {
		return nil, err
	}
	out := &protocol.SupplierStatement{Entries: []protocol.SupplierLedgerEntry{}}
	found := false
	for _, s := range sups {
		if s.ID == supplierID {
			out.Supplier, found = s, true
		}
	}
	if !found {
		return nil, invalid("supplier not found")
	}
	r := h.db.R()
	rows, err := r.QueryContext(ctx, `SELECT id,at,kind,ref_id,ref_no,amount_minor,note,user_name FROM supplier_ledger WHERE supplier_id=? ORDER BY at, rowid`, supplierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bal int64
	for rows.Next() {
		var e protocol.SupplierLedgerEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Kind, &e.RefID, &e.RefNo, &e.AmountMinor, &e.Note, &e.UserName); err != nil {
			return nil, err
		}
		bal += e.AmountMinor
		e.BalanceMinor = bal
		out.Entries = append(out.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.Bills, err = queryBills(ctx, r, "b.supplier_id=? AND b.status='open' ORDER BY b.due_date", supplierID)
	return out, err
}

// PayablesReport splits unpaid bills by how long past due they are.
func (h *Hub) PayablesReport(ctx context.Context, by *Session) (*protocol.PayablesReport, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	today := startOfToday()
	r := h.db.R()
	rows, err := r.QueryContext(ctx, `SELECT s.id, s.name, b.due_date, b.amount_minor-b.paid_minor
		FROM supplier_bills b JOIN suppliers s ON s.id=b.supplier_id WHERE b.status='open' ORDER BY s.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	out := &protocol.PayablesReport{AsOf: today, Rows: []protocol.PayablesRow{}}
	idx := map[string]int{}
	row := func(id, name string) *protocol.PayablesRow {
		i, ok := idx[id]
		if !ok {
			i = len(out.Rows)
			idx[id] = i
			out.Rows = append(out.Rows, protocol.PayablesRow{SupplierID: id, SupplierName: name})
		}
		return &out.Rows[i]
	}
	for rows.Next() {
		var id, name string
		var due, owed int64
		if err := rows.Scan(&id, &name, &due, &owed); err != nil {
			rows.Close()
			return nil, err
		}
		p := row(id, name)
		late := (today - due) / dayMs
		switch {
		case due >= today:
			p.NotDueMinor += owed
		case late <= 30:
			p.Days1To30 += owed
		case late <= 60:
			p.Days31To60 += owed
		case late <= 90:
			p.Days61To90 += owed
		default:
			p.Over90Minor += owed
		}
		p.TotalMinor += owed
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	credits, err := r.QueryContext(ctx, `SELECT s.id, s.name, sum(p.amount_minor-COALESCE((SELECT sum(amount_minor) FROM supplier_allocations a WHERE a.payment_id=p.id),0)) c
		FROM supplier_payments p JOIN suppliers s ON s.id=p.supplier_id GROUP BY s.id HAVING c>0`)
	if err != nil {
		return nil, err
	}
	defer credits.Close()
	for credits.Next() {
		var id, name string
		var c int64
		if err := credits.Scan(&id, &name, &c); err != nil {
			return nil, err
		}
		row(id, name).CreditMinor = c
	}
	t := &out.Totals
	for _, p := range out.Rows {
		t.NotDueMinor += p.NotDueMinor
		t.Days1To30 += p.Days1To30
		t.Days31To60 += p.Days31To60
		t.Days61To90 += p.Days61To90
		t.Over90Minor += p.Over90Minor
		t.TotalMinor += p.TotalMinor
		t.CreditMinor += p.CreditMinor
	}
	return out, credits.Err()
}
