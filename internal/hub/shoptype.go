package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/templates"
)

// TemplateResult says what applying a shop template added.
type TemplateResult struct {
	Categories     []string `json:"categories"`
	PaymentMethods []string `json:"payment_methods"`
}

// applyTemplate sets the shop type, product details and units, and adds
// any of the template's categories and payment methods that are missing.
// Nothing is removed or renamed. Tax settings change only when withTax.
func applyTemplate(ctx context.Context, tx *sql.Tx, t templates.Template, withTax bool, now int64) (TemplateResult, error) {
	var res TemplateResult
	set := map[string]string{"business_type": t.Type, "product_fields": t.FieldsJSON(), "units": t.UnitsJSON()}
	if withTax {
		set["tax_mode"], set["tax_name"], set["default_tax_bp"] = t.TaxMode, t.TaxName, strconv.FormatInt(t.DefaultTaxBP, 10)
	}
	for _, k := range []string{"business_type", "product_fields", "units", "tax_mode", "tax_name", "default_tax_bp"} {
		if v, ok := set[k]; ok {
			if err := putSetting(ctx, tx, k, v, now); err != nil {
				return res, err
			}
		}
	}

	have := func(q, name string) (bool, error) {
		var n int
		err := tx.QueryRowContext(ctx, q, name).Scan(&n)
		return n > 0, err
	}
	for _, name := range t.Categories {
		ok, err := have("SELECT count(*) FROM categories WHERE lower(name)=lower(?)", name)
		if err != nil {
			return res, err
		}
		if ok {
			continue
		}
		id := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO categories(id,name,active,created_at,updated_at) VALUES(?,?,1,?,?)`, id, name, now, now); err != nil {
			return res, err
		}
		if err := recordChange(ctx, tx, protocol.EntityCategory, id); err != nil {
			return res, err
		}
		res.Categories = append(res.Categories, name)
	}

	var order int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(max(sort_order),0) FROM payment_methods").Scan(&order); err != nil {
		return res, err
	}
	for _, m := range t.Payments {
		ok, err := have("SELECT count(*) FROM payment_methods WHERE lower(name)=lower(?)", m.Name)
		if err != nil {
			return res, err
		}
		if ok {
			continue
		}
		order++
		m.ID, m.Active, m.SortOrder = ids.New(), true, order
		if err := upsertPaymentMethod(ctx, tx, m, now); err != nil {
			return res, err
		}
		res.PaymentMethods = append(res.PaymentMethods, m.Name)
	}
	return res, nil
}

// ApplyTemplate switches the shop to another type's set-up (see applyTemplate).
func (h *Hub) ApplyTemplate(ctx context.Context, by *Session, kind string, withTax bool) (TemplateResult, error) {
	if !by.Can(adminRoles...) {
		return TemplateResult{}, ErrForbidden
	}
	if !templates.Known(kind) {
		return TemplateResult{}, invalid("unknown type of business %q", kind)
	}
	var res TemplateResult
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		res, err = applyTemplate(ctx, tx, templates.Get(kind), withTax, ids.NowMs())
		if err == nil {
			audit(ctx, tx, by.UserID, "", "settings.template", "", "", map[string]any{"type": kind, "with_tax": withTax,
				"categories": res.Categories, "payment_methods": res.PaymentMethods})
		}
		return err
	})
	return res, err
}

// validateShopSettings checks the JSON-valued shop settings.
func validateShopSettings(values map[string]string) error {
	if v, ok := values["product_fields"]; ok {
		var fields []templates.Field
		if err := json.Unmarshal([]byte(v), &fields); err != nil {
			return invalid("product details must be a list")
		}
		seen := map[string]bool{}
		for _, f := range fields {
			k := strings.TrimSpace(f.Key)
			if k == "" || strings.TrimSpace(f.Label) == "" {
				return invalid("every product detail needs a name")
			}
			if seen[k] {
				return invalid("product detail %q is listed twice", f.Label)
			}
			seen[k] = true
		}
		if len(fields) > 12 {
			return invalid("at most 12 product details")
		}
	}
	if v, ok := values["units"]; ok {
		var units []string
		if err := json.Unmarshal([]byte(v), &units); err != nil || len(units) == 0 {
			return invalid("units must be a list with at least one unit")
		}
		for _, u := range units {
			if strings.TrimSpace(u) == "" || len(u) > 12 {
				return invalid("unit names must be 1 to 12 characters")
			}
		}
	}
	if v, ok := values["default_tax_bp"]; ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 || n > 10000 {
			return invalid("default tax rate must be between 0 and 100%%")
		}
	}
	if v, ok := values["business_type"]; ok && !templates.Known(v) {
		return invalid("unknown type of business %q", v)
	}
	return nil
}

// backfillShopSettings gives stores set up before templates existed their
// type's product details and units (nothing else changes).
func (h *Hub) backfillShopSettings(ctx context.Context) error {
	if !h.IsSetup(ctx) {
		return nil
	}
	if v, _ := h.Setting(ctx, "product_fields"); v != "" {
		return nil
	}
	kind, _ := h.Setting(ctx, "business_type")
	t := templates.Get(kind)
	return h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		if err := putSetting(ctx, tx, "product_fields", t.FieldsJSON(), now); err != nil {
			return err
		}
		return putSetting(ctx, tx, "units", t.UnitsJSON(), now)
	})
}
