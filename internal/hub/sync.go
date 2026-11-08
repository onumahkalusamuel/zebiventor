package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

const pullPageSize = 1000

// Pull returns entities changed after since. If nothing has changed and wait
// is positive, it blocks until a change commits, wait elapses or ctx ends.
func (h *Hub) Pull(ctx context.Context, since int64, wait time.Duration) (*protocol.PullResponse, error) {
	deadline := time.Now().Add(wait)
	for {
		// Subscribe before querying so a commit between the two isn't missed.
		changed := h.notify.wait()
		resp, err := h.pullOnce(ctx, since)
		if err != nil || resp.Rev > since || wait <= 0 {
			return resp, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return resp, nil
		}
		timer := time.NewTimer(remaining)
		select {
		case <-changed:
			timer.Stop()
		case <-timer.C:
			return resp, nil
		case <-ctx.Done():
			timer.Stop()
			return resp, nil
		}
	}
}

func (h *Hub) pullOnce(ctx context.Context, since int64) (*protocol.PullResponse, error) {
	// One read transaction = one consistent snapshot across all tables.
	tx, err := h.db.R().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, "SELECT rev, entity, entity_id FROM changelog WHERE rev > ? ORDER BY rev LIMIT ?", since, pullPageSize+1)
	if err != nil {
		return nil, err
	}
	byEntity := map[string][]string{}
	resp := &protocol.PullResponse{Rev: since}
	n := 0
	for rows.Next() {
		var rev int64
		var entity, id string
		if err := rows.Scan(&rev, &entity, &id); err != nil {
			rows.Close()
			return nil, err
		}
		n++
		if n > pullPageSize {
			resp.More = true
			break
		}
		resp.Rev = rev
		byEntity[entity] = append(byEntity[entity], id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for entity, idList := range byEntity {
		if err := fetchEntities(ctx, tx, entity, idList, resp); err != nil {
			return nil, fmt.Errorf("pull %s: %w", entity, err)
		}
	}
	return resp, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func anys(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func fetchEntities(ctx context.Context, tx *sql.Tx, entity string, idList []string, resp *protocol.PullResponse) error {
	for start := 0; start < len(idList); start += 400 {
		chunk := idList[start:min(start+400, len(idList))]
		if err := fetchChunk(ctx, tx, entity, chunk, resp); err != nil {
			return err
		}
	}
	return nil
}

func fetchChunk(ctx context.Context, tx *sql.Tx, entity string, idList []string, resp *protocol.PullResponse) error {
	in := placeholders(len(idList))
	args := anys(idList)
	q := func(query string, scan func(*sql.Rows) error) error {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	switch entity {
	case protocol.EntitySetting:
		return q("SELECT key,value FROM settings WHERE key IN ("+in+")", func(r *sql.Rows) error {
			var s protocol.Setting
			err := r.Scan(&s.Key, &s.Value)
			resp.Settings = append(resp.Settings, s)
			return err
		})
	case protocol.EntityLocation:
		return q("SELECT id,name,code,active FROM locations WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var l protocol.Location
			err := r.Scan(&l.ID, &l.Name, &l.Code, &l.Active)
			resp.Locations = append(resp.Locations, l)
			return err
		})
	case protocol.EntityUser:
		return q("SELECT id,name,username,role,password_hash,pin_hash,active FROM users WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var u protocol.SyncUser
			err := r.Scan(&u.ID, &u.Name, &u.Username, &u.Role, &u.PasswordHash, &u.PinHash, &u.Active)
			resp.Users = append(resp.Users, u)
			return err
		})
	case protocol.EntityCategory:
		return q("SELECT id,parent_id,name,active FROM categories WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var c protocol.Category
			err := r.Scan(&c.ID, &c.ParentID, &c.Name, &c.Active)
			resp.Categories = append(resp.Categories, c)
			return err
		})
	case protocol.EntityProduct:
		return q(`SELECT id,name,category_id,description,attributes,unit,allow_fraction,track_stock,tax_rate_bp,active,updated_at
			FROM products WHERE id IN (`+in+")", func(r *sql.Rows) error {
			var p protocol.Product
			var attrs string
			if err := r.Scan(&p.ID, &p.Name, &p.CategoryID, &p.Description, &attrs, &p.Unit, &p.AllowFraction, &p.TrackStock, &p.TaxRateBP, &p.Active, &p.UpdatedAt); err != nil {
				return err
			}
			_ = json.Unmarshal([]byte(attrs), &p.Attributes)
			resp.Products = append(resp.Products, p)
			return nil
		})
	case protocol.EntityVariant:
		return q("SELECT id,product_id,name,sku,price_minor,cost_minor,active,reorder_level_milli FROM variants WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var v protocol.Variant
			err := r.Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.PriceMinor, &v.CostMinor, &v.Active, &v.ReorderLevelMilli)
			resp.Variants = append(resp.Variants, v)
			return err
		})
	case protocol.EntityBarcode:
		return q("SELECT code,variant_id,pack_qty_milli,deleted FROM barcodes WHERE code IN ("+in+")", func(r *sql.Rows) error {
			var b protocol.Barcode
			err := r.Scan(&b.Code, &b.VariantID, &b.PackQtyMilli, &b.Deleted)
			resp.Barcodes = append(resp.Barcodes, b)
			return err
		})
	case protocol.EntityPaymentMethod:
		return q("SELECT id,name,kind,requires_reference,opens_drawer,sort_order,active FROM payment_methods WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var m protocol.PaymentMethod
			err := r.Scan(&m.ID, &m.Name, &m.Kind, &m.RequiresReference, &m.OpensDrawer, &m.SortOrder, &m.Active)
			resp.PaymentMethods = append(resp.PaymentMethods, m)
			return err
		})
	case protocol.EntityCustomer:
		return q("SELECT id,name,phone,email,notes,active,balance_minor,updated_at FROM customers WHERE id IN ("+in+")", func(r *sql.Rows) error {
			var c protocol.Customer
			err := r.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.Active, &c.BalanceMinor, &c.UpdatedAt)
			resp.Customers = append(resp.Customers, c)
			return err
		})
	case protocol.EntityStockLevel:
		// entity_id is "variant|location"; query each explicitly.
		for _, key := range idList {
			v, l, _ := strings.Cut(key, "|")
			s := protocol.StockLevel{VariantID: v, LocationID: l}
			err := tx.QueryRowContext(ctx, "SELECT qty_milli FROM stock_levels WHERE variant_id=? AND location_id=?", v, l).Scan(&s.QtyMilli)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			resp.StockLevels = append(resp.StockLevels, s)
		}
		return nil
	}
	return nil // unknown entity: newer hub feature, ignore
}

// Push applies ops from a terminal. Each op is applied at most once
// (keyed by op id) in its own queued transaction. Validation failures are
// reported per op; infrastructure failures abort with an error so the
// terminal retries later.
func (h *Hub) Push(ctx context.Context, t *Terminal, req protocol.PushRequest) (*protocol.PushResponse, error) {
	resp := &protocol.PushResponse{Results: make([]protocol.OpResult, 0, len(req.Ops))}
	for _, op := range req.Ops {
		res, err := h.applyOp(ctx, t, op)
		if err != nil {
			return resp, err
		}
		resp.Results = append(resp.Results, res)
	}
	return resp, nil
}

func (h *Hub) applyOp(ctx context.Context, t *Terminal, op protocol.Op) (protocol.OpResult, error) {
	res := protocol.OpResult{ID: op.ID}
	if op.ID == "" {
		res.Status, res.Error = protocol.OpRejected, "missing op id"
		return res, nil
	}
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var prevStatus, prevErr string
		err := tx.QueryRowContext(ctx, "SELECT status, error FROM inbox WHERE op_id=?", op.ID).Scan(&prevStatus, &prevErr)
		if err == nil {
			res.Status, res.Error = protocol.OpDuplicate, prevErr
			if prevStatus == protocol.OpRejected {
				res.Status = protocol.OpRejected
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		var applyErr error
		switch op.Kind {
		case protocol.OpSaleCreate:
			applyErr = h.applySale(ctx, tx, t, op)
		case protocol.OpReturnCreate:
			applyErr = h.applyReturn(ctx, tx, t, op)
		case protocol.OpShiftOpen:
			applyErr = h.applyShiftOpen(ctx, tx, t, op)
		case protocol.OpShiftClose:
			applyErr = h.applyShiftClose(ctx, tx, t, op)
		case protocol.OpCashMovement:
			applyErr = h.applyCashMovement(ctx, tx, t, op)
		case protocol.OpCustomerUpsert:
			applyErr = h.applyCustomer(ctx, tx, t, op)
		case protocol.OpAccountPayment:
			applyErr = h.applyAccountPayment(ctx, tx, t, op)
		default:
			applyErr = invalid("unsupported op kind %q", op.Kind)
		}
		if applyErr != nil {
			return applyErr
		}
		res.Status = protocol.OpApplied
		_, err = tx.ExecContext(ctx, "INSERT INTO inbox(op_id,terminal_id,kind,status,received_at) VALUES(?,?,?,?,?)",
			op.ID, t.ID, op.Kind, protocol.OpApplied, ids.NowMs())
		return err
	})

	var ve validationError
	if errors.As(err, &ve) {
		// Record the rejection so a retry gets the same answer.
		res.Status, res.Error = protocol.OpRejected, ve.Error()
		werr := h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO inbox(op_id,terminal_id,kind,status,error,received_at) VALUES(?,?,?,?,?,?)",
				op.ID, t.ID, op.Kind, protocol.OpRejected, ve.Error(), ids.NowMs())
			return err
		})
		return res, werr
	}
	return res, err
}

// applySale records a completed sale and its stock movements.
//
// A sale that reaches the hub has already happened: the customer paid and
// left. So the hub never refuses one for business reasons (insufficient
// stock, a price that changed meanwhile, arithmetic drift); it records it as
// sold and flags anomalies in the audit log. Only malformed payloads are rejected.
func (h *Hub) applySale(ctx context.Context, tx *sql.Tx, t *Terminal, op protocol.Op) error {
	var s protocol.Sale
	if err := json.Unmarshal(op.Payload, &s); err != nil {
		return invalid("bad sale payload: %v", err)
	}
	if s.ID == "" || s.ReceiptNo == "" || len(s.Lines) == 0 {
		return invalid("sale is missing id, receipt number or lines")
	}
	if !strings.HasPrefix(s.ReceiptNo, t.ReceiptPrefix+"-") {
		return invalid("receipt number %q does not belong to terminal %s", s.ReceiptNo, t.ReceiptPrefix)
	}
	now := ids.NowMs()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	loc := s.LocationID
	var ok int
	if loc != "" {
		tx.QueryRowContext(ctx, "SELECT count(*) FROM locations WHERE id=?", loc).Scan(&ok)
	}
	if ok == 0 {
		loc = t.LocationID
	}
	if s.Status == "" {
		s.Status = "completed"
	}

	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sales WHERE id=?", s.ID).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil // same sale re-sent under a different op id; nothing to do
	}

	// Recompute and flag drift rather than reject.
	var sub int64
	for _, l := range s.Lines {
		sub += l.TotalMinor
	}
	var paid int64
	for _, p := range s.Payments {
		paid += p.AmountMinor
	}
	wantTotal := s.SubtotalMinor - s.DiscountMinor
	if s.TaxMode == protocol.TaxExclusive {
		wantTotal += s.TaxMinor
	}
	if sub != s.SubtotalMinor || paid != s.PaidMinor || s.TotalMinor != wantTotal {
		audit(ctx, tx, s.UserID, t.ID, "sale.totals_mismatch", "sale", s.ID,
			map[string]any{"lines_sum": sub, "subtotal": s.SubtotalMinor, "payments_sum": paid, "paid": s.PaidMinor, "total": s.TotalMinor})
	}

	if s.TaxMode == "" {
		s.TaxMode = protocol.TaxNone
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sales(id,receipt_no,terminal_id,location_id,user_id,user_name,customer_id,status,
		subtotal_minor,discount_minor,tax_minor,total_minor,paid_minor,change_minor,note,created_at,received_at,shift_id,approved_by,tax_mode)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.ReceiptNo, t.ID, loc, s.UserID, s.UserName, s.CustomerID, s.Status,
		s.SubtotalMinor, s.DiscountMinor, s.TaxMinor, s.TotalMinor, s.PaidMinor, s.ChangeMinor, s.Note, s.CreatedAt, now,
		s.ShiftID, s.ApprovedBy, s.TaxMode); err != nil {
		return err
	}

	for i, l := range s.Lines {
		if l.ID == "" {
			l.ID = ids.New()
		}
		if l.LineNo == 0 {
			l.LineNo = i + 1
		}
		// Cost of goods sold is the item's (weighted average) cost when the hub records the sale.
		var cost int64
		tx.QueryRowContext(ctx, "SELECT cost_minor FROM variants WHERE id=?", l.VariantID).Scan(&cost)
		if _, err := tx.ExecContext(ctx, `INSERT INTO sale_lines(id,sale_id,line_no,variant_id,product_name,variant_name,sku,qty_milli,unit_price_minor,discount_minor,tax_minor,total_minor,original_price_minor,tax_rate_bp,cost_minor)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, l.ID, s.ID, l.LineNo, l.VariantID, l.ProductName, l.VariantName, l.SKU,
			l.QtyMilli, l.UnitPriceMinor, l.DiscountMinor, l.TaxMinor, l.TotalMinor, l.OriginalPriceMinor, l.TaxRateBP, cost); err != nil {
			return err
		}

		// Stock: the hub's catalogue decides whether the item is tracked.
		var track bool
		err := tx.QueryRowContext(ctx, `SELECT p.track_stock FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, l.VariantID).Scan(&track)
		if errors.Is(err, sql.ErrNoRows) {
			audit(ctx, tx, s.UserID, t.ID, "sale.unknown_variant", "sale", s.ID, map[string]any{"variant_id": l.VariantID})
			continue
		}
		if err != nil {
			return err
		}
		if !track {
			continue
		}
		if err := addStock(ctx, tx, stockMove{
			VariantID: l.VariantID, LocationID: loc, DeltaMilli: -l.QtyMilli, Reason: "sale",
			RefType: "sale", RefID: s.ID, UserID: s.UserID, TerminalID: t.ID, CreatedAt: s.CreatedAt,
		}); err != nil {
			return err
		}
	}

	var onAccount int64
	for _, p := range s.Payments {
		if p.ID == "" {
			p.ID = ids.New()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sale_payments(id,sale_id,method_id,method_name,kind,amount_minor,reference) VALUES(?,?,?,?,?,?,?)`,
			p.ID, s.ID, p.MethodID, p.MethodName, p.Kind, p.AmountMinor, p.Reference); err != nil {
			return err
		}
		if p.Kind == protocol.PayCredit {
			onAccount += p.AmountMinor
		}
	}
	return postLedger(ctx, tx, ledgerEntry{CustomerID: s.CustomerID, Kind: "sale", RefID: s.ID, RefNo: s.ReceiptNo, Amount: onAccount,
		At: s.CreatedAt, UserID: s.UserID, UserName: s.UserName, TerminalID: t.ID})
}

type stockMove struct {
	VariantID, LocationID string
	DeltaMilli            int64
	Reason                string
	RefType, RefID        string
	UserID, TerminalID    string
	Note                  string
	CreatedAt             int64
}

// addStock appends to the stock ledger and updates the running level in
// the same transaction, then marks the level as changed for sync.
func addStock(ctx context.Context, tx *sql.Tx, m stockMove) error {
	now := ids.NowMs()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stock_movements(id,variant_id,location_id,qty_milli,reason,ref_type,ref_id,user_id,terminal_id,note,created_at,received_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, ids.New(), m.VariantID, m.LocationID, m.DeltaMilli, m.Reason, m.RefType, m.RefID,
		m.UserID, m.TerminalID, m.Note, m.CreatedAt, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stock_levels(variant_id,location_id,qty_milli,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(variant_id,location_id) DO UPDATE SET qty_milli=qty_milli+excluded.qty_milli, updated_at=excluded.updated_at`,
		m.VariantID, m.LocationID, m.DeltaMilli, now); err != nil {
		return err
	}
	return recordChange(ctx, tx, protocol.EntityStockLevel, m.VariantID+"|"+m.LocationID)
}
