// Package printing produces thermal receipts.
//
// Pipeline: a Receipt (business data) is laid out into Blocks for a given
// paper width (in character columns), then either
//   - rasterised to a 1-bit image exactly as wide as the print head and as
//     tall as the content (default: crisp, supports ₦ and any Unicode, logo
//     and QR), and encoded as ESC/POS GS v 0 raster bands; or
//   - encoded as plain ESC/POS text (fastest, ASCII only).
//
// Receipt height is never fixed: the printer feeds exactly as many dot rows
// as the content needs and then cuts, so a one-item receipt is short and a
// hundred-item receipt is long.
package printing

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Paper describes a roll width.
type Paper struct {
	Name string `json:"name"`
	Dots int    `json:"dots"` // printable width in dots (203 dpi heads)
	Cols int    `json:"cols"` // characters per line in normal size
}

var (
	Paper80 = Paper{Name: "80mm", Dots: 576, Cols: 48}
	Paper58 = Paper{Name: "58mm", Dots: 384, Cols: 32}
)

// PaperByName returns the paper for "58mm"/"80mm" (default 80mm).
func PaperByName(name string) Paper {
	if name == Paper58.Name {
		return Paper58
	}
	return Paper80
}

// Align is horizontal alignment.
type Align int

const (
	Left Align = iota
	Center
	Right
)

// BlockKind distinguishes content blocks.
type BlockKind int

const (
	KText BlockKind = iota
	KRule
	KDoubleRule
	KSpace
	KQR
)

// Block is one laid-out line (or a QR code).
type Block struct {
	Kind BlockKind
	Text string // already padded to the line width for KText
	Bold bool
	Big  bool // double width and height
	QR   string
}

// Receipt is the printable content of a receipt.
type Receipt struct {
	StoreName   string
	HeaderLines []string
	Title       string
	Meta        [][2]string
	Items       []Item
	Totals      []Row
	Payments    []Row
	FooterLines []string
	QR          string
}

// Item is one sold line.
type Item struct {
	Name   string
	Detail string // "2 x 3,500.00"
	Total  string
}

// Row is a label/value line.
type Row struct {
	Label  string
	Value  string
	Strong bool
}

// FromSale builds a Receipt from a sale and the store settings.
func FromSale(s *protocol.Sale, settings map[string]string, cur money.Currency, terminalName string, reprint bool) Receipt {
	r := Receipt{StoreName: settings["store_name"]}
	for _, k := range []string{"store_address", "store_phone", "store_email"} {
		for _, line := range strings.Split(settings[k], "\n") {
			if line = strings.TrimSpace(line); line != "" {
				r.HeaderLines = append(r.HeaderLines, line)
			}
		}
	}
	if v := strings.TrimSpace(settings["tax_id"]); v != "" {
		r.HeaderLines = append(r.HeaderLines, "TIN: "+v)
	}
	if v := strings.TrimSpace(settings["receipt_header"]); v != "" {
		r.HeaderLines = append(r.HeaderLines, strings.Split(v, "\n")...)
	}
	r.Title = "SALES RECEIPT"
	if reprint {
		r.Title = "*** REPRINT ***"
	}
	r.Meta = [][2]string{
		{"Receipt", s.ReceiptNo},
		{"Date", time.UnixMilli(s.CreatedAt).Format("02 Jan 2006 15:04")},
		{"Cashier", s.UserName},
	}
	if terminalName != "" {
		r.Meta = append(r.Meta, [2]string{"Till", terminalName})
	}
	var count int64
	for _, l := range s.Lines {
		name := l.ProductName
		if l.VariantName != "" {
			name += " (" + l.VariantName + ")"
		}
		r.Items = append(r.Items, Item{
			Name:   name,
			Detail: fmt.Sprintf("%s x %s", money.FormatQty(l.QtyMilli), cur.Format(l.UnitPriceMinor)),
			Total:  cur.Format(l.TotalMinor),
		})
		count += l.QtyMilli
	}
	r.Totals = append(r.Totals, Row{Label: fmt.Sprintf("Subtotal (%s items)", money.FormatQty(count)), Value: cur.Format(s.SubtotalMinor)})
	if s.DiscountMinor != 0 {
		r.Totals = append(r.Totals, Row{Label: "Discount", Value: "-" + cur.Format(s.DiscountMinor)})
	}
	if s.TaxMinor != 0 {
		r.Totals = append(r.Totals, Row{Label: "Tax", Value: cur.Format(s.TaxMinor)})
	}
	r.Totals = append(r.Totals, Row{Label: "TOTAL", Value: cur.FormatSymbol(s.TotalMinor), Strong: true})
	for _, p := range s.Payments {
		label := p.MethodName
		if p.Reference != "" {
			label += " #" + p.Reference
		}
		r.Payments = append(r.Payments, Row{Label: label, Value: cur.Format(p.AmountMinor)})
	}
	if s.ChangeMinor > 0 {
		r.Payments = append(r.Payments, Row{Label: "Change", Value: cur.Format(s.ChangeMinor), Strong: true})
	}
	for _, line := range strings.Split(settings["receipt_footer"], "\n") {
		if line = strings.TrimSpace(line); line != "" {
			r.FooterLines = append(r.FooterLines, line)
		}
	}
	r.QR = s.ReceiptNo
	return r
}

// TestReceipt is a sample used for "print test page".
func TestReceipt(storeName string, cur money.Currency, paper Paper) Receipt {
	return Receipt{
		StoreName:   storeName,
		HeaderLines: []string{"Printer test page", paper.Name + " paper, " + fmt.Sprint(paper.Cols) + " columns"},
		Title:       "TEST PRINT",
		Meta:        [][2]string{{"Date", time.Now().Format("02 Jan 2006 15:04")}},
		Items: []Item{
			{Name: "Short item", Detail: "1 x " + cur.Format(150000), Total: cur.Format(150000)},
			{Name: "A deliberately long product name that must wrap across more than one line", Detail: "3 x " + cur.Format(99999), Total: cur.Format(299997)},
		},
		Totals:      []Row{{Label: "TOTAL", Value: cur.FormatSymbol(449997), Strong: true}},
		FooterLines: []string{"If this prints cleanly and cuts,", "the printer is ready."},
		QR:          "ZEBIVENTOR-TEST",
	}
}

// ---------- layout ----------

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func pad(s string, width int, a Align) string {
	n := runeLen(s)
	if n >= width {
		return s
	}
	switch a {
	case Right:
		return strings.Repeat(" ", width-n) + s
	case Center:
		left := (width - n) / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-n-left)
	default:
		return s + strings.Repeat(" ", width-n)
	}
}

// wrap breaks s into lines of at most width runes, preferring word boundaries.
func wrap(s string, width int) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			continue
		}
		line := ""
		for _, w := range words {
			for runeLen(w) > width { // hard-break very long words (e.g. URLs)
				if line != "" {
					out = append(out, line)
					line = ""
				}
				r := []rune(w)
				out = append(out, string(r[:width]))
				w = string(r[width:])
			}
			switch {
			case line == "":
				line = w
			case runeLen(line)+1+runeLen(w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// twoCol puts left and right on one line, wrapping left if needed so the
// right value always sits flush right on the last line.
func twoCol(left, right string, width int) []string {
	rw := runeLen(right)
	if rw >= width {
		return []string{pad(right, width, Right)}
	}
	trimmed := strings.TrimLeft(left, " ")
	indent := strings.Repeat(" ", min(len(left)-len(trimmed), width/4))
	lines := wrap(trimmed, width-len(indent))
	if len(lines) == 0 {
		lines = []string{""}
	}
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	last := lines[len(lines)-1]
	if runeLen(last)+1+rw > width {
		lines = append(lines, "")
		last = ""
	}
	for i := range lines[:len(lines)-1] {
		lines[i] = pad(lines[i], width, Left)
	}
	lines[len(lines)-1] = last + strings.Repeat(" ", width-runeLen(last)-rw) + right
	return lines
}

// Layout turns a receipt into blocks for the given column count.
func Layout(r Receipt, cols int) []Block {
	var b []Block
	text := func(s string, a Align, bold, big bool) {
		w := cols
		if big {
			w = cols / 2
		}
		for _, line := range wrap(s, w) {
			b = append(b, Block{Kind: KText, Text: pad(line, w, a), Bold: bold, Big: big})
		}
	}
	rows := func(left, right string, bold, big bool) {
		w := cols
		if big {
			w = cols / 2
		}
		for _, line := range twoCol(left, right, w) {
			b = append(b, Block{Kind: KText, Text: line, Bold: bold, Big: big})
		}
	}

	if r.StoreName != "" {
		text(r.StoreName, Center, true, true)
	}
	for _, h := range r.HeaderLines {
		text(h, Center, false, false)
	}
	b = append(b, Block{Kind: KSpace})
	if r.Title != "" {
		text(r.Title, Center, true, false)
	}
	b = append(b, Block{Kind: KRule})
	for _, m := range r.Meta {
		rows(m[0]+":", m[1], false, false)
	}
	b = append(b, Block{Kind: KRule})

	for _, it := range r.Items {
		text(it.Name, Left, true, false)
		rows("  "+it.Detail, it.Total, false, false)
	}
	b = append(b, Block{Kind: KRule})

	for _, t := range r.Totals {
		if t.Strong {
			b = append(b, Block{Kind: KDoubleRule})
			// Big if it fits on half-width columns, else bold normal size.
			if runeLen(t.Label)+1+runeLen(t.Value) <= cols/2 {
				rows(t.Label, t.Value, true, true)
			} else {
				rows(t.Label, t.Value, true, false)
			}
			b = append(b, Block{Kind: KDoubleRule})
			continue
		}
		rows(t.Label, t.Value, false, false)
	}
	if len(r.Payments) > 0 {
		b = append(b, Block{Kind: KSpace})
		for _, p := range r.Payments {
			rows(p.Label, p.Value, p.Strong, false)
		}
	}
	if len(r.FooterLines) > 0 {
		b = append(b, Block{Kind: KSpace})
		for _, f := range r.FooterLines {
			text(f, Center, false, false)
		}
	}
	if r.QR != "" {
		b = append(b, Block{Kind: KSpace}, Block{Kind: KQR, QR: r.QR})
		text(r.QR, Center, false, false)
	}
	return b
}
