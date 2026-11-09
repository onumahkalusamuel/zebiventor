// Package templates holds the starting set-up for each kind of shop:
// categories, units, the product details worth recording, payment methods
// and tax defaults. A template is applied at first-run setup, and can be
// applied again later from Settings; re-applying only adds what is missing.
package templates

import (
	"encoding/json"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Field is a product detail (stored in product attributes under Key).
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
}

// Template is the starting set-up for one kind of shop.
type Template struct {
	Type        string                   `json:"type"`
	Label       string                   `json:"label"`
	Description string                   `json:"description"`
	Categories  []string                 `json:"categories"`
	Units       []string                 `json:"units"`
	Fields      []Field                  `json:"fields"`
	Payments    []protocol.PaymentMethod `json:"payment_methods"`
	TaxMode     string                   `json:"tax_mode"`
	TaxName     string                   `json:"tax_name"`
	// DefaultTaxBP is the tax rate new products start with (basis points).
	DefaultTaxBP int64 `json:"default_tax_bp"`
}

var (
	cash     = protocol.PaymentMethod{Name: "Cash", Kind: protocol.PayCash, OpensDrawer: true}
	card     = protocol.PaymentMethod{Name: "POS / Card", Kind: protocol.PayCard}
	transfer = protocol.PaymentMethod{Name: "Bank Transfer", Kind: protocol.PayTransfer, RequiresReference: true}
	mobile   = protocol.PaymentMethod{Name: "Mobile money", Kind: protocol.PayMobile, RequiresReference: true}
	cheque   = protocol.PaymentMethod{Name: "Cheque", Kind: protocol.PayOther, RequiresReference: true}
	hmo      = protocol.PaymentMethod{Name: "HMO / Insurance", Kind: protocol.PayOther, RequiresReference: true}
)

var all = []Template{
	{
		Type: "general", Label: "General retail", Description: "Any shop: a broad starting point you can shape later.",
		Categories: []string{"General"},
		Units:      []string{"pc", "pack", "box", "kg", "g", "l", "ml", "m", "dozen"},
		Fields: []Field{
			{Key: "brand", Label: "Brand"},
			{Key: "model", Label: "Model"},
			{Key: "colour", Label: "Colour"},
		},
		Payments: []protocol.PaymentMethod{cash, card, transfer},
		TaxMode:  protocol.TaxNone, TaxName: "VAT",
	},
	{
		Type: "bookshop", Label: "Bookshop", Description: "Books by author and ISBN, plus stationery.",
		Categories: []string{"Fiction", "Non-fiction", "Children", "Textbooks", "Religious", "Magazines", "Stationery"},
		Units:      []string{"pc", "pack", "box", "dozen"},
		Fields: []Field{
			{Key: "author", Label: "Author"},
			{Key: "isbn", Label: "ISBN", Placeholder: "978…"},
			{Key: "publisher", Label: "Publisher"},
			{Key: "edition", Label: "Edition / Year"},
		},
		Payments: []protocol.PaymentMethod{cash, card, transfer},
		TaxMode:  protocol.TaxNone, TaxName: "VAT",
	},
	{
		Type: "grocery", Label: "Grocery / Supermarket", Description: "Packaged and loose goods, sold by the piece or by weight.",
		Categories: []string{"Beverages", "Provisions", "Dairy", "Frozen", "Fresh produce", "Snacks", "Household", "Toiletries", "Baby care"},
		Units:      []string{"pc", "kg", "g", "l", "ml", "pack", "carton", "crate", "dozen"},
		Fields: []Field{
			{Key: "brand", Label: "Brand"},
			{Key: "size", Label: "Pack size", Placeholder: "e.g. 50cl, 1kg"},
			{Key: "origin", Label: "Origin"},
		},
		Payments: []protocol.PaymentMethod{cash, card, transfer, mobile},
		TaxMode:  protocol.TaxInclusive, TaxName: "VAT", DefaultTaxBP: 750,
	},
	{
		Type: "pharmacy", Label: "Pharmacy", Description: "Medicines by generic name and strength; HMO payments.",
		Categories: []string{"Prescription medicines", "Over the counter", "Supplements", "Personal care", "Medical devices", "Mother & baby"},
		Units:      []string{"pc", "pack", "strip", "tab", "bottle", "box", "ml"},
		Fields: []Field{
			{Key: "generic_name", Label: "Generic name"},
			{Key: "strength", Label: "Strength", Placeholder: "e.g. 500mg"},
			{Key: "form", Label: "Form", Placeholder: "Tablet, syrup…"},
			{Key: "manufacturer", Label: "Manufacturer"},
			{Key: "nafdac_no", Label: "NAFDAC no."},
		},
		Payments: []protocol.PaymentMethod{cash, card, transfer, hmo},
		TaxMode:  protocol.TaxNone, TaxName: "VAT",
	},
	{
		Type: "distributor", Label: "Distributor / Wholesale", Description: "Cartons and packs, customers on account, cheques.",
		Categories: []string{"Beverages", "Food", "Household", "Personal care"},
		Units:      []string{"carton", "pack", "pc", "bag", "crate", "dozen", "kg"},
		Fields: []Field{
			{Key: "brand", Label: "Brand"},
			{Key: "pack", Label: "Pack", Placeholder: "e.g. Carton of 24"},
			{Key: "supplier_code", Label: "Supplier code"},
		},
		Payments: []protocol.PaymentMethod{cash, transfer, card, cheque},
		TaxMode:  protocol.TaxExclusive, TaxName: "VAT", DefaultTaxBP: 750,
	},
	{
		Type: "fashion", Label: "Fashion / Boutique", Description: "Clothing and shoes in sizes and colours.",
		Categories: []string{"Women", "Men", "Children", "Shoes", "Bags", "Accessories"},
		Units:      []string{"pc", "pair", "set"},
		Fields: []Field{
			{Key: "brand", Label: "Brand"},
			{Key: "material", Label: "Material"},
			{Key: "style_code", Label: "Style code"},
		},
		Payments: []protocol.PaymentMethod{cash, card, transfer},
		TaxMode:  protocol.TaxInclusive, TaxName: "VAT", DefaultTaxBP: 750,
	},
	{
		Type: "hardware", Label: "Hardware / Building materials", Description: "Sold by the piece, length, bag or weight.",
		Categories: []string{"Building materials", "Plumbing", "Electrical", "Paint", "Tools", "Fasteners"},
		Units:      []string{"pc", "bag", "m", "length", "kg", "l", "roll", "box", "pack"},
		Fields: []Field{
			{Key: "brand", Label: "Brand"},
			{Key: "size", Label: "Size / Gauge"},
			{Key: "material", Label: "Material"},
		},
		Payments: []protocol.PaymentMethod{cash, transfer, card, cheque},
		TaxMode:  protocol.TaxExclusive, TaxName: "VAT", DefaultTaxBP: 750,
	},
}

// All returns every template, general first.
func All() []Template { return all }

// Get returns the template for a shop type, falling back to general.
func Get(kind string) Template {
	for _, t := range all {
		if t.Type == kind {
			return t
		}
	}
	return all[0]
}

// Known reports whether kind names a template.
func Known(kind string) bool {
	for _, t := range all {
		if t.Type == kind {
			return true
		}
	}
	return false
}

// FieldsJSON and UnitsJSON are the setting values a template stores.
func (t Template) FieldsJSON() string { b, _ := json.Marshal(t.Fields); return string(b) }
func (t Template) UnitsJSON() string  { b, _ := json.Marshal(t.Units); return string(b) }
