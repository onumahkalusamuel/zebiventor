package terminal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// AccountPaymentInput is money a customer pays towards what they owe.
type AccountPaymentInput struct {
	CustomerID  string `json:"customer_id"`
	MethodID    string `json:"method_id"`
	AmountMinor int64  `json:"amount_minor"`
	Reference   string `json:"reference"`
	Note        string `json:"note"`
}

// pendingBalance is the change to a customer's balance from this till's
// transactions the hub has not acknowledged yet (credit sales, credit
// refunds and payments), so the till shows an up-to-date balance offline.
const pendingBalance = `(COALESCE((SELECT sum(p.amount_minor) FROM sale_payments p JOIN sales s ON s.id=p.sale_id
		WHERE s.customer_id=customers.id AND s.synced_at=0 AND p.kind='credit'),0)
	- COALESCE((SELECT sum(f.amount_minor) FROM return_refunds f JOIN returns r ON r.id=f.return_id
		WHERE r.customer_id=customers.id AND r.synced_at=0 AND f.kind='credit'),0)
	- COALESCE((SELECT sum(amount_minor) FROM account_payments a WHERE a.customer_id=customers.id AND a.synced_at=0),0))`

// RecordAccountPayment takes a payment on a customer's account. It works
// offline, belongs to the open shift (cash goes into the drawer count) and
// is pushed to the hub, which reduces the balance.
func (t *Terminal) RecordAccountPayment(ctx context.Context, u User, in AccountPaymentInput) (*protocol.AccountPayment, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired with a hub")
	}
	if in.AmountMinor <= 0 {
		return nil, saleErr("enter the amount received")
	}
	shiftID := t.kv(ctx, "shift_id")
	if shiftID == "" {
		return nil, saleErr("open a shift before taking payments")
	}
	pay := &protocol.AccountPayment{ID: ids.New(), CustomerID: in.CustomerID, TerminalID: p.TerminalID, ShiftID: shiftID,
		AmountMinor: in.AmountMinor, Reference: strings.TrimSpace(in.Reference), Note: strings.TrimSpace(in.Note), UserID: u.ID, UserName: u.Name}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, "SELECT name, balance_minor + "+pendingBalance+" FROM customers WHERE id=?", in.CustomerID).
			Scan(&pay.CustomerName, &pay.BalanceBefore)
		if err != nil {
			return saleErr("choose a customer")
		}
		var active, needsRef bool
		err = tx.QueryRowContext(ctx, "SELECT name, kind, active, requires_reference FROM payment_methods WHERE id=?", in.MethodID).
			Scan(&pay.MethodName, &pay.Kind, &active, &needsRef)
		if err != nil || !active {
			return saleErr("choose how the customer is paying")
		}
		if pay.Kind == protocol.PayCredit {
			return saleErr("a payment on account cannot itself be put on account")
		}
		if needsRef && pay.Reference == "" {
			return saleErr("enter the %s reference", pay.MethodName)
		}
		pay.CreatedAt = ids.NowMs()
		seq, err := nextSeq(ctx, tx, "payment_seq")
		if err != nil {
			return err
		}
		pay.PaymentNo = fmt.Sprintf("%s-P%05d", p.ReceiptPrefix, seq)
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_payments(id,payment_no,customer_id,customer_name,shift_id,method_id,method_name,kind,
			amount_minor,reference,note,user_id,user_name,balance_before_minor,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			pay.ID, pay.PaymentNo, pay.CustomerID, pay.CustomerName, pay.ShiftID, in.MethodID, pay.MethodName, pay.Kind, pay.AmountMinor,
			pay.Reference, pay.Note, pay.UserID, pay.UserName, pay.BalanceBefore, pay.CreatedAt); err != nil {
			return err
		}
		pay.MethodID = in.MethodID
		return enqueue(ctx, tx, protocol.OpAccountPayment, pay.ID, pay, pay.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	t.Nudge()
	return pay, nil
}

// GetAccountPayment loads a payment taken at this till.
func (t *Terminal) GetAccountPayment(ctx context.Context, id string) (*protocol.AccountPayment, error) {
	pay := &protocol.AccountPayment{}
	err := t.db.R().QueryRowContext(ctx, `SELECT id,payment_no,customer_id,customer_name,shift_id,method_id,method_name,kind,amount_minor,reference,note,
		user_id,user_name,balance_before_minor,created_at,synced_at FROM account_payments WHERE id=? OR payment_no=?`, id, id).
		Scan(&pay.ID, &pay.PaymentNo, &pay.CustomerID, &pay.CustomerName, &pay.ShiftID, &pay.MethodID, &pay.MethodName, &pay.Kind, &pay.AmountMinor,
			&pay.Reference, &pay.Note, &pay.UserID, &pay.UserName, &pay.BalanceBefore, &pay.CreatedAt, &pay.SyncedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, saleErr("payment not found on this till")
	}
	if p, ok := t.Pairing(ctx); ok {
		pay.TerminalID = p.TerminalID
	}
	return pay, err
}
