package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"golang.org/x/crypto/bcrypt"
)

var (
	catalogRoles = []string{protocol.RoleAdmin, protocol.RoleManager, protocol.RoleInventory}
	managerRoles = []string{protocol.RoleAdmin, protocol.RoleManager}
	adminRoles   = []string{protocol.RoleAdmin}
)

// SaveProduct creates or updates a product with its variants and barcodes.
func (h *Hub) SaveProduct(ctx context.Context, by *Session, in protocol.ProductInput) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	var id string
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		id, err = h.saveProductTx(ctx, tx, by, in)
		return err
	})
	return id, err
}

func (h *Hub) saveProductTx(ctx context.Context, tx *sql.Tx, by *Session, in protocol.ProductInput) (string, error) {
	p := in.Product
	p.Name = strings.TrimSpace(p.Name)
	if p.TaxRateBP < 0 || p.TaxRateBP > 10000 {
		return "", invalid("tax rate must be between 0%% and 100%%")
	}
	if p.Name == "" {
		return "", invalid("product name is required")
	}
	if len(in.Variants) == 0 {
		return "", invalid("a product needs at least one variant (price)")
	}
	if p.Unit == "" {
		p.Unit = "pc"
	}
	if p.Attributes == nil {
		p.Attributes = map[string]string{}
	}
	for k, v := range p.Attributes {
		if strings.TrimSpace(v) == "" {
			delete(p.Attributes, k)
		}
	}
	attrs, _ := json.Marshal(p.Attributes)
	isNew := p.ID == ""
	if isNew {
		p.ID = ids.New()
		p.Active = true
	}

	now := ids.NowMs()
	if p.CategoryID != "" {
		var n int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM categories WHERE id=?", p.CategoryID).Scan(&n)
		if n == 0 {
			return "", invalid("unknown category")
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO products(id,name,category_id,description,attributes,unit,allow_fraction,track_stock,tax_rate_bp,active,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, category_id=excluded.category_id, description=excluded.description,
		attributes=excluded.attributes, unit=excluded.unit, allow_fraction=excluded.allow_fraction, track_stock=excluded.track_stock,
		tax_rate_bp=excluded.tax_rate_bp, active=excluded.active, updated_at=excluded.updated_at`,
		p.ID, p.Name, p.CategoryID, p.Description, string(attrs), p.Unit, p.AllowFraction, p.TrackStock, p.TaxRateBP, p.Active, now, now)
	if err != nil {
		return "", err
	}
	if err := recordChange(ctx, tx, protocol.EntityProduct, p.ID); err != nil {
		return "", err
	}
	for _, vi := range in.Variants {
		if err := h.saveVariant(ctx, tx, by, p, vi, now); err != nil {
			return "", err
		}
	}
	action := "product.update"
	if isNew {
		action = "product.create"
	}
	audit(ctx, tx, by.UserID, "", action, "product", p.ID, map[string]any{"name": p.Name})
	return p.ID, nil
}

// loadProductInput reads a product back in the shape SaveProduct accepts.
func loadProductInput(ctx context.Context, tx *sql.Tx, productID string) (protocol.ProductInput, error) {
	var in protocol.ProductInput
	p := &in.Product
	var attrs string
	err := tx.QueryRowContext(ctx, `SELECT id,name,category_id,description,attributes,unit,allow_fraction,track_stock,tax_rate_bp,active FROM products WHERE id=?`, productID).
		Scan(&p.ID, &p.Name, &p.CategoryID, &p.Description, &attrs, &p.Unit, &p.AllowFraction, &p.TrackStock, &p.TaxRateBP, &p.Active)
	if err != nil {
		return in, err
	}
	_ = json.Unmarshal([]byte(attrs), &p.Attributes)
	rows, err := tx.QueryContext(ctx, `SELECT id,name,sku,price_minor,cost_minor,active,reorder_level_milli FROM variants WHERE product_id=? ORDER BY created_at`, productID)
	if err != nil {
		return in, err
	}
	for rows.Next() {
		var v protocol.VariantInput
		v.ProductID = productID
		if err := rows.Scan(&v.ID, &v.Name, &v.SKU, &v.PriceMinor, &v.CostMinor, &v.Active, &v.ReorderLevelMilli); err != nil {
			rows.Close()
			return in, err
		}
		in.Variants = append(in.Variants, v)
	}
	rows.Close()
	for i := range in.Variants {
		brows, err := tx.QueryContext(ctx, "SELECT code FROM barcodes WHERE variant_id=? AND deleted=0", in.Variants[i].ID)
		if err != nil {
			return in, err
		}
		for brows.Next() {
			var c string
			brows.Scan(&c)
			in.Variants[i].Barcodes = append(in.Variants[i].Barcodes, c)
		}
		brows.Close()
	}
	return in, nil
}

func (h *Hub) saveVariant(ctx context.Context, tx *sql.Tx, by *Session, p protocol.Product, vi protocol.VariantInput, now int64) error {
	v := vi.Variant
	if v.PriceMinor < 0 || v.CostMinor < 0 {
		return invalid("prices cannot be negative")
	}
	if v.ReorderLevelMilli < 0 {
		return invalid("the reorder level cannot be negative")
	}
	isNew := v.ID == ""
	if isNew {
		v.ID = ids.New()
		v.Active = true
	} else {
		var owner string
		err := tx.QueryRowContext(ctx, "SELECT product_id FROM variants WHERE id=?", v.ID).Scan(&owner)
		if errors.Is(err, sql.ErrNoRows) {
			isNew = true
		} else if err != nil {
			return err
		} else if owner != p.ID {
			return invalid("variant belongs to another product")
		}
	}
	v.SKU = strings.TrimSpace(v.SKU)
	if v.SKU == "" {
		sku, err := nextSKU(ctx, tx, now)
		if err != nil {
			return err
		}
		v.SKU = sku
	}
	var clash string
	tx.QueryRowContext(ctx, "SELECT p.name FROM variants v JOIN products p ON p.id=v.product_id WHERE v.sku=? COLLATE NOCASE AND v.id<>?", v.SKU, v.ID).Scan(&clash)
	if clash != "" {
		return invalid("SKU %q is already used by %q", v.SKU, clash)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO variants(id,product_id,name,sku,price_minor,cost_minor,active,reorder_level_milli,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, sku=excluded.sku, price_minor=excluded.price_minor, cost_minor=excluded.cost_minor,
		active=excluded.active, reorder_level_milli=excluded.reorder_level_milli, updated_at=excluded.updated_at`,
		v.ID, p.ID, strings.TrimSpace(v.Name), v.SKU, v.PriceMinor, v.CostMinor, v.Active, v.ReorderLevelMilli, now, now); err != nil {
		return err
	}
	if err := recordChange(ctx, tx, protocol.EntityVariant, v.ID); err != nil {
		return err
	}

	// Barcodes: make the stored set match the requested set.
	want := map[string]bool{}
	for _, c := range vi.Barcodes {
		if c = strings.TrimSpace(c); c != "" {
			want[c] = true
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT code FROM barcodes WHERE variant_id=? AND deleted=0", v.ID)
	if err != nil {
		return err
	}
	var existing []string
	for rows.Next() {
		var c string
		rows.Scan(&c)
		existing = append(existing, c)
	}
	rows.Close()
	for _, c := range existing {
		if want[c] {
			delete(want, c)
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE barcodes SET deleted=1 WHERE code=?", c); err != nil {
			return err
		}
		if err := recordChange(ctx, tx, protocol.EntityBarcode, c); err != nil {
			return err
		}
	}
	for c := range want {
		var otherVariant, otherName string
		var deleted bool
		err := tx.QueryRowContext(ctx, `SELECT b.variant_id, b.deleted, COALESCE(p.name,'') FROM barcodes b
			LEFT JOIN variants v ON v.id=b.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE b.code=?`, c).Scan(&otherVariant, &deleted, &otherName)
		if err == nil && !deleted && otherVariant != v.ID {
			return invalid("barcode %s is already assigned to %q", c, otherName)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO barcodes(code,variant_id,pack_qty_milli,deleted) VALUES(?,?,1000,0)
			ON CONFLICT(code) DO UPDATE SET variant_id=excluded.variant_id, pack_qty_milli=1000, deleted=0`, c, v.ID); err != nil {
			return err
		}
		if err := recordChange(ctx, tx, protocol.EntityBarcode, c); err != nil {
			return err
		}
	}

	if isNew && vi.OpeningQtyMilli != 0 && p.TrackStock {
		loc := vi.OpeningLocation
		if loc == "" {
			tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='default_location_id'").Scan(&loc)
		}
		return addStock(ctx, tx, stockMove{VariantID: v.ID, LocationID: loc, DeltaMilli: vi.OpeningQtyMilli,
			Reason: "opening", RefType: "variant", RefID: v.ID, UserID: by.UserID})
	}
	return nil
}

func nextSKU(ctx context.Context, tx *sql.Tx, now int64) (string, error) {
	var cur string
	tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='next_sku'").Scan(&cur)
	n, _ := strconv.Atoi(cur)
	if n == 0 {
		n = 100001
	}
	for {
		sku := strconv.Itoa(n)
		n++
		var exists int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM variants WHERE sku=?", sku).Scan(&exists)
		if exists == 0 {
			return sku, putSetting(ctx, tx, "next_sku", strconv.Itoa(n), now)
		}
	}
}

// AdjustStock records a manual adjustment or a physical count.
func (h *Hub) AdjustStock(ctx context.Context, by *Session, req protocol.StockAdjustRequest) error {
	if !by.Can(catalogRoles...) {
		return ErrForbidden
	}
	if (req.DeltaMilli == nil) == (req.CountMilli == nil) {
		return invalid("give either a quantity change or a counted quantity")
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		loc := req.LocationID
		if loc == "" {
			tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='default_location_id'").Scan(&loc)
		}
		var n int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM variants WHERE id=?", req.VariantID).Scan(&n)
		if n == 0 {
			return invalid("unknown item")
		}
		m := stockMove{VariantID: req.VariantID, LocationID: loc, UserID: by.UserID, Note: req.Note, RefType: "manual"}
		if req.DeltaMilli != nil {
			m.DeltaMilli, m.Reason = *req.DeltaMilli, "adjustment"
		} else {
			var cur int64
			tx.QueryRowContext(ctx, "SELECT qty_milli FROM stock_levels WHERE variant_id=? AND location_id=?", req.VariantID, loc).Scan(&cur)
			m.DeltaMilli, m.Reason = *req.CountMilli-cur, "count"
		}
		if m.DeltaMilli == 0 {
			return nil
		}
		audit(ctx, tx, by.UserID, "", "stock."+m.Reason, "variant", req.VariantID, map[string]any{"delta_milli": m.DeltaMilli, "note": req.Note})
		return addStock(ctx, tx, m)
	})
}

// SaveCategory creates or updates a category.
func (h *Hub) SaveCategory(ctx context.Context, by *Session, c protocol.Category) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return "", invalid("category name is required")
	}
	if c.ID == "" {
		c.ID, c.Active = ids.New(), true
	}
	return c.ID, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		if _, err := tx.ExecContext(ctx, `INSERT INTO categories(id,parent_id,name,active,created_at,updated_at) VALUES(?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET parent_id=excluded.parent_id, name=excluded.name, active=excluded.active, updated_at=excluded.updated_at`,
			c.ID, c.ParentID, c.Name, c.Active, now, now); err != nil {
			return err
		}
		return recordChange(ctx, tx, protocol.EntityCategory, c.ID)
	})
}

// SaveUser creates or updates a staff account.
func (h *Hub) SaveUser(ctx context.Context, by *Session, in protocol.UserInput) (string, error) {
	if !by.Can(adminRoles...) {
		return "", ErrForbidden
	}
	in.Name, in.Username = strings.TrimSpace(in.Name), strings.TrimSpace(in.Username)
	switch in.Role {
	case protocol.RoleAdmin, protocol.RoleManager, protocol.RoleInventory, protocol.RoleCashier:
	default:
		return "", invalid("unknown role %q", in.Role)
	}
	if in.Name == "" || in.Username == "" {
		return "", invalid("name and username are required")
	}
	isNew := in.ID == ""
	if isNew && in.Password == "" && in.Pin == "" {
		return "", invalid("set a password or PIN for the new user")
	}
	if in.Password != "" && len(in.Password) < 6 {
		return "", invalid("password must be at least 6 characters")
	}
	if in.Pin != "" && (len(in.Pin) < 4 || strings.Trim(in.Pin, "0123456789") != "") {
		return "", invalid("PIN must be at least 4 digits")
	}
	var pwHash, pinHash string
	if in.Password != "" {
		b, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		pwHash = string(b)
	}
	if in.Pin != "" {
		b, err := bcrypt.GenerateFromPassword([]byte(in.Pin), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		pinHash = string(b)
	}
	if isNew {
		in.ID = ids.New()
	}
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		var clash int
		tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE username=? AND id<>?", in.Username, in.ID).Scan(&clash)
		if clash > 0 {
			return invalid("username %q is taken", in.Username)
		}
		if isNew {
			if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,name,username,role,password_hash,pin_hash,active,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`,
				in.ID, in.Name, in.Username, in.Role, pwHash, pinHash, in.Active, now, now); err != nil {
				return err
			}
		} else {
			res, err := tx.ExecContext(ctx, `UPDATE users SET name=?, username=?, role=?, active=?, updated_at=?,
				password_hash=CASE WHEN ?<>'' THEN ? ELSE password_hash END, pin_hash=CASE WHEN ?<>'' THEN ? ELSE pin_hash END WHERE id=?`,
				in.Name, in.Username, in.Role, in.Active, now, pwHash, pwHash, pinHash, pinHash, in.ID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return invalid("user not found")
			}
			var admins int
			tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE role='admin' AND active=1").Scan(&admins)
			if admins == 0 {
				return invalid("at least one active admin is required")
			}
		}
		if !in.Active {
			tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", in.ID)
		}
		audit(ctx, tx, by.UserID, "", map[bool]string{true: "user.create", false: "user.update"}[isNew], "user", in.ID,
			map[string]any{"username": in.Username, "role": in.Role, "active": in.Active})
		return recordChange(ctx, tx, protocol.EntityUser, in.ID)
	})
	return in.ID, err
}

// SavePaymentMethod creates or updates a payment method.
func (h *Hub) SavePaymentMethod(ctx context.Context, by *Session, m protocol.PaymentMethod) (string, error) {
	if !by.Can(adminRoles...) {
		return "", ErrForbidden
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return "", invalid("payment method name is required")
	}
	switch m.Kind {
	case protocol.PayCash, protocol.PayCard, protocol.PayTransfer, protocol.PayMobile, protocol.PayCredit, protocol.PayVoucher, protocol.PayOther:
	default:
		return "", invalid("unknown payment kind %q", m.Kind)
	}
	if m.ID == "" {
		m.ID = ids.New()
	}
	return m.ID, h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return upsertPaymentMethod(ctx, tx, m, ids.NowMs())
	})
}

func upsertPaymentMethod(ctx context.Context, tx *sql.Tx, m protocol.PaymentMethod, now int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO payment_methods(id,name,kind,requires_reference,opens_drawer,sort_order,active,updated_at) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, kind=excluded.kind, requires_reference=excluded.requires_reference,
		opens_drawer=excluded.opens_drawer, sort_order=excluded.sort_order, active=excluded.active, updated_at=excluded.updated_at`,
		m.ID, m.Name, m.Kind, m.RequiresReference, m.OpensDrawer, m.SortOrder, m.Active, now); err != nil {
		return err
	}
	return recordChange(ctx, tx, protocol.EntityPaymentMethod, m.ID)
}

// editableSettings are the store settings admins may change via the API.
var editableSettings = map[string]bool{
	"store_name": true, "business_type": true, "store_address": true, "store_phone": true, "store_email": true,
	"tax_id": true, "currency_code": true, "currency_symbol": true, "receipt_header": true, "receipt_footer": true,
	"tax_mode": true, "tax_name": true, "product_fields": true, "units": true, "default_tax_bp": true, "require_encryption": true,
	"approval_discount": true, "approval_price_override": true, "approval_refund": true, "approval_paid_out": true, "approval_no_sale": true,
}

// SaveSettings updates store-wide settings.
func (h *Hub) SaveSettings(ctx context.Context, by *Session, values map[string]string) error {
	if !by.Can(adminRoles...) {
		return ErrForbidden
	}
	for k := range values {
		if !editableSettings[k] {
			return invalid("setting %q cannot be changed", k)
		}
	}
	if err := validateShopSettings(values); err != nil {
		return err
	}
	if v, ok := values["tax_mode"]; ok && v != protocol.TaxNone && v != protocol.TaxInclusive && v != protocol.TaxExclusive {
		return invalid("tax mode must be none, inclusive or exclusive")
	}
	for k, v := range values {
		if (strings.HasPrefix(k, "approval_") || k == "require_encryption") && v != "0" && v != "1" {
			return invalid("%s must be 0 or 1", k)
		}
	}
	if v, ok := values["store_name"]; ok && strings.TrimSpace(v) == "" {
		return invalid("store name cannot be empty")
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		for k, v := range values {
			if err := putSetting(ctx, tx, k, strings.TrimSpace(v), now); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "settings.update", "", "", values)
		return nil
	})
}

// SystemSession is used for in-process administrative calls (CLI, seeding).
var SystemSession = &Session{UserID: "system", Name: "System", Role: protocol.RoleAdmin}

// Summary returns quick counts for status displays.
func (h *Hub) Summary(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for k, q := range map[string]string{
		"products":    "SELECT count(*) FROM products WHERE active=1",
		"terminals":   "SELECT count(*) FROM terminals WHERE active=1",
		"sales":       "SELECT count(*) FROM sales",
		"sales_today": fmt.Sprintf("SELECT count(*) FROM sales WHERE created_at >= %d", startOfDayMs()),
	} {
		var n int64
		if err := h.db.R().QueryRowContext(ctx, q).Scan(&n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, nil
}
