package printing

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

var ngn = money.Currency{Code: "NGN", Symbol: "₦", Digits: 2}

func sale(lines int) *protocol.Sale {
	s := &protocol.Sale{ReceiptNo: "T01-000042", UserName: "Ada", CreatedAt: 1762600000000}
	for i := range lines {
		l := protocol.SaleLine{ProductName: "Things Fall Apart", QtyMilli: 2000, UnitPriceMinor: 350000, TotalMinor: 700000}
		if i%3 == 1 {
			l.ProductName = "Half of a Yellow Sun — Chimamanda Ngozi Adichie (Anniversary Hardcover Edition)"
			l.QtyMilli, l.TotalMinor = 1000, 350000
		}
		s.Lines = append(s.Lines, l)
		s.SubtotalMinor += l.TotalMinor
	}
	s.TotalMinor = s.SubtotalMinor
	s.Payments = []protocol.SalePayment{
		{MethodName: "Bank Transfer", Reference: "TRF8812", AmountMinor: s.TotalMinor - 100000},
		{MethodName: "Cash", AmountMinor: 200000},
	}
	s.PaidMinor = s.TotalMinor + 100000
	s.ChangeMinor = 100000
	return s
}

var settings = map[string]string{
	"store_name": "Zebi Books & Stationery", "store_address": "12 Allen Avenue, Ikeja, Lagos",
	"store_phone": "0803 000 0000", "receipt_footer": "Thank you for your patronage!\nGoods sold in good condition are not returnable after 7 days.",
}

func TestLayoutFitsColumns(t *testing.T) {
	for _, p := range []Paper{Paper80, Paper58} {
		for _, b := range Layout(FromSale(sale(5), settings, ngn, "Till 1", false), p.Cols) {
			if b.Kind != KText {
				continue
			}
			w := p.Cols
			if b.Big {
				w /= 2
			}
			if n := runeLen(b.Text); n > w {
				t.Errorf("%s: line exceeds %d cols (%d): %q", p.Name, w, n, b.Text)
			}
		}
	}
}

func TestTotalsRightAligned(t *testing.T) {
	lines := twoCol("A very long label that will need to wrap onto another line", "1,234.00", 20)
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "1,234.00") || runeLen(last) != 20 {
		t.Fatalf("bad last line %q", last)
	}
}

// Height must grow with content: short receipts are short, long ones long.
func TestDynamicHeight(t *testing.T) {
	short, err := Render(Layout(FromSale(sale(1), settings, ngn, "Till 1", false), Paper80.Cols), Paper80)
	if err != nil {
		t.Fatal(err)
	}
	long, _ := Render(Layout(FromSale(sale(60), settings, ngn, "Till 1", false), Paper80.Cols), Paper80)
	if short.Rect.Dx() != 576 || long.Rect.Dx() != 576 {
		t.Fatalf("width %d/%d, want 576", short.Rect.Dx(), long.Rect.Dx())
	}
	if long.Rect.Dy() <= short.Rect.Dy()*3 {
		t.Fatalf("long receipt (%d) not much taller than short (%d)", long.Rect.Dy(), short.Rect.Dy())
	}
	t.Logf("1 line: %d dots (%.0fmm); 60 lines: %d dots (%.0fmm)",
		short.Rect.Dy(), float64(short.Rect.Dy())/8, long.Rect.Dy(), float64(long.Rect.Dy())/8)

	// Every pixel is pure black or white (thermal heads are 1-bit).
	for _, v := range short.Pix {
		if v != 0 && v != 255 {
			t.Fatal("non-binary pixel")
		}
	}
}

func TestEncodeRaster(t *testing.T) {
	img, _ := Render(Layout(FromSale(sale(3), settings, ngn, "Till 1", false), Paper80.Cols), Paper80)
	job := EncodeRaster(img, JobOptions{Cut: true, FeedLines: 4, OpenDrawer: true})

	if !bytes.HasPrefix(job, []byte{0x1B, 0x40, 0x1B, 0x70}) {
		t.Fatal("job must start with init then drawer kick")
	}
	if !bytes.HasSuffix(job, []byte{0x1D, 0x56, 0x42, 96}) {
		t.Fatalf("job must end with feed+partial cut, got % x", job[len(job)-4:])
	}
	// Count raster rows across all GS v 0 bands: must equal image height.
	rows, i := 0, 0
	for {
		j := bytes.Index(job[i:], []byte{0x1D, 0x76, 0x30, 0x00})
		if j < 0 {
			break
		}
		p := i + j + 4
		x := int(job[p]) | int(job[p+1])<<8
		y := int(job[p+2]) | int(job[p+3])<<8
		if x != 72 {
			t.Fatalf("band width %d bytes, want 72", x)
		}
		rows += y
		i = p + 4 + x*y
	}
	if rows != img.Rect.Dy() {
		t.Fatalf("raster rows %d != image height %d", rows, img.Rect.Dy())
	}
}

func TestEncodeTextASCII(t *testing.T) {
	r := FromSale(sale(2), settings, ngn, "Till 1", false)
	job := EncodeText(Layout(r, 48), 48, DefaultJob)
	for _, c := range job {
		if c >= 0x80 {
			t.Fatalf("non-ASCII byte 0x%x in text job", c)
		}
	}
	if !bytes.Contains(job, []byte("TOTAL")) || !bytes.Contains(job, []byte("N")) {
		t.Fatal("text job missing total")
	}
}

func TestWidthRuler(t *testing.T) {
	for _, cols := range []int{32, 42, 48} {
		r := widthRuler(cols)
		if len(r) != cols || r[0] != '|' || r[cols-1] != '|' {
			t.Fatalf("ruler(%d) = %q", cols, r)
		}
		for _, l := range Layout(TestReceipt("S", ngn, Paper{Name: "x", Cols: cols}), cols) {
			_ = l
		}
	}
}

// Writes previews for eyeballing when RECEIPT_PREVIEW_DIR is set.
func TestWritePreviews(t *testing.T) {
	dir := os.Getenv("RECEIPT_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set RECEIPT_PREVIEW_DIR to write PNG previews")
	}
	write := func(name string, r Receipt) {
		img, err := Render(Layout(r, Paper80.Cols), Paper80)
		if err != nil {
			t.Fatal(err)
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, img)
		f.Close()
	}
	taxed := sale(2)
	taxed.TaxMode, taxed.TaxMinor, taxed.CustomerName, taxed.DiscountMinor = protocol.TaxInclusive, 97674, "Chidi Okeke", 50000
	taxed.Lines[0].OriginalPriceMinor = 400000
	st := map[string]string{"tax_name": "VAT", "tax_mode": "inclusive"}
	for k, v := range settings {
		st[k] = v
	}
	write("receipt-taxed.png", FromSale(taxed, st, ngn, "Till 1", false))
	write("test-page.png", TestReceipt("Kalu Books", ngn, Paper80))
	write("refund.png", FromReturn(&protocol.Return{ReturnNo: "T02-R00001", SaleReceiptNo: "T01-000042", UserName: "Musa", CreatedAt: 1762600000000,
		CustomerName: "Chidi Okeke", Reason: "Damaged spine", TotalMinor: 400000, TaxMinor: 27907,
		Lines:   []protocol.ReturnLine{{ProductName: "Things Fall Apart", QtyMilli: 1000, TotalMinor: 400000, Restock: false}},
		Refunds: []protocol.SalePayment{{MethodName: "Cash", AmountMinor: 400000}}}, st, ngn, "Till 2", false))
	write("z-report.png", ShiftReport(&protocol.Shift{Status: "closed", OpenedAt: 1762590000000, OpenedByName: "Kemi", ClosedAt: 1762620000000, ClosedByName: "Kemi",
		Note: "Short by 500", Summary: &protocol.ShiftSummary{SalesCount: 3, ItemsMilli: 4000, GrossMinor: 1800000, DiscountMinor: 100000, TaxMinor: 89564,
			NetSalesMinor: 1767500, ReturnsCount: 1, ReturnsMinor: 483750, OpeningFloat: 1000000, CashSales: 1283750, CashRefunds: 483750, PaidIn: 50000,
			PaidOut: 120000, ExpectedCash: 1730000, CountedCash: 1680000, Variance: -50000, Closed: true,
			ByMethod: []protocol.MethodTotal{{MethodName: "Cash", Kind: "cash", SalesMinor: 1283750, RefundsMinor: 483750, NetMinor: 800000},
				{MethodName: "On account", Kind: "credit", SalesMinor: 483750, NetMinor: 483750}}}}, st, ngn, "Till 1"))
	for name, n := range map[string]int{"receipt-short.png": 1, "receipt-long.png": 12} {
		img, err := Render(Layout(FromSale(sale(n), settings, ngn, "Till 1", false), Paper80.Cols), Paper80)
		if err != nil {
			t.Fatal(err)
		}
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, img)
		f.Close()
	}
}

func TestConcurrentRender(t *testing.T) {
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Render(Layout(FromSale(sale(4), settings, ngn, "Till 1", false), Paper80.Cols), Paper80); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
