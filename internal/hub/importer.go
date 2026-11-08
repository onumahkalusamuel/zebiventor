package hub

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Catalogue CSV columns. Headers are matched case-insensitively; any other
// column becomes a product attribute (author, isbn, brand, strength...).
var importAliases = map[string]string{
	"name": "name", "product": "name", "product_name": "name", "title": "name", "item": "name",
	"variant": "variant", "variant_name": "variant", "size": "", "option": "variant",
	"sku": "sku", "code": "sku", "item_code": "sku",
	"barcode": "barcode", "barcodes": "barcode", "ean": "barcode", "upc": "barcode",
	"price": "price", "selling_price": "price", "retail_price": "price", "unit_price": "price",
	"cost": "cost", "cost_price": "cost", "unit_cost": "cost", "buying_price": "cost",
	"quantity": "qty", "qty": "qty", "stock": "qty", "opening_stock": "qty", "on_hand": "qty",
	"category": "category", "department": "category",
	"unit": "unit", "uom": "unit",
	"tax": "tax", "tax_rate": "tax", "vat": "tax", "vat_rate": "tax",
	"reorder_level": "reorder", "reorder": "reorder", "min_stock": "reorder",
	"description": "description",
	"track_stock": "track", "sold_by_weight": "fraction",
}

const errDryRunMsg = "dry run"

var errDryRun = errors.New(errDryRunMsg)

// ImportRequest carries a CSV file's text.
type ImportRequest struct {
	CSV        string `json:"csv"`
	DryRun     bool   `json:"dry_run"`
	LocationID string `json:"location_id"` // where opening stock of new items goes
}

type importRow struct {
	line                 int
	name, variant, sku   string
	barcodes             []string
	price, cost, qty     *int64
	reorder, tax         *int64
	category, unit, desc string
	track, fraction      *bool
	attrs                map[string]string
}

// ImportProducts creates and updates products from a CSV file. Existing
// items are matched by SKU, then barcode; for them, only the columns present
// are changed and quantity is ignored (use a stock count). Rows with the
// same name and a variant column become variants of one product. The whole
// file is applied in one transaction; DryRun reports what would happen.
func (h *Hub) ImportProducts(ctx context.Context, by *Session, req ImportRequest) (*protocol.ImportResult, error) {
	if !by.Can(catalogRoles...) {
		return nil, ErrForbidden
	}
	res := &protocol.ImportResult{DryRun: req.DryRun, Errors: []protocol.ImportError{}, Category: []string{}}
	digits := 2
	var d string
	h.db.R().QueryRowContext(ctx, "SELECT value FROM settings WHERE key='currency_digits'").Scan(&d)
	if n, err := strconv.Atoi(d); err == nil {
		digits = n
	}

	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(req.CSV, "\ufeff")))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, invalid("the file is empty or not a CSV file")
	}
	cols := make([]string, len(header))
	hasName := false
	for i, hname := range header {
		k := strings.ToLower(strings.TrimSpace(hname))
		k = strings.NewReplacer(" ", "_", "-", "_", ".", "").Replace(k)
		if a, ok := importAliases[k]; ok && a != "" {
			cols[i] = a
		} else if k != "" {
			cols[i] = "attr:" + k
		}
		if cols[i] == "name" {
			hasName = true
		}
		res.Columns = append(res.Columns, cols[i])
	}
	if !hasName {
		return nil, invalid("the file needs a name (or title/product) column")
	}

	var rows []importRow
	line := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			res.Errors = append(res.Errors, protocol.ImportError{Row: line, Message: err.Error()})
			continue
		}
		row, err := parseImportRow(cols, rec, digits)
		row.line = line
		if err != nil {
			res.Errors = append(res.Errors, protocol.ImportError{Row: line, Message: err.Error()})
			continue
		}
		if row.name == "" && row.sku == "" && len(row.barcodes) == 0 {
			continue // blank line
		}
		rows = append(rows, row)
	}
	res.Rows = len(rows) + len(res.Errors)
	if len(rows) > 20000 {
		return nil, invalid("import at most 20,000 rows at a time")
	}

	err = h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		loc := req.LocationID
		if loc == "" {
			loc = defaultLocation(ctx, tx)
		}
		if !locationExists(ctx, tx, loc) {
			return invalid("unknown location")
		}
		cats := map[string]string{}
		crow, err := tx.QueryContext(ctx, "SELECT id, name FROM categories")
		if err != nil {
			return err
		}
		for crow.Next() {
			var id, name string
			crow.Scan(&id, &name)
			cats[strings.ToLower(name)] = id
		}
		crow.Close()
		newProducts := map[string]string{} // lower(name) -> product id created in this import (for variant rows)

		for _, row := range rows {
			if _, err := tx.ExecContext(ctx, "SAVEPOINT import_row"); err != nil {
				return err
			}
			nCat := len(res.Category)
			created, err := h.importOne(ctx, tx, by, row, loc, cats, newProducts, res)
			if err != nil {
				var ve validationError
				if !errors.As(err, &ve) {
					return err
				}
				tx.ExecContext(ctx, "ROLLBACK TO import_row")
				tx.ExecContext(ctx, "RELEASE import_row")
				for _, name := range res.Category[nCat:] { // created inside the rolled-back row
					delete(cats, strings.ToLower(name))
				}
				res.Category = res.Category[:nCat]
				res.Errors = append(res.Errors, protocol.ImportError{Row: row.line, Message: ve.Error()})
				continue
			}
			if _, err := tx.ExecContext(ctx, "RELEASE import_row"); err != nil {
				return err
			}
			if created {
				res.Created++
			} else {
				res.Updated++
			}
		}
		if req.DryRun {
			return errDryRun
		}
		audit(ctx, tx, by.UserID, "", "catalogue.import", "", "", map[string]any{"created": res.Created, "updated": res.Updated, "errors": len(res.Errors)})
		return nil
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return nil, err
	}
	sort.Slice(res.Errors, func(i, j int) bool { return res.Errors[i].Row < res.Errors[j].Row })
	res.Skipped = len(res.Errors)
	return res, nil
}

func (h *Hub) importOne(ctx context.Context, tx *sql.Tx, by *Session, row importRow, loc string, cats, newProducts map[string]string, res *protocol.ImportResult) (bool, error) {
	// Find an existing variant by SKU, then by barcode.
	var variantID string
	if row.sku != "" {
		tx.QueryRowContext(ctx, "SELECT id FROM variants WHERE sku=? COLLATE NOCASE", row.sku).Scan(&variantID)
	}
	if variantID == "" {
		for _, b := range row.barcodes {
			tx.QueryRowContext(ctx, "SELECT variant_id FROM barcodes WHERE code=? AND deleted=0", b).Scan(&variantID)
			if variantID != "" {
				break
			}
		}
	}

	categoryID := ""
	if row.category != "" {
		categoryID = cats[strings.ToLower(row.category)]
		if categoryID == "" {
			id, err := h.saveCategoryTx(ctx, tx, protocol.Category{Name: row.category})
			if err != nil {
				return false, err
			}
			cats[strings.ToLower(row.category)] = id
			categoryID = id
			res.Category = append(res.Category, row.category)
		}
	}

	if variantID != "" {
		var productID string
		tx.QueryRowContext(ctx, "SELECT product_id FROM variants WHERE id=?", variantID).Scan(&productID)
		in, err := loadProductInput(ctx, tx, productID)
		if err != nil {
			return false, err
		}
		applyRowToProduct(&in.Product, row, categoryID)
		for i := range in.Variants {
			v := &in.Variants[i]
			if v.ID != variantID {
				continue
			}
			if row.variant != "" {
				v.Name = row.variant
			}
			if row.sku != "" {
				v.SKU = row.sku
			}
			if row.price != nil {
				v.PriceMinor = *row.price
			}
			if row.cost != nil {
				v.CostMinor = *row.cost
			}
			if row.reorder != nil {
				v.ReorderLevelMilli = *row.reorder
			}
			have := map[string]bool{}
			for _, b := range v.Barcodes {
				have[b] = true
			}
			for _, b := range row.barcodes {
				if !have[b] {
					v.Barcodes = append(v.Barcodes, b)
				}
			}
		}
		_, err = h.saveProductTx(ctx, tx, by, in)
		return false, err
	}

	// New item.
	if row.name == "" {
		return false, invalid("a name is needed for a new item")
	}
	if row.price == nil {
		return false, invalid("a price is needed for a new item")
	}
	vi := protocol.VariantInput{Variant: protocol.Variant{Name: row.variant, SKU: row.sku, PriceMinor: *row.price}, Barcodes: row.barcodes, OpeningLocation: loc}
	if row.cost != nil {
		vi.CostMinor = *row.cost
	}
	if row.reorder != nil {
		vi.ReorderLevelMilli = *row.reorder
	}
	if row.qty != nil {
		vi.OpeningQtyMilli = *row.qty
	}
	key := strings.ToLower(row.name)
	if pid := newProducts[key]; pid != "" && row.variant != "" {
		// Another variant of a product created earlier in this file.
		in, err := loadProductInput(ctx, tx, pid)
		if err != nil {
			return false, err
		}
		for i := range in.Variants {
			in.Variants[i].OpeningQtyMilli = 0
		}
		in.Variants = append(in.Variants, vi)
		_, err = h.saveProductTx(ctx, tx, by, in)
		return true, err
	}
	p := protocol.Product{Name: row.name, Unit: "pc", TrackStock: true, Active: true}
	applyRowToProduct(&p, row, categoryID)
	id, err := h.saveProductTx(ctx, tx, by, protocol.ProductInput{Product: p, Variants: []protocol.VariantInput{vi}})
	if err == nil && row.variant != "" {
		newProducts[key] = id
	}
	return true, err
}

func applyRowToProduct(p *protocol.Product, row importRow, categoryID string) {
	if row.name != "" {
		p.Name = row.name
	}
	if categoryID != "" {
		p.CategoryID = categoryID
	}
	if row.unit != "" {
		p.Unit = row.unit
	}
	if row.desc != "" {
		p.Description = row.desc
	}
	if row.tax != nil {
		p.TaxRateBP = *row.tax
	}
	if row.track != nil {
		p.TrackStock = *row.track
	}
	if row.fraction != nil {
		p.AllowFraction = *row.fraction
	}
	if p.Attributes == nil {
		p.Attributes = map[string]string{}
	}
	for k, v := range row.attrs {
		p.Attributes[k] = v
	}
}

func parseImportRow(cols []string, rec []string, digits int) (importRow, error) {
	row := importRow{attrs: map[string]string{}}
	for i, raw := range rec {
		if i >= len(cols) || cols[i] == "" {
			continue
		}
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		var err error
		switch c := cols[i]; c {
		case "name":
			row.name = v
		case "variant":
			row.variant = v
		case "sku":
			row.sku = v
		case "barcode":
			for _, b := range strings.FieldsFunc(v, func(r rune) bool { return r == '|' || r == ';' || r == ' ' }) {
				row.barcodes = append(row.barcodes, b)
			}
		case "price", "cost":
			var m int64
			if m, err = parseDecimal(v, digits); err == nil {
				if m < 0 {
					err = fmt.Errorf("%s cannot be negative", c)
				} else if c == "price" {
					row.price = &m
				} else {
					row.cost = &m
				}
			}
		case "qty", "reorder":
			var q int64
			if q, err = parseDecimal(v, 3); err == nil {
				if c == "qty" {
					row.qty = &q
				} else if q < 0 {
					err = errors.New("reorder level cannot be negative")
				} else {
					row.reorder = &q
				}
			}
		case "tax":
			var bp int64
			if bp, err = parseDecimal(strings.TrimSuffix(v, "%"), 2); err == nil {
				if bp < 0 || bp > 10000 {
					err = errors.New("tax rate must be between 0 and 100")
				}
				row.tax = &bp
			}
		case "category":
			row.category = v
		case "unit":
			row.unit = strings.ToLower(v)
		case "description":
			row.desc = v
		case "track", "fraction":
			b := parseBool(v)
			if c == "track" {
				row.track = &b
			} else {
				row.fraction = &b
			}
		default:
			if k, ok := strings.CutPrefix(c, "attr:"); ok {
				row.attrs[k] = v
			}
		}
		if err != nil {
			return row, fmt.Errorf("column %d (%q): %v", i+1, v, err)
		}
	}
	// A bookshop's ISBN is its barcode unless one is given.
	if len(row.barcodes) == 0 {
		if isbn := strings.ReplaceAll(row.attrs["isbn"], "-", ""); isbn != "" {
			row.barcodes = []string{isbn}
		}
	}
	return row, nil
}

// parseDecimal parses "1,234.5" into an integer with the given number of
// decimal places (1234.5 with 2 digits -> 123450).
func parseDecimal(s string, digits int) (int64, error) {
	s = strings.NewReplacer(",", "", " ", "", "₦", "", "$", "", "£", "", "€", "").Replace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" && frac == "" {
		return 0, errors.New("not a number")
	}
	if strings.Trim(whole+frac, "0123456789") != "" {
		return 0, errors.New("not a number")
	}
	if len(frac) > digits {
		// Round the extra digits half up.
		extra := frac[digits:]
		frac = frac[:digits]
		n, err := parseDecimal(whole+"."+frac, digits)
		if err != nil {
			return 0, err
		}
		if extra[0] >= '5' {
			n++
		}
		if neg {
			n = -n
		}
		return n, nil
	}
	frac += strings.Repeat("0", digits-len(frac))
	w, err := strconv.ParseInt("0"+whole, 10, 64)
	if err != nil || w > math.MaxInt64/int64(math.Pow10(digits)) {
		return 0, errors.New("number too large")
	}
	f, _ := strconv.ParseInt("0"+frac, 10, 64)
	n := w*int64(math.Pow10(digits)) + f
	if neg {
		n = -n
	}
	return n, nil
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "y", "yes", "true", "t", "x":
		return true
	}
	return false
}

func formatDecimal(n int64, digits int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	p := int64(math.Pow10(digits))
	s := strconv.FormatInt(n/p, 10)
	if digits > 0 {
		s += "." + fmt.Sprintf("%0*d", digits, n%p)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ExportProducts writes the catalogue as CSV in the import format.
func (h *Hub) ExportProducts(ctx context.Context, by *Session, locationID string) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	r := h.db.R()
	if locationID == "" {
		locationID = defaultLocation(ctx, r)
	}
	digits := 2
	var d string
	r.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='currency_digits'").Scan(&d)
	if n, err := strconv.Atoi(d); err == nil {
		digits = n
	}
	rows, err := r.QueryContext(ctx, `SELECT p.name, v.name, v.sku, COALESCE((SELECT group_concat(code,'|') FROM barcodes b WHERE b.variant_id=v.id AND b.deleted=0),''),
		v.price_minor, v.cost_minor, COALESCE(s.qty_milli,0), COALESCE(c.name,''), p.unit, p.tax_rate_bp, v.reorder_level_milli, p.track_stock, p.allow_fraction,
		p.description, p.attributes
		FROM variants v JOIN products p ON p.id=v.product_id LEFT JOIN categories c ON c.id=p.category_id
		LEFT JOIN stock_levels s ON s.variant_id=v.id AND s.location_id=?
		WHERE p.active=1 AND v.active=1 ORDER BY p.name COLLATE NOCASE, v.name`, locationID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type rec struct {
		fixed []string
		attrs map[string]string
	}
	var recs []rec
	attrKeys := map[string]bool{}
	for rows.Next() {
		var name, vname, sku, codes, cat, unit, desc, attrs string
		var price, cost, qty, tax, reorder int64
		var track, fraction bool
		if err := rows.Scan(&name, &vname, &sku, &codes, &price, &cost, &qty, &cat, &unit, &tax, &reorder, &track, &fraction, &desc, &attrs); err != nil {
			return "", err
		}
		a := map[string]string{}
		_ = json.Unmarshal([]byte(attrs), &a)
		for k := range a {
			attrKeys[k] = true
		}
		yn := map[bool]string{true: "yes", false: "no"}
		recs = append(recs, rec{fixed: []string{name, vname, sku, codes, formatDecimal(price, digits), formatDecimal(cost, digits),
			strings.TrimSuffix(strings.TrimRight(formatDecimal(qty, 3), "0"), "."), cat, unit, strings.TrimSuffix(strings.TrimRight(formatDecimal(tax, 2), "0"), "."),
			strings.TrimSuffix(strings.TrimRight(formatDecimal(reorder, 3), "0"), "."), yn[track], yn[fraction], desc}, attrs: a})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	keys := make([]string, 0, len(attrKeys))
	for k := range attrKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(append([]string{"name", "variant", "sku", "barcode", "price", "cost", "quantity", "category", "unit", "tax_rate", "reorder_level", "track_stock", "sold_by_weight", "description"}, keys...))
	for _, r := range recs {
		row := r.fixed
		for _, k := range keys {
			row = append(row, r.attrs[k])
		}
		w.Write(row)
	}
	w.Flush()
	return buf.String(), w.Error()
}

// saveCategoryTx creates a category inside an existing transaction.
func (h *Hub) saveCategoryTx(ctx context.Context, tx *sql.Tx, c protocol.Category) (string, error) {
	c.Name = trim(c.Name)
	if c.Name == "" {
		return "", invalid("category name is required")
	}
	if c.ID == "" {
		c.ID, c.Active = ids.New(), true
	}
	now := ids.NowMs()
	if _, err := tx.ExecContext(ctx, `INSERT INTO categories(id,parent_id,name,active,created_at,updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET parent_id=excluded.parent_id, name=excluded.name, active=excluded.active, updated_at=excluded.updated_at`,
		c.ID, c.ParentID, c.Name, c.Active, now, now); err != nil {
		return "", err
	}
	return c.ID, recordChange(ctx, tx, protocol.EntityCategory, c.ID)
}
