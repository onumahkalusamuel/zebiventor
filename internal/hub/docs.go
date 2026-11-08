package hub

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
)

// nextDocNo issues the next number in a hub document series, e.g. PO-000042.
// Counters are private settings ("seq.<series>") so they never reach tills.
func nextDocNo(ctx context.Context, tx *sql.Tx, series, prefix string) (string, error) {
	key := "seq." + series
	var cur string
	tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&cur)
	n, _ := strconv.Atoi(cur)
	n++
	if err := putSetting(ctx, tx, key, strconv.Itoa(n), ids.NowMs()); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%06d", prefix, n), nil
}

// mulDiv returns a*b/c rounded half away from zero, without overflow.
func mulDiv(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	x := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	d := big.NewInt(c)
	q, r := new(big.Int).QuoRem(x, d, new(big.Int))
	// Round: compare 2|r| with |c|.
	r2 := new(big.Int).Abs(r)
	r2.Lsh(r2, 1)
	if r2.Cmp(new(big.Int).Abs(d)) >= 0 {
		if (x.Sign() < 0) != (d.Sign() < 0) {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Int64()
}

// defaultLocation returns the store's main location.
func defaultLocation(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) string {
	var loc string
	q.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='default_location_id'").Scan(&loc)
	return loc
}

func locationExists(ctx context.Context, tx *sql.Tx, id string) bool {
	var n int
	tx.QueryRowContext(ctx, "SELECT count(*) FROM locations WHERE id=? AND active=1", id).Scan(&n)
	return n > 0
}

// variantInfo is what back-office documents need to know about an item.
type variantInfo struct {
	ID, ProductName, VariantName, SKU, Unit, CategoryID string
	CostMinor, PriceMinor                               int64
	TrackStock, AllowFraction                           bool
}

func loadVariant(ctx context.Context, tx *sql.Tx, id string) (variantInfo, error) {
	var v variantInfo
	err := tx.QueryRowContext(ctx, `SELECT v.id, p.name, v.name, v.sku, p.unit, p.category_id, v.cost_minor, v.price_minor, p.track_stock, p.allow_fraction
		FROM variants v JOIN products p ON p.id=v.product_id WHERE v.id=?`, id).
		Scan(&v.ID, &v.ProductName, &v.VariantName, &v.SKU, &v.Unit, &v.CategoryID, &v.CostMinor, &v.PriceMinor, &v.TrackStock, &v.AllowFraction)
	if err == sql.ErrNoRows {
		return v, invalid("unknown item %s", id)
	}
	return v, err
}

func (v variantInfo) label() string {
	if v.VariantName != "" {
		return v.ProductName + " (" + v.VariantName + ")"
	}
	return v.ProductName
}

// checkQty validates a quantity for an item sold in whole units or fractions.
func (v variantInfo) checkQty(q int64) error {
	if q <= 0 {
		return invalid("quantity for %s must be more than zero", v.label())
	}
	if !v.AllowFraction && q%1000 != 0 {
		return invalid("%s is counted in whole units", v.label())
	}
	return nil
}

func trim(s string) string { return strings.TrimSpace(s) }
