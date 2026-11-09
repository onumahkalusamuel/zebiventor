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
			ON CONFLICT(id) DO UPDATE SET status=excluded.status, data=excluded.data, updated_at=excluded.updated_at, sent_locally=0,
			transfer_no=excluded.transfer_no,
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
		WHERE to_location_id=? AND status='in_transit' AND sent_locally=0 ORDER BY transfer_no`, p.LocationID)
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

// ---------- sending stock from this till (works offline) ----------

// Location is a place stock can be sent to.
type Location struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Locations lists the store's active locations other than this till's.
func (t *Terminal) Locations(ctx context.Context) ([]Location, error) {
	p, _ := t.Pairing(ctx)
	rows, err := t.db.R().QueryContext(ctx, "SELECT id,name FROM locations WHERE active=1 AND id<>? ORDER BY name", p.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Location{}
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SendTransferInput is stock sent from this till's location.
type SendTransferInput struct {
	ToLocationID string                       `json:"to_location_id"`
	Note         string                       `json:"note"`
	Lines        []protocol.StockTransferLine `json:"lines"`
}

// SendTransfer sends stock from this till's location to another. It is
// queued and the hub numbers it when it arrives (TR-…).
func (t *Terminal) SendTransfer(ctx context.Context, u User, in SendTransferInput) (*protocol.StockTransfer, error) {
	if !slices.Contains(receiveRoles, u.Role) {
		return nil, errors.New("sending stock needs a manager or stock keeper")
	}
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired")
	}
	tr := protocol.StockTransfer{ID: ids.New(), FromLocationID: p.LocationID, ToLocationID: in.ToLocationID, Note: in.Note,
		CreatedBy: u.Name, CreatedAt: ids.NowMs(), Status: protocol.TransferInTransit}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_ = tx.QueryRowContext(ctx, "SELECT name FROM locations WHERE id=?", p.LocationID).Scan(&tr.FromLocationName)
		if err := tx.QueryRowContext(ctx, "SELECT name FROM locations WHERE id=? AND active=1", in.ToLocationID).Scan(&tr.ToLocationName); err != nil {
			return errors.New("choose where the stock is going")
		}
		if in.ToLocationID == p.LocationID {
			return errors.New("choose another location")
		}
		seen := map[string]bool{}
		for _, l := range in.Lines {
			if l.QtyMilli <= 0 {
				continue
			}
			if seen[l.VariantID] {
				return errors.New("an item is listed twice")
			}
			seen[l.VariantID] = true
			var track, fraction bool
			err := tx.QueryRowContext(ctx, `SELECT p.name, v.name, v.sku, p.unit, p.track_stock, p.allow_fraction FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`,
				l.VariantID).Scan(&l.ProductName, &l.VariantName, &l.SKU, &l.Unit, &track, &fraction)
			if err != nil {
				return errors.New("item not found")
			}
			if !track {
				return errors.New(l.ProductName + " does not track stock")
			}
			if !fraction && l.QtyMilli%1000 != 0 {
				return errors.New(l.ProductName + " is sent in whole units")
			}
			tr.Lines = append(tr.Lines, l)
			tr.ItemsMilli += l.QtyMilli
		}
		if len(tr.Lines) == 0 {
			return errors.New("add the items being sent")
		}
		op := protocol.TransferSendOp{ID: tr.ID, ToLocationID: tr.ToLocationID, Note: tr.Note, Lines: tr.Lines, UserID: u.ID, SentAt: tr.CreatedAt}
		if err := enqueue(ctx, tx, protocol.OpTransferOut, tr.ID, op, tr.CreatedAt); err != nil {
			return err
		}
		b, _ := json.Marshal(tr)
		_, err := tx.ExecContext(ctx, `INSERT INTO transfers(id,transfer_no,from_location_id,to_location_id,status,data,sent_locally,updated_at)
			VALUES(?,?,?,?,?,?,1,?)`, tr.ID, "", tr.FromLocationID, tr.ToLocationID, tr.Status, string(b), tr.CreatedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	t.log.Info("transfer sent from till", "transfer", tr.ID, "to", tr.ToLocationName)
	t.refreshCounts(ctx)
	t.Nudge()
	return &tr, nil
}

// OutgoingTransfers lists transfers sent from this till that the hub
// hasn't confirmed yet.
func (t *Terminal) OutgoingTransfers(ctx context.Context) ([]protocol.StockTransfer, error) {
	rows, err := t.db.R().QueryContext(ctx, "SELECT data FROM transfers WHERE sent_locally=1 ORDER BY updated_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.StockTransfer{}
	for rows.Next() {
		var data string
		var tr protocol.StockTransfer
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(data), &tr) == nil {
			out = append(out, tr)
		}
	}
	return out, rows.Err()
}

// ---------- deliveries expected at this till (works offline) ----------

// ExpectedDelivery is a purchase order still due at this till's location.
type ExpectedDelivery struct {
	Order        protocol.PurchaseOrder `json:"order"`
	ReceivedHere bool                   `json:"received_here"`
}

func savePurchaseOrders(ctx context.Context, tx *sql.Tx, list []protocol.PurchaseOrder) error {
	now := ids.NowMs()
	for _, o := range list {
		b, err := json.Marshal(o)
		if err != nil {
			return err
		}
		// A delivery booked here is settled once the hub's order changes.
		if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_orders(id,po_no,location_id,status,data,updated_at) VALUES(?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET po_no=excluded.po_no, location_id=excluded.location_id, status=excluded.status,
			data=excluded.data, updated_at=excluded.updated_at, received_locally=0`, o.ID, o.PONo, o.LocationID, o.Status, string(b), now); err != nil {
			return err
		}
	}
	if len(list) > 0 {
		_, err := tx.ExecContext(ctx, "DELETE FROM purchase_orders WHERE status NOT IN ('ordered','partial') AND updated_at < ?", now-60*24*3600*1000)
		return err
	}
	return nil
}

// ExpectedDeliveries lists purchase orders due at this till's location.
func (t *Terminal) ExpectedDeliveries(ctx context.Context) ([]ExpectedDelivery, error) {
	p, _ := t.Pairing(ctx)
	rows, err := t.db.R().QueryContext(ctx, `SELECT data, received_locally FROM purchase_orders
		WHERE location_id=? AND status IN ('ordered','partial') ORDER BY po_no`, p.LocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExpectedDelivery{}
	for rows.Next() {
		var data string
		var d ExpectedDelivery
		if err := rows.Scan(&data, &d.ReceivedHere); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(data), &d.Order) == nil {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// DeliveryLine is how much of an order line arrived.
type DeliveryLine struct {
	POLineID string `json:"po_line_id"`
	QtyMilli int64  `json:"qty_milli"`
}

// DeliveryInput is what arrived against a purchase order.
type DeliveryInput struct {
	SupplierRef string         `json:"supplier_ref"`
	Note        string         `json:"note"`
	Lines       []DeliveryLine `json:"lines"`
	ClosePO     bool           `json:"close_po"`
}

// ReceiveDelivery books in a delivery against a purchase order at this
// till, at the order's prices. Queued, so it works offline.
func (t *Terminal) ReceiveDelivery(ctx context.Context, u User, poID string, in DeliveryInput) error {
	if !slices.Contains(receiveRoles, u.Role) {
		return errors.New("receiving stock needs a manager or stock keeper")
	}
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var data string
		var done bool
		err := tx.QueryRowContext(ctx, "SELECT data, received_locally FROM purchase_orders WHERE id=? AND status IN ('ordered','partial')", poID).Scan(&data, &done)
		if err != nil {
			return errors.New("this order is no longer expected here")
		}
		if done {
			return errors.New("a delivery for this order is already waiting to reach the hub")
		}
		var o protocol.PurchaseOrder
		if err := json.Unmarshal([]byte(data), &o); err != nil {
			return err
		}
		byLine := map[string]protocol.PurchaseOrderLine{}
		for _, l := range o.Lines {
			byLine[l.ID] = l
		}
		gi := protocol.GoodsReceiptInput{POID: o.ID, SupplierID: o.SupplierID, SupplierRef: in.SupplierRef, Note: in.Note, ClosePO: in.ClosePO}
		for _, l := range in.Lines {
			pl, ok := byLine[l.POLineID]
			if !ok {
				return errors.New("an item that isn't on the order is listed")
			}
			if l.QtyMilli < 0 {
				return errors.New("quantities cannot be negative")
			}
			if l.QtyMilli == 0 {
				continue
			}
			gi.Lines = append(gi.Lines, protocol.GoodsReceiptLine{POLineID: pl.ID, VariantID: pl.VariantID, QtyMilli: l.QtyMilli, UnitCostMinor: pl.UnitCostMinor})
		}
		if len(gi.Lines) == 0 {
			return errors.New("enter what arrived")
		}
		now := ids.NowMs()
		op := protocol.GoodsReceiveOp{ID: ids.New(), UserID: u.ID, ReceivedAt: now, GoodsReceiptInput: gi}
		if err := enqueue(ctx, tx, protocol.OpGoodsIn, o.ID, op, now); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE purchase_orders SET received_locally=1 WHERE id=?", o.ID)
		return err
	})
	if err == nil {
		t.log.Info("delivery received at till", "po", poID, "user", u.Username)
		t.refreshCounts(ctx)
		t.Nudge()
	}
	return err
}
