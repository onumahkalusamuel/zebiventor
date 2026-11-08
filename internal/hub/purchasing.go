package hub

import (
	"context"
	"database/sql"
	"errors"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// ---------- suppliers ----------

// ListSuppliers returns all suppliers with open orders and value received.
func (h *Hub) ListSuppliers(ctx context.Context, by *Session) ([]protocol.Supplier, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	rows, err := h.db.R().QueryContext(ctx, `SELECT s.id,s.name,s.contact_name,s.phone,s.email,s.address,s.notes,s.active,
		(SELECT count(*) FROM purchase_orders o WHERE o.supplier_id=s.id AND o.status IN ('ordered','partial')),
		(SELECT COALESCE(sum(total_minor),0) FROM goods_receipts g WHERE g.supplier_id=s.id)
		FROM suppliers s ORDER BY s.active DESC, s.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.Supplier{}
	for rows.Next() {
		var s protocol.Supplier
		if err := rows.Scan(&s.ID, &s.Name, &s.ContactName, &s.Phone, &s.Email, &s.Address, &s.Notes, &s.Active, &s.OpenOrders, &s.ReceivedMinor); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SaveSupplier creates or updates a supplier.
func (h *Hub) SaveSupplier(ctx context.Context, by *Session, s protocol.Supplier) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	s.Name = trim(s.Name)
	if s.Name == "" {
		return "", invalid("supplier name is required")
	}
	if s.ID == "" {
		s.ID, s.Active = ids.New(), true
	}
	return s.ID, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		var clash string
		tx.QueryRowContext(ctx, "SELECT id FROM suppliers WHERE name=? COLLATE NOCASE AND id<>?", s.Name, s.ID).Scan(&clash)
		if clash != "" {
			return invalid("a supplier called %q already exists", s.Name)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO suppliers(id,name,contact_name,phone,email,address,notes,active,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, contact_name=excluded.contact_name, phone=excluded.phone, email=excluded.email,
			address=excluded.address, notes=excluded.notes, active=excluded.active, updated_at=excluded.updated_at`,
			s.ID, s.Name, trim(s.ContactName), trim(s.Phone), trim(s.Email), trim(s.Address), trim(s.Notes), s.Active, now, now)
		return err
	})
}

// ---------- purchase orders ----------

const poSelect = `SELECT o.id,o.po_no,o.supplier_id,COALESCE(s.name,''),o.location_id,COALESCE(l.name,''),o.status,o.expected_at,o.note,
	COALESCE(u.name,o.created_by),o.created_at,o.updated_at,o.ordered_at,
	(SELECT COALESCE(sum(qty_ordered_milli*unit_cost_minor/1000),0) FROM purchase_order_lines WHERE po_id=o.id),
	(SELECT count(*) FROM purchase_order_lines WHERE po_id=o.id)
	FROM purchase_orders o LEFT JOIN suppliers s ON s.id=o.supplier_id LEFT JOIN locations l ON l.id=o.location_id
	LEFT JOIN users u ON u.id=o.created_by`

func scanPO(r interface{ Scan(...any) error }) (protocol.PurchaseOrder, error) {
	var o protocol.PurchaseOrder
	err := r.Scan(&o.ID, &o.PONo, &o.SupplierID, &o.SupplierName, &o.LocationID, &o.LocationName, &o.Status, &o.ExpectedAt, &o.Note,
		&o.CreatedBy, &o.CreatedAt, &o.UpdatedAt, &o.OrderedAt, &o.TotalMinor, &o.LineCount)
	return o, err
}

// ListPurchaseOrders lists orders, newest first, optionally filtered.
func (h *Hub) ListPurchaseOrders(ctx context.Context, by *Session, status, supplierID string) ([]protocol.PurchaseOrder, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	q := poSelect + " WHERE 1=1"
	var args []any
	switch status {
	case "":
	case "open":
		q += " AND o.status IN ('draft','ordered','partial')"
	default:
		q += " AND o.status=?"
		args = append(args, status)
	}
	if supplierID != "" {
		q += " AND o.supplier_id=?"
		args = append(args, supplierID)
	}
	rows, err := h.db.R().QueryContext(ctx, q+" ORDER BY o.created_at DESC LIMIT 500", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.PurchaseOrder{}
	for rows.Next() {
		o, err := scanPO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetPurchaseOrder loads an order with its lines and the deliveries against it.
func (h *Hub) GetPurchaseOrder(ctx context.Context, by *Session, id string) (*protocol.PurchaseOrder, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	r := h.db.R()
	o, err := scanPO(r.QueryRowContext(ctx, poSelect+" WHERE o.id=? OR o.po_no=?", id, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("purchase order not found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.QueryContext(ctx, `SELECT l.id,l.variant_id,COALESCE(p.name,''),COALESCE(v.name,''),COALESCE(v.sku,''),COALESCE(p.unit,'pc'),
		l.qty_ordered_milli,l.qty_received_milli,l.unit_cost_minor
		FROM purchase_order_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id
		WHERE l.po_id=? ORDER BY l.line_no`, o.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l protocol.PurchaseOrderLine
		if err := rows.Scan(&l.ID, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.Unit, &l.QtyOrderedMilli, &l.QtyReceivedMilli, &l.UnitCostMinor); err != nil {
			rows.Close()
			return nil, err
		}
		o.Lines = append(o.Lines, l)
	}
	rows.Close()
	o.Receipts, err = h.listReceipts(ctx, "g.po_id=?", o.ID)
	return &o, err
}

// SavePurchaseOrder creates a draft order or replaces a draft's contents.
// With Submit it is marked as ordered (sent to the supplier).
func (h *Hub) SavePurchaseOrder(ctx context.Context, by *Session, in protocol.PurchaseOrderInput) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	if len(in.Lines) == 0 {
		return "", invalid("add at least one item to the order")
	}
	id := in.ID
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		var supActive bool
		if err := tx.QueryRowContext(ctx, "SELECT active FROM suppliers WHERE id=?", in.SupplierID).Scan(&supActive); err != nil {
			return invalid("choose a supplier")
		}
		loc := in.LocationID
		if loc == "" {
			loc = defaultLocation(ctx, tx)
		}
		if !locationExists(ctx, tx, loc) {
			return invalid("unknown location")
		}
		status := protocol.PODraft
		if in.Submit {
			status = protocol.POOrdered
		}
		var orderedAt int64
		if in.Submit {
			orderedAt = now
		}
		if id == "" {
			id = ids.New()
			no, err := nextDocNo(ctx, tx, "po", "PO")
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_orders(id,po_no,supplier_id,location_id,status,expected_at,note,created_by,created_at,updated_at,ordered_at)
				VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, no, in.SupplierID, loc, status, in.ExpectedAt, trim(in.Note), by.UserID, now, now, orderedAt); err != nil {
				return err
			}
		} else {
			var cur string
			if err := tx.QueryRowContext(ctx, "SELECT status FROM purchase_orders WHERE id=?", id).Scan(&cur); err != nil {
				return invalid("purchase order not found")
			}
			if cur != protocol.PODraft {
				return invalid("only draft orders can be edited")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE purchase_orders SET supplier_id=?, location_id=?, status=?, expected_at=?, note=?, updated_at=?, ordered_at=? WHERE id=?`,
				in.SupplierID, loc, status, in.ExpectedAt, trim(in.Note), now, orderedAt, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM purchase_order_lines WHERE po_id=?", id); err != nil {
				return err
			}
		}
		seen := map[string]bool{}
		for i, l := range in.Lines {
			v, err := loadVariant(ctx, tx, l.VariantID)
			if err != nil {
				return err
			}
			if seen[v.ID] {
				return invalid("%s is on the order twice", v.label())
			}
			seen[v.ID] = true
			if err := v.checkQty(l.QtyOrderedMilli); err != nil {
				return err
			}
			if l.UnitCostMinor < 0 {
				return invalid("cost cannot be negative")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_order_lines(id,po_id,line_no,variant_id,qty_ordered_milli,unit_cost_minor) VALUES(?,?,?,?,?,?)`,
				ids.New(), id, i+1, v.ID, l.QtyOrderedMilli, l.UnitCostMinor); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "po.save", "purchase_order", id, map[string]any{"status": status, "lines": len(in.Lines)})
		return nil
	})
	return id, err
}

// SetPurchaseOrderStatus moves an order along: draft -> ordered, and any open
// order -> cancelled (nothing received yet) or received (close short).
func (h *Hub) SetPurchaseOrderStatus(ctx context.Context, by *Session, id, status string) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var cur string
		if err := tx.QueryRowContext(ctx, "SELECT status FROM purchase_orders WHERE id=?", id).Scan(&cur); err != nil {
			return invalid("purchase order not found")
		}
		var received int64
		tx.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_received_milli),0) FROM purchase_order_lines WHERE po_id=?", id).Scan(&received)
		now := ids.NowMs()
		switch {
		case status == protocol.POOrdered && cur == protocol.PODraft:
			_, err := tx.ExecContext(ctx, "UPDATE purchase_orders SET status=?, ordered_at=?, updated_at=? WHERE id=?", status, now, now, id)
			if err != nil {
				return err
			}
		case status == protocol.POCancelled && (cur == protocol.PODraft || cur == protocol.POOrdered):
			if received > 0 {
				return invalid("goods have been received on this order; close it instead")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE purchase_orders SET status=?, updated_at=? WHERE id=?", status, now, id); err != nil {
				return err
			}
		case status == protocol.POReceived && (cur == protocol.POOrdered || cur == protocol.POPartial):
			if _, err := tx.ExecContext(ctx, "UPDATE purchase_orders SET status=?, updated_at=? WHERE id=?", status, now, id); err != nil {
				return err
			}
		default:
			return invalid("an order that is %s cannot be marked %s", cur, status)
		}
		audit(ctx, tx, by.UserID, "", "po.status", "purchase_order", id, map[string]any{"from": cur, "to": status})
		return nil
	})
}

// ---------- goods received ----------

func (h *Hub) listReceipts(ctx context.Context, where string, args ...any) ([]protocol.GoodsReceipt, error) {
	rows, err := h.db.R().QueryContext(ctx, `SELECT g.id,g.grn_no,g.po_id,COALESCE(o.po_no,''),g.supplier_id,COALESCE(s.name,''),g.location_id,COALESCE(l.name,''),
		g.supplier_ref,g.note,COALESCE(u.name,g.received_by),g.created_at,g.total_minor
		FROM goods_receipts g LEFT JOIN purchase_orders o ON o.id=g.po_id LEFT JOIN suppliers s ON s.id=g.supplier_id
		LEFT JOIN locations l ON l.id=g.location_id LEFT JOIN users u ON u.id=g.received_by
		WHERE `+where+` ORDER BY g.created_at DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.GoodsReceipt{}
	for rows.Next() {
		var g protocol.GoodsReceipt
		if err := rows.Scan(&g.ID, &g.GRNNo, &g.POID, &g.PONo, &g.SupplierID, &g.SupplierName, &g.LocationID, &g.LocationName,
			&g.SupplierRef, &g.Note, &g.ReceivedBy, &g.CreatedAt, &g.TotalMinor); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListGoodsReceipts lists deliveries, optionally for one supplier.
func (h *Hub) ListGoodsReceipts(ctx context.Context, by *Session, supplierID string) ([]protocol.GoodsReceipt, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	if supplierID != "" {
		return h.listReceipts(ctx, "g.supplier_id=?", supplierID)
	}
	return h.listReceipts(ctx, "1=1")
}

// GetGoodsReceipt loads one delivery with its lines.
func (h *Hub) GetGoodsReceipt(ctx context.Context, by *Session, id string) (*protocol.GoodsReceipt, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	list, err := h.listReceipts(ctx, "g.id=? OR g.grn_no=?", id, id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, invalid("delivery not found")
	}
	g := list[0]
	rows, err := h.db.R().QueryContext(ctx, `SELECT l.id,l.po_line_id,l.variant_id,COALESCE(p.name,''),COALESCE(v.name,''),COALESCE(v.sku,''),l.qty_milli,l.unit_cost_minor,l.total_minor
		FROM goods_receipt_lines l LEFT JOIN variants v ON v.id=l.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE l.grn_id=? ORDER BY rowid`, g.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l protocol.GoodsReceiptLine
		if err := rows.Scan(&l.ID, &l.POLineID, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyMilli, &l.UnitCostMinor, &l.TotalMinor); err != nil {
			return nil, err
		}
		g.Lines = append(g.Lines, l)
	}
	return &g, rows.Err()
}

// ReceiveGoods books a delivery into stock. Item costs become the weighted
// average of the stock on hand and the delivery, so profit reports reflect
// what goods actually cost. Receiving against an order updates what is
// still outstanding on it.
func (h *Hub) ReceiveGoods(ctx context.Context, by *Session, in protocol.GoodsReceiptInput) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	var lines []protocol.GoodsReceiptLine
	for _, l := range in.Lines {
		if l.QtyMilli != 0 {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", invalid("enter the quantity received for at least one item")
	}
	id := ids.New()
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		supplier, loc := in.SupplierID, in.LocationID
		poLines := map[string]string{} // po line id -> variant id
		if in.POID != "" {
			var status string
			if err := tx.QueryRowContext(ctx, "SELECT supplier_id, location_id, status FROM purchase_orders WHERE id=?", in.POID).Scan(&supplier, &loc, &status); err != nil {
				return invalid("purchase order not found")
			}
			if status != protocol.POOrdered && status != protocol.POPartial {
				return invalid("this order is %s; only ordered goods can be received", status)
			}
			if in.LocationID != "" {
				loc = in.LocationID
			}
			rows, err := tx.QueryContext(ctx, "SELECT id, variant_id FROM purchase_order_lines WHERE po_id=?", in.POID)
			if err != nil {
				return err
			}
			for rows.Next() {
				var lid, vid string
				rows.Scan(&lid, &vid)
				poLines[lid] = vid
			}
			rows.Close()
		}
		if supplier != "" {
			var n int
			tx.QueryRowContext(ctx, "SELECT count(*) FROM suppliers WHERE id=?", supplier).Scan(&n)
			if n == 0 {
				return invalid("unknown supplier")
			}
		}
		if loc == "" {
			loc = defaultLocation(ctx, tx)
		}
		if !locationExists(ctx, tx, loc) {
			return invalid("unknown location")
		}
		no, err := nextDocNo(ctx, tx, "grn", "GRN")
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO goods_receipts(id,grn_no,po_id,supplier_id,location_id,supplier_ref,note,received_by,created_at)
			VALUES(?,?,?,?,?,?,?,?,?)`, id, no, in.POID, supplier, loc, trim(in.SupplierRef), trim(in.Note), by.UserID, now); err != nil {
			return err
		}
		var total int64
		for _, l := range lines {
			v, err := loadVariant(ctx, tx, l.VariantID)
			if err != nil {
				return err
			}
			if err := v.checkQty(l.QtyMilli); err != nil {
				return err
			}
			if l.UnitCostMinor < 0 || l.NewPriceMinor < 0 {
				return invalid("costs and prices cannot be negative")
			}
			if l.POLineID != "" {
				if poLines[l.POLineID] != v.ID {
					return invalid("%s is not on this order", v.label())
				}
				if _, err := tx.ExecContext(ctx, "UPDATE purchase_order_lines SET qty_received_milli=qty_received_milli+? WHERE id=?", l.QtyMilli, l.POLineID); err != nil {
					return err
				}
			}
			lineTotal := mulDiv(l.QtyMilli, l.UnitCostMinor, 1000)
			total += lineTotal
			if _, err := tx.ExecContext(ctx, `INSERT INTO goods_receipt_lines(id,grn_id,po_line_id,variant_id,qty_milli,unit_cost_minor,total_minor) VALUES(?,?,?,?,?,?,?)`,
				ids.New(), id, l.POLineID, v.ID, l.QtyMilli, l.UnitCostMinor, lineTotal); err != nil {
				return err
			}
			if err := h.receiveCost(ctx, tx, v, l.QtyMilli, l.UnitCostMinor, l.NewPriceMinor, now); err != nil {
				return err
			}
			if v.TrackStock {
				if err := addStock(ctx, tx, stockMove{VariantID: v.ID, LocationID: loc, DeltaMilli: l.QtyMilli, Reason: "receipt",
					RefType: "receipt", RefID: id, UserID: by.UserID, CreatedAt: now}); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE goods_receipts SET total_minor=? WHERE id=?", total, id); err != nil {
			return err
		}
		if in.POID != "" {
			var short int
			tx.QueryRowContext(ctx, "SELECT count(*) FROM purchase_order_lines WHERE po_id=? AND qty_received_milli < qty_ordered_milli", in.POID).Scan(&short)
			status := protocol.POPartial
			if short == 0 || in.ClosePO {
				status = protocol.POReceived
			}
			if _, err := tx.ExecContext(ctx, "UPDATE purchase_orders SET status=?, updated_at=? WHERE id=?", status, now, in.POID); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "goods.received", "goods_receipt", id, map[string]any{"grn": no, "po_id": in.POID, "total": total, "lines": len(lines)})
		return nil
	})
	return id, err
}

// receiveCost updates an item's cost to the weighted average of the stock
// already on hand (across all locations) and the units just received, and
// optionally its selling price.
func (h *Hub) receiveCost(ctx context.Context, tx *sql.Tx, v variantInfo, qty, unitCost, newPrice, now int64) error {
	var onHand int64
	tx.QueryRowContext(ctx, "SELECT COALESCE(sum(qty_milli),0) FROM stock_levels WHERE variant_id=?", v.ID).Scan(&onHand)
	cost := unitCost
	if onHand > 0 && v.TrackStock {
		cost = mulDiv(onHand, v.CostMinor, onHand+qty) + mulDiv(qty, unitCost, onHand+qty)
	}
	price := v.PriceMinor
	if newPrice > 0 {
		price = newPrice
	}
	if cost == v.CostMinor && price == v.PriceMinor {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE variants SET cost_minor=?, price_minor=?, updated_at=? WHERE id=?", cost, price, now, v.ID); err != nil {
		return err
	}
	return recordChange(ctx, tx, protocol.EntityVariant, v.ID)
}
