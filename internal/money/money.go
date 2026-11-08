// Package money holds integer money and quantity arithmetic.
//
// Money is always int64 minor units (kobo for NGN). Quantities are int64
// thousandths ("milli-units") so that weighed goods (1.250 kg) and pack
// conversions work without floating point.
package money

import (
	"strconv"
	"strings"
)

// QtyScale is the number of milli-units in one unit.
const QtyScale = 1000

// LineTotal returns qtyMilli * unitPrice rounded half away from zero.
func LineTotal(qtyMilli, unitPriceMinor int64) int64 {
	p := qtyMilli * unitPriceMinor
	if p >= 0 {
		return (p + QtyScale/2) / QtyScale
	}
	return (p - QtyScale/2) / QtyScale
}

// Currency describes how to display amounts.
type Currency struct {
	Code   string `json:"code"`   // ISO code, e.g. NGN
	Symbol string `json:"symbol"` // e.g. ₦
	Digits int    `json:"digits"` // minor unit digits, e.g. 2
}

// Format renders minor units as e.g. "12,345.50" (no symbol).
func (c Currency) Format(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	digits := c.Digits
	div := int64(1)
	for range digits {
		div *= 10
	}
	whole := strconv.FormatInt(minor/div, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if digits > 0 {
		frac := strconv.FormatInt(minor%div, 10)
		b.WriteByte('.')
		b.WriteString(strings.Repeat("0", digits-len(frac)) + frac)
	}
	return b.String()
}

// FormatSymbol renders with the currency symbol, e.g. "₦12,345.50".
func (c Currency) FormatSymbol(minor int64) string {
	s := c.Format(minor)
	if strings.HasPrefix(s, "-") {
		return "-" + c.Symbol + s[1:]
	}
	return c.Symbol + s
}

// FormatQty renders milli-units compactly: 2000 -> "2", 1250 -> "1.25".
func FormatQty(qtyMilli int64) string {
	neg := qtyMilli < 0
	if neg {
		qtyMilli = -qtyMilli
	}
	s := strconv.FormatInt(qtyMilli/QtyScale, 10)
	if frac := qtyMilli % QtyScale; frac != 0 {
		f := strconv.FormatInt(frac, 10)
		f = strings.Repeat("0", 3-len(f)) + f
		s += "." + strings.TrimRight(f, "0")
	}
	if neg {
		s = "-" + s
	}
	return s
}
