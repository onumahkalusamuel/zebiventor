package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// IncomingTransfer is stock on its way to this till's location.
type IncomingTransfer struct {
	protocol.StockTransfer
	// ReceivedHere: booked in at this till, not yet confirmed by the hub.
	ReceivedHere bool `json:"received_here"`
}

var receiveRoles = []string{protocol.RoleAdmin, protocol.RoleManager, protocol.RoleInventory}

// saveTransfers stores transfers pulled from the hub.
func saveTransfers(ctx context.Context, tx *sql.Tx, list []protocol.StockTransfer) error {
	now := ids.NowMs()
	for _, tr := range list {
		b, err := json.Marshal(tr)
		if err != nil {
			return err
		}
		// Once the hub has settled it, the local "received" flag is done with.
		if _, err := tx.ExecContext(ctx, `INSERT INTO transfers(id,transfer_no,from_location_id,to_location_id,status,data,updated_at) VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET status=excluded.status, data=excluded.data, updated_at=excluded.updated_at,
			received_locally=CASE WHEN excluded.status='in_transit' THEN transfers.received_locally ELSE 0 END`,
			tr.ID, tr.TransferNo, tr.FromLocationID, tr.ToLocationID, tr.Status, string(b), now); err != nil {
			return err
		}
	}
	if len(list) > 0 {
		_, err := tx.ExecContext(ctx, "DELETE FROM transfers WHERE status<>'in_transit' AND updated_at < ?", now-60*24*3600*1000)
		return err
	}
	return nil
}

// IncomingTransfers lists transfers on their way to this till's location.
func (t *Terminal) IncomingTransfers(ctx context.Context) ([]IncomingTransfer, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired")
	}
	rows, err := t.db.R().QueryContext(ctx, `SELECT data, received_locally FROM transfers
		WHERE to_location_id=? AND status='in_transit' ORDER BY transfer_no`, p.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IncomingTransfer{}
	for rows.Next() {
		var data string
		var it IncomingTransfer
		if err := rows.Scan(&data, &it.ReceivedHere); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &it.StockTransfer); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReceiveTransfer books a transfer in at this till. It works offline: the
// receipt is queued and reaches the hub with the next sync.
func (t *Terminal) ReceiveTransfer(ctx context.Context, u User, id string, in protocol.TransferReceipt) error {
	if !slices.Contains(receiveRoles, u.Role) {
		return errors.New("receiving stock needs a manager or stock keeper")
	}
	p, ok := t.Pairing(ctx)
	if !ok {
		return errors.New("this till is not paired")
	}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var data string
		var done bool
		err := tx.QueryRowContext(ctx, "SELECT data, received_locally FROM transfers WHERE id=? AND status='in_transit'", id).Scan(&data, &done)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("this transfer is no longer on its way here")
		}
		if err != nil {
			return err
		}
		if done {
			return errors.New("this transfer has already been received here")
		}
		var tr protocol.StockTransfer
		if err := json.Unmarshal([]byte(data), &tr); err != nil {
			return err
		}
		if tr.ToLocationID != p.LocationID {
			return errors.New("this transfer is going to " + tr.ToLocationName + ", not this till's location")
		}
		sent := map[string]protocol.StockTransferLine{}
		for _, l := range tr.Lines {
			sent[l.VariantID] = l
		}
		for _, l := range in.Lines {
			s, ok := sent[l.VariantID]
			switch {
			case !ok:
				return errors.New("an item that was not sent is listed")
			case l.ReceivedQtyMilli < 0:
				return errors.New("received quantity cannot be negative")
			case l.ReceivedQtyMilli > s.QtyMilli:
				return errors.New("more " + s.ProductName + " received than was sent (" + strconv.FormatInt(s.QtyMilli/1000, 10) + ")")
			}
		}
		now := ids.NowMs()
		op := protocol.TransferReceiveOp{TransferID: id, UserID: u.ID, ReceivedAt: now, TransferReceipt: in}
		if err := enqueue(ctx, tx, protocol.OpTransferIn, id, op, now); err != nil {
			return err
		}
		b, _ := json.Marshal(in)
		_, err = tx.ExecContext(ctx, "UPDATE transfers SET received_locally=1, local_receipt=? WHERE id=?", string(b), id)
		return err
	})
	if err == nil {
		t.log.Info("transfer received at till", "transfer", id, "user", u.Username)
		t.refreshCounts(ctx)
		t.Nudge()
	}
	return err
}
