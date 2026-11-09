package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// ---------- locations ----------

// LocationInfo is a location with what is stored and selling there.
type LocationInfo struct {
	protocol.Location
	Terminals  int64 `json:"terminals"`
	ValueMinor int64 `json:"value_minor"`
	IsDefault  bool  `json:"is_default"`
}

// ListLocations returns all locations (shop floors, stores, warehouses).
func (h *Hub) ListLocations(ctx context.Context, by *Session) ([]LocationInfo, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	def := defaultLocation(ctx, h.db.R())
	rows, err := h.db.R().QueryContext(ctx, `SELECT l.id,l.name,l.code,l.active,
		(SELECT count(*) FROM terminals t WHERE t.location_id=l.id AND t.active=1),
		(SELECT COALESCE(sum(s.qty_milli*v.cost_minor/1000),0) FROM stock_levels s JOIN variants v ON v.id=s.variant_id WHERE s.location_id=l.id AND s.qty_milli>0)
		FROM locations l ORDER BY l.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LocationInfo{}
	for rows.Next() {
		var l LocationInfo
		if err := rows.Scan(&l.ID, &l.Name, &l.Code, &l.Active, &l.Terminals, &l.ValueMinor); err != nil {
			return nil, err
		}
		l.IsDefault = l.ID == def
		out = append(out, l)
	}
	return out, rows.Err()
}

// SaveLocation creates or renames a location.
func (h *Hub) SaveLocation(ctx context.Context, by *Session, l protocol.Location) (string, error) {
	if !by.Can(managerRoles...) {
		return "", ErrForbidden
	}
	l.Name, l.Code = trim(l.Name), trim(l.Code)
	if l.Name == "" {
		return "", invalid("location name is required")
	}
	if l.ID == "" {
		l.ID, l.Active = ids.New(), true
	}
	return l.ID, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		if !l.Active {
			if l.ID == defaultLocation(ctx, tx) {
				return invalid("the main location cannot be deactivated")
			}
			var tills int
			tx.QueryRowContext(ctx, "SELECT count(*) FROM terminals WHERE location_id=? AND active=1", l.ID).Scan(&tills)
			if tills > 0 {
				return invalid("tills are still selling from this location")
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO locations(id,name,code,active,created_at,updated_at) VALUES(?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, code=excluded.code, active=excluded.active, updated_at=excluded.updated_at`,
			l.ID, l.Name, l.Code, l.Active, now, now); err != nil {
			return err
		}
		return recordChange(ctx, tx, protocol.EntityLocation, l.ID)
	})
}

// ---------- transfers ----------

// TransferInput sends stock from one location to another. Goods leave the
// source at once and stay "in transit" until received at the destination,
// unless ReceiveNow is set (both places in the same building).
type TransferInput struct {
	FromLocationID string                       `json:"from_location_id"`
	ToLocationID   string                       `json:"to_location_id"`
	Note           string                       `json:"note"`
	ReceiveNow     bool                         `json:"receive_now"`
	Lines          []protocol.StockTransferLine `json:"lines"`
}

// TransferStock sends goods from one location to another.
func (h *Hub) TransferStock(ctx context.Context, by *Session, in TransferInput) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	if in.FromLocationID == in.ToLocationID {
		return "", invalid("choose two different locations")
	}
	if len(in.Lines) == 0 {
		return "", invalid("add at least one item to transfer")
	}
	id := ids.New()
	return id, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return createTransfer(ctx, tx, id, by.UserID, "", in, ids.NowMs())
	})
}

// createTransfer records a transfer sent by userID (from the back office,
// or queued at a till while offline) and takes the stock out of the source.
func createTransfer(ctx context.Context, tx *sql.Tx, id, userID, terminalID string, in TransferInput, now int64) error {
	if in.FromLocationID == in.ToLocationID {
		return invalid("choose two different locations")
	}
	if len(in.Lines) == 0 {
		return invalid("add at least one item to transfer")
	}
	if !locationExists(ctx, tx, in.FromLocationID) || !locationExists(ctx, tx, in.ToLocationID) {
		return invalid("unknown location")
	}
	no, err := nextDocNo(ctx, tx, "transfer", "TR")
	if err != nil {
		return err
	}
	status, recvBy, recvAt := protocol.TransferInTransit, "", int64(0)
	if in.ReceiveNow {
		status, recvBy, recvAt = protocol.TransferReceived, userID, now
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stock_transfers(id,transfer_no,from_location_id,to_location_id,note,created_by,created_at,status,received_by,received_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, id, no, in.FromLocationID, in.ToLocationID, trim(in.Note), userID, now, status, recvBy, recvAt); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, l := range in.Lines {
		v, err := loadVariant(ctx, tx, l.VariantID)
		if err != nil {
			return err
		}
		if !v.TrackStock {
			return invalid("%s does not track stock", v.label())
		}
		if seen[v.ID] {
			return invalid("%s is listed twice", v.label())
		}
		seen[v.ID] = true
		if err := v.checkQty(l.QtyMilli); err != nil {
			return err
		}
		received := int64(0)
		if in.ReceiveNow {
			received = l.QtyMilli
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stock_transfer_lines(id,transfer_id,variant_id,qty_milli,received_qty_milli) VALUES(?,?,?,?,?)`,
			ids.New(), id, v.ID, l.QtyMilli, received); err != nil {
			return err
		}
		move := stockMove{VariantID: v.ID, LocationID: in.FromLocationID, DeltaMilli: -l.QtyMilli, Reason: "transfer_out",
			RefType: "transfer", RefID: id, UserID: userID, CreatedAt: now}
		if err := addStock(ctx, tx, move); err != nil {
			return err
		}
		if in.ReceiveNow {
			move.LocationID, move.DeltaMilli, move.Reason = in.ToLocationID, l.QtyMilli, "transfer_in"
			if err := addStock(ctx, tx, move); err != nil {
				return err
			}
		}
	}
	audit(ctx, tx, userID, terminalID, "stock.transfer", "stock_transfer", id, map[string]any{"no": no, "lines": len(in.Lines), "status": status})
	return recordChange(ctx, tx, protocol.EntityTransfer, id)
}

// ReceiveTransfer books an in-transit transfer in at its destination. Any
// shortfall stays out of stock (it already left the source) and is noted.
func (h *Hub) ReceiveTransfer(ctx context.Context, by *Session, id string, in protocol.TransferReceipt) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		t, err := scanTransfer(tx.QueryRowContext(ctx, transferSelect+" WHERE t.id=?", id))
		if errors.Is(err, sql.ErrNoRows) {
			return invalid("transfer not found")
		}
		if err != nil {
			return err
		}
		return receiveTransfer(ctx, tx, &t, by.UserID, "", in, ids.NowMs())
	})
}

// receiveTransfer books transfer t in, as received by userID at time now.
func receiveTransfer(ctx context.Context, tx *sql.Tx, t *protocol.StockTransfer, userID, terminalID string, in protocol.TransferReceipt, now int64) error {
	id := t.ID
	if t.Status != protocol.TransferInTransit {
		return invalid("%s has already been %s", t.TransferNo, strings.ReplaceAll(t.Status, "_", " "))
	}
	got := map[string]int64{}
	for _, l := range in.Lines {
		if l.ReceivedQtyMilli < 0 {
			return invalid("received quantity cannot be negative")
		}
		got[l.VariantID] = l.ReceivedQtyMilli
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,variant_id,qty_milli FROM stock_transfer_lines WHERE transfer_id=? ORDER BY rowid", id)
	if err != nil {
		return err
	}
	type line struct {
		id, variant string
		sent, recv  int64
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.id, &l.variant, &l.sent); err != nil {
			rows.Close()
			return err
		}
		l.recv = l.sent
		if q, ok := got[l.variant]; ok {
			l.recv = q
		}
		lines = append(lines, l)
	}
	rows.Close()
	var short int64
	for _, l := range lines {
		if l.recv > l.sent {
			v, _ := loadVariant(ctx, tx, l.variant)
			return invalid("more %s received than was sent", v.label())
		}
		if _, err := tx.ExecContext(ctx, "UPDATE stock_transfer_lines SET received_qty_milli=? WHERE id=?", l.recv, l.id); err != nil {
			return err
		}
		short += l.sent - l.recv
		if l.recv == 0 {
			continue
		}
		if err := addStock(ctx, tx, stockMove{VariantID: l.variant, LocationID: t.ToLocationID, DeltaMilli: l.recv, Reason: "transfer_in",
			RefType: "transfer", RefID: id, UserID: userID, CreatedAt: now}); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE stock_transfers SET status=?,received_by=?,received_at=?,receive_note=? WHERE id=?",
		protocol.TransferReceived, userID, now, trim(in.Note), id); err != nil {
		return err
	}
	audit(ctx, tx, userID, terminalID, "stock.transfer_receive", "stock_transfer", id, map[string]any{"no": t.TransferNo, "short_milli": short})
	return recordChange(ctx, tx, protocol.EntityTransfer, id)
}

// applyTransferIn books in a transfer received at a till, possibly while
// the till was offline. The till must be at the destination.
func applyTransferIn(ctx context.Context, tx *sql.Tx, term *Terminal, op protocol.Op) error {
	var in protocol.TransferReceiveOp
	if err := json.Unmarshal(op.Payload, &in); err != nil {
		return invalid("bad transfer payload: %v", err)
	}
	t, err := scanTransfer(tx.QueryRowContext(ctx, transferSelect+" WHERE t.id=?", in.TransferID))
	if errors.Is(err, sql.ErrNoRows) {
		return invalid("transfer not found")
	}
	if err != nil {
		return err
	}
	if t.ToLocationID != term.LocationID {
		return invalid("%s is going to %s, not this till's location", t.TransferNo, t.ToLocationName)
	}
	if err := checkTillUser(ctx, tx, in.UserID); err != nil {
		return err
	}
	at := in.ReceivedAt
	if at <= 0 || at > ids.NowMs()+24*3600*1000 {
		at = ids.NowMs()
	}
	return receiveTransfer(ctx, tx, &t, in.UserID, term.ID, in.TransferReceipt, at)
}

// CancelTransfer returns an in-transit transfer to where it came from.
func (h *Hub) CancelTransfer(ctx context.Context, by *Session, id string) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		t, err := scanTransfer(tx.QueryRowContext(ctx, transferSelect+" WHERE t.id=?", id))
		if errors.Is(err, sql.ErrNoRows) {
			return invalid("transfer not found")
		}
		if err != nil {
			return err
		}
		if t.Status != protocol.TransferInTransit {
			return invalid("only a transfer still in transit can be cancelled")
		}
		rows, err := tx.QueryContext(ctx, "SELECT variant_id,qty_milli FROM stock_transfer_lines WHERE transfer_id=?", id)
		if err != nil {
			return err
		}
		var moves []stockMove
		now := ids.NowMs()
		for rows.Next() {
			m := stockMove{LocationID: t.FromLocationID, Reason: "transfer_in", RefType: "transfer", RefID: id, UserID: by.UserID,
				Note: "transfer cancelled", CreatedAt: now}
			if err := rows.Scan(&m.VariantID, &m.DeltaMilli); err != nil {
				rows.Close()
				return err
			}
			moves = append(moves, m)
		}
		rows.Close()
		for _, m := range moves {
			if err := addStock(ctx, tx, m); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE stock_transfers SET status=?,received_by=?,received_at=? WHERE id=?",
			protocol.TransferCancelled, by.UserID, now, id); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "stock.transfer_cancel", "stock_transfer", id, map[string]any{"no": t.TransferNo})
		return recordChange(ctx, tx, protocol.EntityTransfer, id)
	})
}

const transferSelect = `SELECT t.id,t.transfer_no,t.from_location_id,COALESCE(lf.name,''),t.to_location_id,COALESCE(lt.name,''),t.note,
	COALESCE(u.name,t.created_by),t.created_at,(SELECT COALESCE(sum(qty_milli),0) FROM stock_transfer_lines WHERE transfer_id=t.id),
	t.status,COALESCE(ru.name,t.received_by),t.received_at,t.receive_note,
	(SELECT COALESCE(sum(l.qty_milli*v.cost_minor/1000),0) FROM stock_transfer_lines l JOIN variants v ON v.id=l.variant_id WHERE l.transfer_id=t.id)
	FROM stock_transfers t LEFT JOIN locations lf ON lf.id=t.from_location_id LEFT JOIN locations lt ON lt.id=t.to_location_id
	LEFT JOIN users u ON u.id=t.created_by LEFT JOIN users ru ON ru.id=t.received_by`

func scanTransfer(r interface{ Scan(...any) error }) (protocol.StockTransfer, error) {
	var t protocol.StockTransfer
	err := r.Scan(&t.ID, &t.TransferNo, &t.FromLocationID, &t.FromLocationName, &t.ToLocationID, &t.ToLocationName, &t.Note, &t.CreatedBy,
		&t.CreatedAt, &t.ItemsMilli, &t.Status, &t.ReceivedBy, &t.ReceivedAt, &t.ReceiveNote, &t.CostMinor)
	return t, err
}

// ListTransfers lists stock transfers, newest first; status filters them.
func (h *Hub) ListTransfers(ctx context.Context, by *Session, status string) ([]protocol.StockTransfer, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	q, args := transferSelect, []any{}
	if status != "" {
		q += " WHERE t.status=?"
		args = append(args, status)
	}
	rows, err := h.db.R().QueryContext(ctx, q+" ORDER BY t.created_at DESC LIMIT 500", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.StockTransfer{}
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTransfer loads a transfer with its lines.
func (h *Hub) GetTransfer(ctx context.Context, by *Session, id string) (*protocol.StockTransfer, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	t, err := scanTransfer(h.db.R().QueryRowContext(ctx, transferSelect+" WHERE t.id=? OR t.transfer_no=?", id, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("transfer not found")
	}
	if err != nil {
		return nil, err
	}
	t.Lines, err = transferLines(ctx, h.db.R(), t.ID)
	return &t, err
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func transferLines(ctx context.Context, q queryer, id string) ([]protocol.StockTransferLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT l.variant_id,COALESCE(p.name,''),COALESCE(v.name,''),COALESCE(v.sku,''),COALESCE(p.unit,''),
		l.qty_milli,l.received_qty_milli,COALESCE(v.cost_minor,0)
		FROM stock_transfer_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE l.transfer_id=? ORDER BY l.rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []protocol.StockTransferLine
	for rows.Next() {
		var l protocol.StockTransferLine
		if err := rows.Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.Unit, &l.QtyMilli, &l.ReceivedQtyMilli, &l.CostMinor); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---------- stock counts ----------

// StockCountInput opens a count.
type StockCountInput struct {
	Name       string `json:"name"`
	LocationID string `json:"location_id"`
	CategoryID string `json:"category_id"`
}

// CreateStockCount opens a physical count for a location, optionally limited to one category.
func (h *Hub) CreateStockCount(ctx context.Context, by *Session, in StockCountInput) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	id := ids.New()
	return id, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		loc := in.LocationID
		if loc == "" {
			loc = defaultLocation(ctx, tx)
		}
		if !locationExists(ctx, tx, loc) {
			return invalid("unknown location")
		}
		if in.CategoryID != "" {
			var n int
			tx.QueryRowContext(ctx, "SELECT count(*) FROM categories WHERE id=?", in.CategoryID).Scan(&n)
			if n == 0 {
				return invalid("unknown category")
			}
		}
		var open int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM stock_counts WHERE location_id=? AND category_id=? AND status='open'", loc, in.CategoryID).Scan(&open)
		if open > 0 {
			return invalid("a count for this location and category is already open")
		}
		no, err := nextDocNo(ctx, tx, "count", "SC")
		if err != nil {
			return err
		}
		name := trim(in.Name)
		if name == "" {
			name = "Stock count " + ids.Now().Format("2 Jan 2006")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO stock_counts(id,count_no,name,location_id,category_id,status,created_by,created_at) VALUES(?,?,?,?,?,'open',?,?)`,
			id, no, name, loc, in.CategoryID, by.UserID, ids.NowMs())
		return err
	})
}

const countSelect = `SELECT c.id,c.count_no,c.name,c.location_id,COALESCE(l.name,''),c.category_id,COALESCE(cat.name,''),c.status,
	COALESCE(u.name,c.created_by),c.created_at,COALESCE(pu.name,c.posted_by),c.posted_at,c.note,
	(SELECT count(*) FROM stock_count_lines WHERE count_id=c.id)
	FROM stock_counts c LEFT JOIN locations l ON l.id=c.location_id LEFT JOIN categories cat ON cat.id=c.category_id
	LEFT JOIN users u ON u.id=c.created_by LEFT JOIN users pu ON pu.id=c.posted_by`

func scanCount(r interface{ Scan(...any) error }) (protocol.StockCount, error) {
	var c protocol.StockCount
	err := r.Scan(&c.ID, &c.CountNo, &c.Name, &c.LocationID, &c.LocationName, &c.CategoryID, &c.CategoryName, &c.Status,
		&c.CreatedBy, &c.CreatedAt, &c.PostedBy, &c.PostedAt, &c.Note, &c.LineCount)
	return c, err
}

// ListStockCounts lists counts, open ones first.
func (h *Hub) ListStockCounts(ctx context.Context, by *Session) ([]protocol.StockCount, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	rows, err := h.db.R().QueryContext(ctx, countSelect+" ORDER BY c.status='open' DESC, c.created_at DESC LIMIT 300")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.StockCount{}
	for rows.Next() {
		c, err := scanCount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ledgerAsOf is the stock of an item at a location at a moment, from the ledger.
func ledgerAsOf(ctx context.Context, q rowQuerier, variantID, locationID string, at int64) int64 {
	var n int64
	q.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_milli),0) FROM stock_movements WHERE variant_id=? AND location_id=? AND created_at<=?", variantID, locationID, at).Scan(&n)
	return n
}

// GetStockCount loads a count. For open counts each line's expected quantity
// is the ledger as of when it was counted, and the count's warnings say
// which tills may still hold sales that would change those figures.
func (h *Hub) GetStockCount(ctx context.Context, by *Session, id string) (*protocol.StockCount, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	r := h.db.R()
	c, err := scanCount(r.QueryRowContext(ctx, countSelect+" WHERE c.id=? OR c.count_no=?", id, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("stock count not found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.QueryContext(ctx, `SELECT l.variant_id,COALESCE(p.name,''),COALESCE(v.name,''),COALESCE(v.sku,''),COALESCE(p.unit,'pc'),
		l.counted_milli,l.counted_at,COALESCE(u.name,l.counted_by),l.expected_milli,CASE WHEN l.cost_minor>0 THEN l.cost_minor ELSE COALESCE(v.cost_minor,0) END,l.uncounted
		FROM stock_count_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id
		LEFT JOIN users u ON u.id=l.counted_by WHERE l.count_id=? ORDER BY l.counted_at DESC`, c.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.StockCountLine
		if err := rows.Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.Unit, &l.CountedMilli, &l.CountedAt, &l.CountedBy,
			&l.ExpectedMilli, &l.CostMinor, &l.Uncounted); err != nil {
			rows.Close()
			return nil, err
		}
		c.Lines = append(c.Lines, l)
	}
	rows.Close()
	if c.Status != protocol.CountOpen {
		return &c, nil
	}
	var latest int64
	for i := range c.Lines {
		l := &c.Lines[i]
		l.ExpectedMilli = ledgerAsOf(ctx, r, l.VariantID, c.LocationID, l.CountedAt)
		latest = max(latest, l.CountedAt)
	}
	if latest > 0 {
		// Connected tills check in at least every 30s (long-poll), so allow 90s (check-ins are recorded at most every 30s).
		trows, err := r.QueryContext(ctx, "SELECT name, last_seen_at FROM terminals WHERE location_id=? AND active=1 AND last_seen_at < ?", c.LocationID, latest-90_000)
		if err != nil {
			return nil, err
		}
		defer trows.Close()
		for trows.Next() {
			var name string
			var seen int64
			trows.Scan(&name, &seen)
			when := "never"
			if seen > 0 {
				when = time.UnixMilli(seen).Format("2 Jan 15:04")
			}
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s last synced %s; sales it made since may not be included yet.", name, when))
		}
	}
	return &c, nil
}

// CountEntry records a counted quantity (scanned: add; typed: set).
func (h *Hub) CountEntry(ctx context.Context, by *Session, countID string, e protocol.StockCountEntry) (*protocol.StockCountLine, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	var out protocol.StockCountLine
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var status, loc, cat string
		if err := tx.QueryRowContext(ctx, "SELECT status, location_id, category_id FROM stock_counts WHERE id=?", countID).Scan(&status, &loc, &cat); err != nil {
			return invalid("stock count not found")
		}
		if status != protocol.CountOpen {
			return invalid("this count is %s", status)
		}
		v, err := loadVariant(ctx, tx, e.VariantID)
		if err != nil {
			return err
		}
		if !v.TrackStock {
			return invalid("%s does not track stock", v.label())
		}
		if cat != "" && v.CategoryID != cat {
			return invalid("%s is not in this count's category", v.label())
		}
		now := ids.NowMs()
		switch e.Mode {
		case "remove":
			_, err := tx.ExecContext(ctx, "DELETE FROM stock_count_lines WHERE count_id=? AND variant_id=?", countID, v.ID)
			out = protocol.StockCountLine{VariantID: v.ID}
			return err
		case "add":
			if e.QtyMilli == 0 {
				return invalid("enter a quantity")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO stock_count_lines(count_id,variant_id,counted_milli,counted_at,counted_by) VALUES(?,?,?,?,?)
				ON CONFLICT(count_id,variant_id) DO UPDATE SET counted_milli=counted_milli+excluded.counted_milli, counted_at=excluded.counted_at, counted_by=excluded.counted_by`,
				countID, v.ID, e.QtyMilli, now, by.UserID); err != nil {
				return err
			}
		case "set", "":
			if _, err := tx.ExecContext(ctx, `INSERT INTO stock_count_lines(count_id,variant_id,counted_milli,counted_at,counted_by) VALUES(?,?,?,?,?)
				ON CONFLICT(count_id,variant_id) DO UPDATE SET counted_milli=excluded.counted_milli, counted_at=excluded.counted_at, counted_by=excluded.counted_by`,
				countID, v.ID, e.QtyMilli, now, by.UserID); err != nil {
				return err
			}
		default:
			return invalid("unknown count mode %q", e.Mode)
		}
		var counted int64
		tx.QueryRowContext(ctx, "SELECT counted_milli FROM stock_count_lines WHERE count_id=? AND variant_id=?", countID, v.ID).Scan(&counted)
		if counted < 0 {
			return invalid("the counted quantity cannot be negative")
		}
		if !v.AllowFraction && counted%1000 != 0 {
			return invalid("%s is counted in whole units", v.label())
		}
		out = protocol.StockCountLine{VariantID: v.ID, ProductName: v.ProductName, VariantName: v.VariantName, SKU: v.SKU, Unit: v.Unit,
			CountedMilli: counted, CountedAt: now, CountedBy: by.Name, CostMinor: v.CostMinor,
			ExpectedMilli: ledgerAsOf(ctx, tx, v.ID, loc, now)}
		return nil
	})
	return &out, err
}

// PostStockCount applies a count: each counted item is corrected by
// (counted - ledger as of when it was counted), dated at that moment, so
// sales before and after the count are all accounted for. With
// ZeroUncounted, items in scope that were not counted are set to zero.
func (h *Hub) PostStockCount(ctx context.Context, by *Session, countID string, in protocol.StockCountPost) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var status, loc, cat, no string
		if err := tx.QueryRowContext(ctx, "SELECT status, location_id, category_id, count_no FROM stock_counts WHERE id=?", countID).Scan(&status, &loc, &cat, &no); err != nil {
			return invalid("stock count not found")
		}
		if status != protocol.CountOpen {
			return invalid("this count is already %s", status)
		}
		now := ids.NowMs()
		if in.ZeroUncounted {
			q := `SELECT s.variant_id FROM stock_levels s JOIN variants v ON v.id=s.variant_id JOIN products p ON p.id=v.product_id
				WHERE s.location_id=? AND s.qty_milli<>0 AND p.track_stock=1
				AND s.variant_id NOT IN (SELECT variant_id FROM stock_count_lines WHERE count_id=?)`
			args := []any{loc, countID}
			if cat != "" {
				q += " AND p.category_id=?"
				args = append(args, cat)
			}
			rows, err := tx.QueryContext(ctx, q, args...)
			if err != nil {
				return err
			}
			var missing []string
			for rows.Next() {
				var id string
				rows.Scan(&id)
				missing = append(missing, id)
			}
			rows.Close()
			for _, id := range missing {
				if _, err := tx.ExecContext(ctx, `INSERT INTO stock_count_lines(count_id,variant_id,counted_milli,counted_at,counted_by,uncounted) VALUES(?,?,0,?,?,1)`,
					countID, id, now, by.UserID); err != nil {
					return err
				}
			}
		}
		rows, err := tx.QueryContext(ctx, "SELECT variant_id, counted_milli, counted_at FROM stock_count_lines WHERE count_id=?", countID)
		if err != nil {
			return err
		}
		type line struct {
			variant      string
			counted, at  int64
			expected, ct int64
		}
		var lines []line
		for rows.Next() {
			var l line
			rows.Scan(&l.variant, &l.counted, &l.at)
			lines = append(lines, l)
		}
		rows.Close()
		var variance, value int64
		for _, l := range lines {
			l.expected = ledgerAsOf(ctx, tx, l.variant, loc, l.at)
			tx.QueryRowContext(ctx, "SELECT cost_minor FROM variants WHERE id=?", l.variant).Scan(&l.ct)
			if _, err := tx.ExecContext(ctx, "UPDATE stock_count_lines SET expected_milli=?, cost_minor=? WHERE count_id=? AND variant_id=?",
				l.expected, l.ct, countID, l.variant); err != nil {
				return err
			}
			delta := l.counted - l.expected
			if delta == 0 {
				continue
			}
			variance += delta
			value += mulDiv(delta, l.ct, 1000)
			if err := addStock(ctx, tx, stockMove{VariantID: l.variant, LocationID: loc, DeltaMilli: delta, Reason: "count",
				RefType: "count", RefID: countID, UserID: by.UserID, CreatedAt: l.at}); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE stock_counts SET status='posted', posted_by=?, posted_at=?, note=? WHERE id=?", by.UserID, now, trim(in.Note), countID); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "stock.count_posted", "stock_count", countID, map[string]any{"no": no, "lines": len(lines), "variance_milli": variance, "variance_value": value})
		return nil
	})
}

// CancelStockCount abandons an open count without changing stock.
func (h *Hub) CancelStockCount(ctx context.Context, by *Session, countID string) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "UPDATE stock_counts SET status='cancelled', posted_by=?, posted_at=? WHERE id=? AND status='open'", by.UserID, ids.NowMs(), countID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return invalid("only open counts can be cancelled")
		}
		return nil
	})
}

// ---------- ledger ----------

// StockMovements returns the latest ledger entries for an item (at one
// location, or all), newest first, each with the running balance after it.
func (h *Hub) StockMovements(ctx context.Context, by *Session, variantID, locationID string, limit int) ([]protocol.StockMovement, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	r := h.db.R()
	where, args := "m.variant_id=?", []any{variantID}
	var level int64
	if locationID != "" {
		where += " AND m.location_id=?"
		args = append(args, locationID)
		r.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_milli),0) FROM stock_levels WHERE variant_id=? AND location_id=?", variantID, locationID).Scan(&level)
	} else {
		r.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_milli),0) FROM stock_levels WHERE variant_id=?", variantID).Scan(&level)
	}
	args = append(args, limit)
	rows, err := r.QueryContext(ctx, `SELECT m.id,m.variant_id,m.location_id,COALESCE(l.name,''),m.qty_milli,m.reason,m.ref_type,m.ref_id,
		COALESCE(CASE m.ref_type
			WHEN 'sale' THEN (SELECT receipt_no FROM sales WHERE id=m.ref_id)
			WHEN 'return' THEN (SELECT return_no FROM returns WHERE id=m.ref_id)
			WHEN 'receipt' THEN (SELECT grn_no FROM goods_receipts WHERE id=m.ref_id)
			WHEN 'transfer' THEN (SELECT transfer_no FROM stock_transfers WHERE id=m.ref_id)
			WHEN 'count' THEN (SELECT count_no FROM stock_counts WHERE id=m.ref_id)
		END,''),
		COALESCE(u.name,''),COALESCE(t.name,''),m.note,m.created_at
		FROM stock_movements m LEFT JOIN locations l ON l.id=m.location_id LEFT JOIN users u ON u.id=m.user_id
		LEFT JOIN terminals t ON t.id=m.terminal_id
		WHERE `+where+` ORDER BY m.created_at DESC, m.received_at DESC, m.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.StockMovement{}
	bal := level
	for rows.Next() {
		var m protocol.StockMovement
		if err := rows.Scan(&m.ID, &m.VariantID, &m.LocationID, &m.LocationName, &m.QtyMilli, &m.Reason, &m.RefType, &m.RefID, &m.RefNo,
			&m.UserName, &m.TerminalName, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.BalanceMilli = bal
		bal -= m.QtyMilli
		out = append(out, m)
	}
	return out, rows.Err()
}

// checkTillUser: stock work queued at a till must be by a current manager
// or stock keeper.
func checkTillUser(ctx context.Context, tx *sql.Tx, userID string) error {
	var role string
	var active bool
	if err := tx.QueryRowContext(ctx, "SELECT role,active FROM users WHERE id=?", userID).Scan(&role, &active); err != nil || !active {
		return invalid("the person who did this is not a current user")
	}
	if !slices.Contains(catalogRoles, role) {
		return invalid("this needs a manager or stock keeper")
	}
	return nil
}

// applyTransferOut records a transfer sent from a till, possibly offline.
// It always leaves from the till's own location.
func applyTransferOut(ctx context.Context, tx *sql.Tx, term *Terminal, op protocol.Op) error {
	var in protocol.TransferSendOp
	if err := json.Unmarshal(op.Payload, &in); err != nil || in.ID == "" {
		return invalid("bad transfer payload")
	}
	if err := checkTillUser(ctx, tx, in.UserID); err != nil {
		return err
	}
	var exists int
	tx.QueryRowContext(ctx, "SELECT count(*) FROM stock_transfers WHERE id=?", in.ID).Scan(&exists)
	if exists > 0 {
		return nil
	}
	at := in.SentAt
	if at <= 0 || at > ids.NowMs()+24*3600*1000 {
		at = ids.NowMs()
	}
	return createTransfer(ctx, tx, in.ID, in.UserID, term.ID, TransferInput{FromLocationID: term.LocationID, ToLocationID: in.ToLocationID,
		Note: in.Note, Lines: in.Lines}, at)
}
