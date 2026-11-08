package terminal

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Writes random carts with Go-computed totals so the frontend's pricing.ts
// can be checked against the backend (PRICING_FIXTURE=path).
func TestPricingFixture(t *testing.T) {
	path := os.Getenv("PRICING_FIXTURE")
	if path == "" {
		t.Skip("set PRICING_FIXTURE")
	}
	type line struct{ Qty, Unit, BP int64 }
	type tc struct {
		Lines                []line
		Discount             int64
		Mode                 string
		Subtotal, Tax, Total int64
	}
	r := rand.New(rand.NewSource(1))
	var cases []tc
	for range 2000 {
		c := tc{Mode: []string{protocol.TaxNone, protocol.TaxInclusive, protocol.TaxExclusive}[r.Intn(3)]}
		for range 1 + r.Intn(6) {
			c.Lines = append(c.Lines, line{Qty: int64(1+r.Intn(5)) * 1000 / int64(1+r.Intn(4)), Unit: int64(r.Intn(2_000_000)), BP: []int64{0, 500, 750, 1000}[r.Intn(4)]})
		}
		var gross []int64
		for _, l := range c.Lines {
			g := money.LineTotal(l.Qty, l.Unit)
			gross = append(gross, g)
			c.Subtotal += g
		}
		if c.Subtotal > 0 {
			c.Discount = r.Int63n(c.Subtotal/2 + 1)
		}
		shares := allocate(c.Discount, gross)
		var net int64
		for i, g := range gross {
			n := g - shares[i]
			bp := c.Lines[i].BP
			if c.Mode == protocol.TaxNone {
				bp = 0
			}
			c.Tax += lineTax(c.Mode, n, bp)
			net += n
		}
		c.Total = net
		if c.Mode == protocol.TaxExclusive {
			c.Total += c.Tax
		}
		cases = append(cases, c)
	}
	b, _ := json.Marshal(cases)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
