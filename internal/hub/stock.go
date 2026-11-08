package hub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// TransferInput moves stock between two locations in one step.
type TransferInput struct {
	FromLocationID string                       `json:"from_location_id"`
	ToLocationID   string                       `json:"to_location_id"`
	Note           string                       `json:"note"`
	Lines          []protocol.StockTransferLine `json:"lines"`
}

// TransferStock moves goods from one location to another.
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
		if !locationExists(ctx, tx, in.FromLocationID) || !locationExists(ctx, tx, in.ToLocationID) {
			return invalid("unknown location")
		}
		now := ids.NowMs()
		no, err := nextDocNo(ctx, tx, "transfer", "TR")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stock_transfers(id,transfer_no,from_location_id,to_location_id,note,created_by,created_at) VALUES(?,?,?,?,?,?,?)`,
			id, no, in.FromLocationID, in.ToLocationID, trim(in.Note), by.UserID, now); err != nil {
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
			if _, err := tx.ExecContext(ctx, `INSERT INTO stock_transfer_lines(id,transfer_id,variant_id,qty_milli) VALUES(?,?,?,?)`, ids.New(), id, v.ID, l.QtyMilli); err != nil {
				return err
			}
			base := stockMove{VariantID: v.ID, RefType: "transfer", RefID: id, UserID: by.UserID, CreatedAt: now}
			out, into := base, base
			out.LocationID, out.DeltaMilli, out.Reason = in.FromLocationID, -l.QtyMilli, "transfer_out"
			into.LocationID, into.DeltaMilli, into.Reason = in.ToLocationID, l.QtyMilli, "transfer_in"
			if err := addStock(ctx, tx, out); err != nil {
				return err
			}
			if err := addStock(ctx, tx, into); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "stock.transfer", "stock_transfer", id, map[string]any{"no": no, "lines": len(in.Lines)})
		return nil
	})
}

const transferSelect = `SELECT t.id,t.transfer_no,t.from_location_id,COALESCE(lf.name,''),t.to_location_id,COALESCE(lt.name,''),t.note,
	COALESCE(u.name,t.created_by),t.created_at,(SELECT COALESCE(sum(qty_milli),0) FROM stock_transfer_lines WHERE transfer_id=t.id)
	FROM stock_transfers t LEFT JOIN locations lf ON lf.id=t.from_location_id LEFT JOIN locations lt ON lt.id=t.to_location_id
	LEFT JOIN users u ON u.id=t.created_by`

func scanTransfer(r interface{ Scan(...any) error }) (protocol.StockTransfer, error) {
	var t protocol.StockTransfer
	err := r.Scan(&t.ID, &t.TransferNo, &t.FromLocationID, &t.FromLocationName, &t.ToLocationID, &t.ToLocationName, &t.Note, &t.CreatedBy, &t.CreatedAt, &t.ItemsMilli)
	return t, err
}

// ListTransfers lists stock transfers, newest first.
func (h *Hub) ListTransfers(ctx context.Context, by *Session) ([]protocol.StockTransfer, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	rows, err := h.db.R().QueryContext(ctx, transferSelect+" ORDER BY t.created_at DESC LIMIT 500")
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
	rows, err := h.db.R().QueryContext(ctx, `SELECT l.variant_id,COALESCE(p.name,''),COALESCE(v.name,''),COALESCE(v.sku,''),l.qty_milli
		FROM stock_transfer_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE l.transfer_id=? ORDER BY l.rowid`, t.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l protocol.StockTransferLine
		if err := rows.Scan(&l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli); err != nil {
			return nil, err
		}
		t.Lines = append(t.Lines, l)
	}
	return &t, rows.Err()
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
			name = "Stock count " + time.Now().Format("2 Jan 2006")
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
