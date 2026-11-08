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

// applyAccountPayment records money a customer paid against their account.
// Like a sale, it already happened at the till, so it is never refused for
// business reasons (an overpayment simply leaves the customer in credit).
func (h *Hub) applyAccountPayment(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var p protocol.AccountPayment
	if err := json.Unmarshal(op.Payload, &p); err != nil || p.ID == "" || p.PaymentNo == "" {
		return invalid("bad account payment payload")
	}
	if !strings.HasPrefix(p.PaymentNo, t.ReceiptPrefix+"-") {
		return invalid("payment number %q does not belong to terminal %s", p.PaymentNo, t.ReceiptPrefix)
	}
	if p.AmountMinor <= 0 {
		return invalid("payment amount must be positive")
	}
	var exists int
	tx.QueryRowContext(ctx, "SELECT count(*) FROM account_payments WHERE id=?", p.ID).Scan(&exists)
	if exists > 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_payments(id,payment_no,customer_id,terminal_id,shift_id,method_id,method_name,kind,amount_minor,
		reference,note,user_id,user_name,created_at,received_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.PaymentNo, p.CustomerID, t.ID, p.ShiftID, p.MethodID, p.MethodName, p.Kind, p.AmountMinor,
		p.Reference, p.Note, p.UserID, p.UserName, p.CreatedAt, ids.NowMs()); err != nil {
		return err
	}
	return postLedger(ctx, tx, ledgerEntry{CustomerID: p.CustomerID, Kind: "payment", RefID: p.ID, RefNo: p.PaymentNo, Amount: -p.AmountMinor,
		At: p.CreatedAt, Note: p.Reference, UserID: p.UserID, UserName: p.UserName, TerminalID: t.ID})
}

// CustomerStatement lists every change to a customer's balance, oldest first.
func (h *Hub) CustomerStatement(ctx context.Context, by *Session, customerID string) (*protocol.CustomerStatement, error) {
	if !by.Can(managerRoles...) {
		return nil, ErrForbidden
	}
	r := h.db.R()
	out := &protocol.CustomerStatement{Entries: []protocol.CustomerLedgerEntry{}}
	c := &out.Customer
	err := r.QueryRowContext(ctx, `SELECT id,name,phone,email,notes,active,balance_minor,updated_at FROM customers WHERE id=?`, customerID).
		Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.Active, &c.BalanceMinor, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("customer not found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.QueryContext(ctx, `SELECT id,at,kind,ref_id,ref_no,amount_minor,note,user_name FROM customer_ledger WHERE customer_id=? ORDER BY at, id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bal int64
	for rows.Next() {
		var e protocol.CustomerLedgerEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Kind, &e.RefID, &e.RefNo, &e.AmountMinor, &e.Note, &e.UserName); err != nil {
			return nil, err
		}
		bal += e.AmountMinor
		e.BalanceMinor = bal
		out.Entries = append(out.Entries, e)
	}
	return out, rows.Err()
}

// AdjustCustomerBalance records an opening balance or a correction (e.g. a
// write-off) on a customer's account.
func (h *Hub) AdjustCustomerBalance(ctx context.Context, by *Session, customerID string, in protocol.BalanceAdjustment) error {
	if !by.Can(adminRoles...) {
		return ErrForbidden
	}
	if in.AmountMinor == 0 {
		return invalid("enter an amount")
	}
	if strings.TrimSpace(in.Note) == "" {
		return invalid("give a reason for the adjustment")
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var n int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM customers WHERE id=?", customerID).Scan(&n)
		if n == 0 {
			return invalid("customer not found")
		}
		id := ids.New()
		audit(ctx, tx, by.UserID, "", "customer.balance_adjust", "customer", customerID, map[string]any{"amount": in.AmountMinor, "note": in.Note})
		return postLedger(ctx, tx, ledgerEntry{CustomerID: customerID, Kind: "adjustment", RefID: id, Amount: in.AmountMinor,
			Note: strings.TrimSpace(in.Note), UserID: by.UserID, UserName: by.Name})
	})
}
