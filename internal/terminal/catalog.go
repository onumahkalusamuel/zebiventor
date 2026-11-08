package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Item is a sellable variant as shown at the till.
type Item struct {
	VariantID     string            `json:"variant_id"`
	ProductID     string            `json:"product_id"`
	Name          string            `json:"name"`
	VariantName   string            `json:"variant_name"`
	SKU           string            `json:"sku"`
	PriceMinor    int64             `json:"price_minor"`
	Unit          string            `json:"unit"`
	AllowFraction bool              `json:"allow_fraction"`
	TrackStock    bool              `json:"track_stock"`
	StockMilli    int64             `json:"stock_milli"`
	CategoryID    string            `json:"category_id"`
	Attributes    map[string]string `json:"attributes"`
	// PackQtyMilli is set when the item was found by a pack barcode.
	PackQtyMilli int64 `json:"pack_qty_milli"`
}

// Settings returns all synced store settings.
func (t *Terminal) Settings(ctx context.Context) map[string]string {
	out := map[string]string{}
	rows, err := t.db.R().QueryContext(ctx, "SELECT key,value FROM settings")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		out[k] = v
	}
	return out
}

// Currency returns the store currency.
func (t *Terminal) Currency(ctx context.Context) money.Currency {
	s := t.Settings(ctx)
	c := money.Currency{Code: s["currency_code"], Symbol: s["currency_symbol"], Digits: 2}
	if d, err := strconv.Atoi(s["currency_digits"]); err == nil {
		c.Digits = d
	}
	if c.Code == "" {
		c.Code, c.Symbol = "NGN", "₦"
	}
	return c
}

// Stock on this till's view = last level from the hub minus this till's
// sales that the hub hasn't counted yet.
const itemSelect = `SELECT v.id, p.id, p.name, v.name, v.sku, v.price_minor, p.unit, p.allow_fraction, p.track_stock,
	COALESCE((SELECT qty_milli FROM stock_levels WHERE variant_id=v.id AND location_id=?),0)
	- COALESCE((SELECT sum(l.qty_milli) FROM sale_lines l JOIN sales s ON s.id=l.sale_id WHERE l.variant_id=v.id AND s.synced_at=0 AND l.track_stock=1),0),
	p.category_id, p.attributes
	FROM variants v JOIN products p ON p.id=v.product_id`

func scanItem(r interface{ Scan(...any) error }) (Item, error) {
	var it Item
	var attrs string
	err := r.Scan(&it.VariantID, &it.ProductID, &it.Name, &it.VariantName, &it.SKU, &it.PriceMinor, &it.Unit,
		&it.AllowFraction, &it.TrackStock, &it.StockMilli, &it.CategoryID, &attrs)
	if err == nil {
		_ = json.Unmarshal([]byte(attrs), &it.Attributes)
	}
	return it, err
}

// LookupCode finds an item by exact barcode or SKU (what a scanner sends).
func (t *Terminal) LookupCode(ctx context.Context, code string) (*Item, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	p, _ := t.Pairing(ctx)
	var pack int64 = money.QtyScale
	row := t.db.R().QueryRowContext(ctx, itemSelect+` JOIN barcodes b ON b.variant_id=v.id
		WHERE b.code=? AND b.deleted=0 AND v.active=1 AND p.active=1`, p.LocationID, code)
	it, err := scanItem(row)
	if err == nil {
		t.db.R().QueryRowContext(ctx, "SELECT pack_qty_milli FROM barcodes WHERE code=?", code).Scan(&pack)
		it.PackQtyMilli = pack
		return &it, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	row = t.db.R().QueryRowContext(ctx, itemSelect+` WHERE v.sku=? COLLATE NOCASE AND v.active=1 AND p.active=1`, p.LocationID, code)
	it, err = scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	it.PackQtyMilli = pack
	return &it, nil
}

// SearchItems does a word-by-word search over names, attributes (ISBN,
// author, ...) and SKUs.
func (t *Terminal) SearchItems(ctx context.Context, query string, limit int) ([]Item, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	p, _ := t.Pairing(ctx)
	where := []string{"v.active=1", "p.active=1"}
	args := []any{p.LocationID}
	for _, w := range strings.Fields(strings.ToLower(query)) {
		where = append(where, "(p.search LIKE ? ESCAPE '\\' OR v.sku LIKE ? ESCAPE '\\' OR lower(v.name) LIKE ? ESCAPE '\\')")
		like := "%" + escapeLike(w) + "%"
		args = append(args, like, like, like)
	}
	args = append(args, limit)
	rows, err := t.db.R().QueryContext(ctx, itemSelect+" WHERE "+strings.Join(where, " AND ")+" ORDER BY p.name COLLATE NOCASE, v.name LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		it.PackQtyMilli = money.QtyScale
		out = append(out, it)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// PaymentMethods returns active payment methods in display order.
func (t *Terminal) PaymentMethods(ctx context.Context) ([]protocol.PaymentMethod, error) {
	rows, err := t.db.R().QueryContext(ctx, "SELECT id,name,kind,requires_reference,opens_drawer,sort_order,active FROM payment_methods WHERE active=1 ORDER BY sort_order, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.PaymentMethod{}
	for rows.Next() {
		var m protocol.PaymentMethod
		if err := rows.Scan(&m.ID, &m.Name, &m.Kind, &m.RequiresReference, &m.OpensDrawer, &m.SortOrder, &m.Active); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Categories returns active categories.
func (t *Terminal) Categories(ctx context.Context) ([]protocol.Category, error) {
	rows, err := t.db.R().QueryContext(ctx, "SELECT id,parent_id,name,active FROM categories WHERE active=1 ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.Category{}
	for rows.Next() {
		var c protocol.Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ProductDetail is a product with its variants for the catalogue screen.
type ProductDetail struct {
	protocol.Product
	CategoryName string          `json:"category_name"`
	Variants     []VariantDetail `json:"variants"`
}

// VariantDetail adds barcodes and stock to a variant.
type VariantDetail struct {
	protocol.Variant
	Barcodes   []string `json:"barcodes"`
	StockMilli int64    `json:"stock_milli"`
}

// ListProducts returns products (including inactive ones) matching query.
func (t *Terminal) ListProducts(ctx context.Context, query string, limit int) ([]ProductDetail, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	p, _ := t.Pairing(ctx)
	where := []string{"1=1"}
	var args []any
	for _, w := range strings.Fields(strings.ToLower(query)) {
		like := "%" + escapeLike(w) + "%"
		where = append(where, `(p.search LIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM variants v WHERE v.product_id=p.id AND v.sku LIKE ? ESCAPE '\')
			OR EXISTS (SELECT 1 FROM barcodes b JOIN variants v ON v.id=b.variant_id WHERE v.product_id=p.id AND b.code=? AND b.deleted=0))`)
		args = append(args, like, like, w)
	}
	args = append(args, limit)
	rows, err := t.db.R().QueryContext(ctx, `SELECT p.id,p.name,p.category_id,p.description,p.attributes,p.unit,p.allow_fraction,p.track_stock,
		p.tax_rate_bp,p.active,p.updated_at,COALESCE(c.name,'') FROM products p LEFT JOIN categories c ON c.id=p.category_id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY p.active DESC, p.name COLLATE NOCASE LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	var out []ProductDetail
	index := map[string]int{}
	for rows.Next() {
		var d ProductDetail
		var attrs string
		if err := rows.Scan(&d.ID, &d.Name, &d.CategoryID, &d.Description, &attrs, &d.Unit, &d.AllowFraction, &d.TrackStock,
			&d.TaxRateBP, &d.Active, &d.UpdatedAt, &d.CategoryName); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal([]byte(attrs), &d.Attributes)
		if d.Attributes == nil {
			d.Attributes = map[string]string{}
		}
		d.Variants = []VariantDetail{}
		index[d.ID] = len(out)
		out = append(out, d)
	}
	rows.Close()
	if len(out) == 0 {
		return []ProductDetail{}, rows.Err()
	}

	ids := make([]any, 0, len(out)+1)
	ids = append(ids, p.LocationID)
	for _, d := range out {
		ids = append(ids, d.ID)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(out)), ",")
	vrows, err := t.db.R().QueryContext(ctx, `SELECT v.id,v.product_id,v.name,v.sku,v.price_minor,v.cost_minor,v.active,
		COALESCE((SELECT qty_milli FROM stock_levels WHERE variant_id=v.id AND location_id=?1),0)
		- COALESCE((SELECT sum(l.qty_milli) FROM sale_lines l JOIN sales s ON s.id=l.sale_id WHERE l.variant_id=v.id AND s.synced_at=0 AND l.track_stock=1),0),
		COALESCE((SELECT group_concat(code, char(31)) FROM barcodes WHERE variant_id=v.id AND deleted=0),'')
		FROM variants v WHERE v.product_id IN (`+ph+`) ORDER BY v.name`, ids...)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var vd VariantDetail
		var codes string
		if err := vrows.Scan(&vd.ID, &vd.ProductID, &vd.Name, &vd.SKU, &vd.PriceMinor, &vd.CostMinor, &vd.Active, &vd.StockMilli, &codes); err != nil {
			return nil, err
		}
		vd.Barcodes = []string{}
		if codes != "" {
			vd.Barcodes = strings.Split(codes, "\x1f")
		}
		i := index[vd.ProductID]
		out[i].Variants = append(out[i].Variants, vd)
	}
	return out, vrows.Err()
}

// StaffUser is a user without credential hashes.
type StaffUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Active   bool   `json:"active"`
	HasPin   bool   `json:"has_pin"`
}

// Users lists staff accounts (synced from the hub).
func (t *Terminal) Users(ctx context.Context) ([]StaffUser, error) {
	rows, err := t.db.R().QueryContext(ctx, "SELECT id,name,username,role,active,pin_hash<>'' FROM users ORDER BY active DESC, name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaffUser{}
	for rows.Next() {
		var u StaffUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.Role, &u.Active, &u.HasPin); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// HasUsers reports whether any user data has been synced yet.
func (t *Terminal) HasUsers(ctx context.Context) bool {
	var n int
	t.db.R().QueryRowContext(ctx, "SELECT count(*) FROM users WHERE active=1").Scan(&n)
	return n > 0
}
