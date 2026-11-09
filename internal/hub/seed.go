package hub

import (
	"context"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// SeedDemo adds a small sample catalogue so a new installation can be
// tried immediately. It is safe to call once on a fresh store.
func (h *Hub) SeedDemo(ctx context.Context) error {
	cats := map[string]string{}
	for _, name := range []string{"Fiction", "Non-fiction", "Children", "Stationery"} {
		var id string
		_ = h.db.R().QueryRowContext(ctx, "SELECT id FROM categories WHERE lower(name)=lower(?)", name).Scan(&id)
		if id == "" {
			var err error
			if id, err = h.SaveCategory(ctx, SystemSession, protocol.Category{Name: name}); err != nil {
				return err
			}
		}
		cats[name] = id
	}
	type book struct {
		title, author, isbn, cat string
		price, stock             int64
	}
	books := []book{
		{"Things Fall Apart", "Chinua Achebe", "9780385474542", "Fiction", 450000, 40},
		{"Half of a Yellow Sun", "Chimamanda Ngozi Adichie", "9781400095209", "Fiction", 650000, 25},
		{"Purple Hibiscus", "Chimamanda Ngozi Adichie", "9781616202415", "Fiction", 550000, 18},
		{"The Famished Road", "Ben Okri", "9780385425131", "Fiction", 600000, 7},
		{"Stay With Me", "Ayobami Adebayo", "9781101946077", "Fiction", 520000, 12},
		{"There Was a Country", "Chinua Achebe", "9780143124030", "Non-fiction", 700000, 9},
		{"Born a Crime", "Trevor Noah", "9780399588198", "Non-fiction", 800000, 15},
		{"Anna Hibiscus", "Atinuke", "9781877467226", "Children", 300000, 30},
	}
	for _, b := range books {
		_, err := h.SaveProduct(ctx, SystemSession, protocol.ProductInput{
			Product: protocol.Product{Name: b.title, CategoryID: cats[b.cat], TrackStock: true, Unit: "pc",
				Attributes: map[string]string{"author": b.author, "isbn": b.isbn}},
			Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: b.price, CostMinor: b.price * 7 / 10},
				Barcodes: []string{b.isbn}, OpeningQtyMilli: b.stock * 1000}},
		})
		if err != nil {
			return err
		}
	}
	// A product with variants and a non-book item.
	if _, err := h.SaveProduct(ctx, SystemSession, protocol.ProductInput{
		Product: protocol.Product{Name: "Exercise Book", CategoryID: cats["Stationery"], TrackStock: true, Unit: "pc"},
		Variants: []protocol.VariantInput{
			{Variant: protocol.Variant{Name: "40 leaves", PriceMinor: 30000}, Barcodes: []string{"6151100000401"}, OpeningQtyMilli: 200_000},
			{Variant: protocol.Variant{Name: "80 leaves", PriceMinor: 50000}, Barcodes: []string{"6151100000802"}, OpeningQtyMilli: 150_000},
		},
	}); err != nil {
		return err
	}
	_, err := h.SaveProduct(ctx, SystemSession, protocol.ProductInput{
		Product:  protocol.Product{Name: "Book wrapping (gift)", CategoryID: cats["Stationery"], TrackStock: false, Unit: "pc"},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 50000}}},
	})
	return err
}
