package money

import "testing"

func TestLineTotal(t *testing.T) {
	cases := []struct{ q, p, want int64 }{
		{2000, 350000, 700000},
		{1250, 100001, 125001}, // 1.25 * 1000.01 = 1250.0125 -> 1250.01
		{333, 100, 33},         // 0.333 * 1.00 = 0.333 -> 0.33
		{-2000, 350000, -700000},
		{500, 1, 1}, // half rounds away from zero
	}
	for _, c := range cases {
		if got := LineTotal(c.q, c.p); got != c.want {
			t.Errorf("LineTotal(%d,%d)=%d want %d", c.q, c.p, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	ngn := Currency{Code: "NGN", Symbol: "₦", Digits: 2}
	cases := map[int64]string{0: "0.00", 5: "0.05", 123456789: "1,234,567.89", -150000: "-1,500.00", 100000: "1,000.00"}
	for in, want := range cases {
		if got := ngn.Format(in); got != want {
			t.Errorf("Format(%d)=%q want %q", in, got, want)
		}
	}
	if got := ngn.FormatSymbol(-150000); got != "-₦1,500.00" {
		t.Errorf("got %q", got)
	}
	for in, want := range map[int64]string{2000: "2", 1250: "1.25", 1005: "1.005", -500: "-0.5"} {
		if got := FormatQty(in); got != want {
			t.Errorf("FormatQty(%d)=%q want %q", in, got, want)
		}
	}
}
