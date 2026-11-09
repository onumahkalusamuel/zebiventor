package documents

import (
	"bytes"
	_ "embed"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// DejaVu Sans Condensed: wide Unicode coverage (₦, accents) and compact
// enough for tables on A4.
var (
	//go:embed fonts/DejaVuSansCondensed.ttf
	fontRegular []byte
	//go:embed fonts/DejaVuSansCondensed-Bold.ttf
	fontBold []byte
)

const (
	pageW, margin = 210.0, 14.0
	contentW      = pageW - 2*margin
)

// PDF renders the document as an A4 PDF (for email attachments).
func (d Doc) PDF() ([]byte, error) {
	if d.Printed == "" {
		d.Printed = time.Now().Format("2 Jan 2006 15:04")
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(strings.TrimSpace(d.Title+" "+d.Number), true)
	pdf.SetAuthor(d.Store.Name, true)
	pdf.SetCreator("Zebiventor", true)
	pdf.AddUTF8FontFromBytes("dv", "", fontRegular)
	pdf.AddUTF8FontFromBytes("dv", "B", fontBold)
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, 16)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-11)
		pdf.SetFont("dv", "", 7.5)
		pdf.SetTextColor(110, 110, 110)
		left := d.Footer
		if left == "" {
			left = strings.TrimSpace(d.Title + " " + d.Number)
		}
		pdf.CellFormat(contentW/2, 4, left, "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 4, "Printed "+d.Printed+" · page "+itoa(pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	ink := func() { pdf.SetTextColor(28, 28, 26) }
	grey := func() { pdf.SetTextColor(90, 90, 90) }

	// Letterhead and title.
	top := pdf.GetY()
	ink()
	pdf.SetFont("dv", "B", 15)
	pdf.MultiCell(contentW*0.58, 6.5, d.Store.Name, "", "L", false)
	pdf.SetFont("dv", "", 9)
	grey()
	for _, l := range d.Store.Lines {
		pdf.MultiCell(contentW*0.58, 4.3, l, "", "L", false)
	}
	leftBottom := pdf.GetY()
	pdf.SetXY(margin+contentW*0.58, top)
	ink()
	pdf.SetFont("dv", "B", 17)
	pdf.CellFormat(contentW*0.42, 7.5, d.Title, "", 2, "R", false, 0, "")
	if d.Number != "" {
		pdf.SetFont("dv", "B", 11)
		pdf.SetX(margin + contentW*0.58)
		pdf.CellFormat(contentW*0.42, 5.5, d.Number, "", 2, "R", false, 0, "")
	}
	y := max(leftBottom, pdf.GetY()) + 2
	pdf.SetDrawColor(28, 28, 26)
	pdf.SetLineWidth(0.6)
	pdf.Line(margin, y, pageW-margin, y)
	pdf.SetLineWidth(0.2)

	// Party (left) and details (right).
	y += 5
	pdf.SetXY(margin, y)
	if len(d.Party) > 0 {
		grey()
		pdf.SetFont("dv", "B", 7.5)
		pdf.CellFormat(contentW*0.55, 4, strings.ToUpper(d.PartyHead), "", 2, "L", false, 0, "")
		ink()
		pdf.SetFont("dv", "B", 11)
		pdf.MultiCell(contentW*0.55, 5.2, d.Party[0], "", "L", false)
		pdf.SetFont("dv", "", 9)
		grey()
		for _, l := range d.Party[1:] {
			pdf.MultiCell(contentW*0.55, 4.3, l, "", "L", false)
		}
	}
	partyBottom := pdf.GetY()
	pdf.SetY(y)
	for _, m := range d.Meta {
		pdf.SetX(margin + contentW*0.58)
		grey()
		pdf.SetFont("dv", "B", 7.5)
		pdf.CellFormat(contentW*0.17, 5, strings.ToUpper(m.Label), "", 0, "L", false, 0, "")
		ink()
		pdf.SetFont("dv", "", 9.5)
		pdf.CellFormat(contentW*0.25, 5, m.Value, "", 2, "R", false, 0, "")
	}
	pdf.SetY(max(partyBottom, pdf.GetY()) + 6)

	// The table.
	if len(d.Columns) > 0 {
		d.table(pdf)
	}

	// Totals.
	if len(d.Totals) > 0 {
		pdf.Ln(3)
		x := margin + contentW*0.5
		for i, t := range d.Totals {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			pdf.SetX(x)
			ink()
			if i == 0 {
				pdf.SetFont("dv", "B", 11.5)
				pdf.SetLineWidth(0.4)
				pdf.Line(x, pdf.GetY(), pageW-margin, pdf.GetY())
				pdf.SetLineWidth(0.2)
			} else {
				pdf.SetFont("dv", "", 9.5)
			}
			pdf.CellFormat(contentW*0.3, 6.5, t.Label, "", 0, "L", false, 0, "")
			pdf.CellFormat(contentW*0.2, 6.5, t.Value, "", 1, "R", false, 0, "")
		}
	}

	// Notes.
	if strings.TrimSpace(d.Notes) != "" {
		pdf.Ln(5)
		pdf.SetFillColor(245, 245, 243)
		pdf.SetFont("dv", "", 9)
		ink()
		pdf.MultiCell(contentW, 4.6, d.Notes, "", "L", true)
	}

	// Signature boxes.
	if len(d.Signoffs) > 0 {
		pdf.Ln(16)
		if pdf.GetY() > 255 {
			pdf.AddPage()
			pdf.Ln(10)
		}
		w := (contentW - float64(len(d.Signoffs)-1)*8) / float64(len(d.Signoffs))
		y := pdf.GetY()
		for i, s := range d.Signoffs {
			x := margin + float64(i)*(w+8)
			pdf.Line(x, y, x+w, y)
			pdf.SetXY(x, y+1)
			grey()
			pdf.SetFont("dv", "", 8.5)
			pdf.CellFormat(w, 4.5, s, "", 2, "L", false, 0, "")
			pdf.SetX(x)
			pdf.SetFont("dv", "", 7.5)
			pdf.CellFormat(w, 4.5, "Name, signature, date", "", 0, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// table draws the line items, wrapping long text and repeating the header
// on each new page.
func (d Doc) table(pdf *fpdf.Fpdf) {
	const lh, pad = 4.4, 1.6
	n := len(d.Columns)
	pdf.SetFont("dv", "", 9)
	// Natural widths, then fit the page: right-aligned (numbers) keep
	// theirs, text columns share what's left.
	widths := make([]float64, n)
	for i, c := range d.Columns {
		pdf.SetFont("dv", "B", 7.5)
		widths[i] = pdf.GetStringWidth(strings.ToUpper(c.Title)) + 2*pad
		pdf.SetFont("dv", "", 9)
		for _, r := range d.Rows {
			if i < len(r) {
				widths[i] = max(widths[i], pdf.GetStringWidth(r[i])+2*pad)
			}
		}
	}
	// Every column gets its natural width (plus slack so numbers never
	// wrap); the widest text column takes what's left and wraps.
	wide, wideW := -1, 0.0
	for i, c := range d.Columns {
		widths[i] += 1
		if !c.Right && widths[i] > wideW {
			wide, wideW = i, widths[i]
		}
	}
	if wide >= 0 {
		var others float64
		for i, w := range widths {
			if i != wide {
				others += w
			}
		}
		widths[wide] = max(contentW-others, 30)
	}
	var total float64
	for _, w := range widths {
		total += w
	}
	if total > contentW { // still too wide: squeeze the text columns
		var text float64
		for i, c := range d.Columns {
			if !c.Right {
				text += widths[i]
			}
		}
		for i, c := range d.Columns {
			if !c.Right {
				widths[i] -= (total - contentW) * widths[i] / text
			}
		}
	}
	align := func(c Column) string {
		if c.Right {
			return "R"
		}
		return "L"
	}
	header := func() {
		pdf.SetFont("dv", "B", 7.5)
		pdf.SetTextColor(85, 85, 85)
		y, x := pdf.GetY(), float64(margin)
		for i, c := range d.Columns {
			pdf.SetXY(x+pad, y)
			pdf.CellFormat(widths[i]-2*pad, 6, strings.ToUpper(c.Title), "", 0, align(c), false, 0, "")
			x += widths[i]
		}
		pdf.SetXY(margin, y+6)
		pdf.SetLineWidth(0.4)
		pdf.Line(margin, pdf.GetY(), pageW-margin, pdf.GetY())
		pdf.SetLineWidth(0.2)
		pdf.SetFont("dv", "", 9)
		pdf.SetTextColor(28, 28, 26)
	}
	header()
	pdf.SetDrawColor(215, 215, 215)
	for _, r := range d.Rows {
		cells := make([][]string, n)
		lines := 1
		for i := range d.Columns {
			txt := ""
			if i < len(r) {
				txt = r[i]
			}
			cells[i] = wrap(pdf, txt, widths[i]-2*pad)
			if d.Columns[i].Right {
				cells[i] = []string{txt} // numbers never wrap
			}
			lines = max(lines, len(cells[i]))
		}
		h := float64(lines)*lh + 2*pad
		if pdf.GetY()+h > 297-18 {
			pdf.AddPage()
			header()
		}
		y := pdf.GetY()
		x := margin
		for i, c := range d.Columns {
			for j, line := range cells[i] {
				pdf.SetXY(x+pad, y+pad+float64(j)*lh)
				pdf.CellFormat(widths[i]-2*pad, lh, line, "", 0, align(c), false, 0, "")
			}
			x += widths[i]
		}
		pdf.SetY(y + h)
		pdf.Line(margin, y+h, pageW-margin, y+h)
	}
	pdf.SetDrawColor(28, 28, 26)
}

// wrap splits text into lines that fit width, breaking at spaces (or
// within a word longer than a line).
func wrap(pdf *fpdf.Fpdf, text string, width float64) []string {
	if pdf.GetStringWidth(text) <= width {
		return []string{text}
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		try := strings.TrimSpace(line + " " + word)
		if pdf.GetStringWidth(try) <= width {
			line = try
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		line = ""
		for _, r := range word { // a word too long for the column
			if pdf.GetStringWidth(line+string(r)) > width && line != "" {
				lines = append(lines, line)
				line = ""
			}
			line += string(r)
		}
	}
	if line != "" || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}
