package terminal_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

// Phase 2 end-to-end: shifts, tax, approvals, customers on account,
// cross-till and offline returns, cash movements and the Z report.
func TestPOSOperations(t *testing.T) {
	ctx := context.Background()
	h, err := hub.Open(ctx, hub.Config{DataDir: t.TempDir(), Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(h.Setup(ctx, hub.SetupInput{StoreName: "Shop", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}))
	must(h.SaveSettings(ctx, hub.SystemSession, map[string]string{"tax_mode": protocol.TaxInclusive, "tax_name": "VAT"}))
	_, err = h.SaveUser(ctx, hub.SystemSession, protocol.UserInput{Name: "Kemi", Username: "kemi", Role: protocol.RoleCashier, Pin: "1234", Active: true})
	must(err)
	_, err = h.SaveUser(ctx, hub.SystemSession, protocol.UserInput{Name: "Musa", Username: "musa", Role: protocol.RoleManager, Pin: "9999", Active: true})
	must(err)
	_, err = h.SaveProduct(ctx, hub.SystemSession, protocol.ProductInput{
		Product:  protocol.Product{Name: "Things Fall Apart", TrackStock: true, TaxRateBP: 750},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 450000}, Barcodes: []string{"BOOK"}, OpeningQtyMilli: 40_000}},
	})
	must(err)

	sw := &switchable{h: h.Handler()}
	srv := httptest.NewServer(sw)
	defer srv.Close()

	pair := func(name string) *terminal.Terminal {
		code, err := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
		must(err)
		tm, err := terminal.Open(ctx, terminal.Config{DataDir: t.TempDir(), Logger: quiet})
		must(err)
		t.Cleanup(func() { tm.Close() })
		_, err = tm.Pair(ctx, srv.URL, code.Code, name)
		must(err)
		tm.StartSync()
		waitFor(t, name+" initial sync", 5*time.Second, func() bool {
			it, _ := tm.LookupCode(ctx, "BOOK")
			return it != nil && tm.HasUsers(ctx) && tm.Settings(ctx)["tax_mode"] == protocol.TaxInclusive
		})
		return tm
	}
	t1, t2 := pair("Till 1"), pair("Till 2")

	kemi, err := t1.Login(ctx, "kemi", "1234")
	must(err)
	musa, err := t1.Login(ctx, "musa", "9999")
	must(err)
	book, _ := t1.LookupCode(ctx, "BOOK")
	methods, _ := t1.PaymentMethods(ctx)
	byKind := map[string]protocol.PaymentMethod{}
	for _, m := range methods {
		byKind[m.Kind] = m
	}
	cash := byKind[protocol.PayCash]

	// Selling needs an open shift.
	line := func(q int64) []terminal.CartLine {
		return []terminal.CartLine{{VariantID: book.VariantID, QtyMilli: q}}
	}
	if _, err := t1.RecordSale(ctx, kemi, nil, terminal.SaleInput{Lines: line(1000), Payments: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 450000}}}); err == nil {
		t.Fatal("sale without shift accepted")
	}
	shift, err := t1.OpenShift(ctx, kemi, 1_000_000)
	must(err)
	if _, err := t1.OpenShift(ctx, kemi, 0); err == nil {
		t.Fatal("second shift opened")
	}

	// A cashier's discount needs a manager.
	saleIn := terminal.SaleInput{Lines: line(2000), DiscountMinor: 100000, Payments: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 1_000_000}}}
	if _, err := t1.RecordSale(ctx, kemi, nil, saleIn); !errors.Is(err, terminal.ErrApprovalRequired) {
		t.Fatalf("expected approval error, got %v", err)
	}
	saleA, err := t1.RecordSale(ctx, kemi, &musa, saleIn)
	must(err)
	// Inclusive VAT 7.5% on (9,000 - 1,000) = 8,000: tax = 8000 - 8000/1.075 = 558.14
	if saleA.ApprovedBy != musa.ID || saleA.TotalMinor != 800000 || saleA.TaxMinor != 55814 || saleA.ChangeMinor != 200000 {
		t.Fatalf("sale A: approved=%q total=%d tax=%d change=%d", saleA.ApprovedBy, saleA.TotalMinor, saleA.TaxMinor, saleA.ChangeMinor)
	}

	// Exclusive tax adds on top.
	must(h.SaveSettings(ctx, hub.SystemSession, map[string]string{"tax_mode": protocol.TaxExclusive}))
	waitFor(t, "tax mode change", 5*time.Second, func() bool { return t1.Settings(ctx)["tax_mode"] == protocol.TaxExclusive })
	saleB, err := t1.RecordSale(ctx, kemi, nil, terminal.SaleInput{Lines: line(1000), Payments: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 483750}}})
	must(err)
	if saleB.TotalMinor != 483750 || saleB.TaxMinor != 33750 {
		t.Fatalf("sale B total=%d tax=%d", saleB.TotalMinor, saleB.TaxMinor)
	}

	// Customer created on Till 2 reaches Till 1; credit sale goes on account.
	cust, err := t2.SaveCustomer(ctx, protocol.Customer{Name: "Chidi Okeke", Phone: "08030000001"})
	must(err)
	waitFor(t, "customer on till 1", 5*time.Second, func() bool {
		cs, _ := t1.SearchCustomers(ctx, "chidi", 5)
		return len(cs) == 1
	})
	credit := byKind[protocol.PayCredit]
	if credit.ID == "" {
		id, err := h.SavePaymentMethod(ctx, hub.SystemSession, protocol.PaymentMethod{Name: "On account", Kind: protocol.PayCredit, Active: true, SortOrder: 9})
		must(err)
		credit.ID = id
		waitFor(t, "credit method", 5*time.Second, func() bool {
			ms, _ := t1.PaymentMethods(ctx)
			return len(ms) == len(methods)+1
		})
	}
	if _, err := t1.RecordSale(ctx, kemi, nil, terminal.SaleInput{Lines: line(1000), Payments: []terminal.PaymentInput{{MethodID: credit.ID, AmountMinor: 483750}}}); err == nil {
		t.Fatal("credit sale without customer accepted")
	}
	saleC, err := t1.RecordSale(ctx, kemi, nil, terminal.SaleInput{Lines: line(1000), CustomerID: cust.ID, Payments: []terminal.PaymentInput{{MethodID: credit.ID, AmountMinor: 483750}}})
	must(err)
	waitFor(t, "balance on till 2", 5*time.Second, func() bool {
		cs, _ := t2.SearchCustomers(ctx, "chidi", 5)
		return len(cs) == 1 && cs[0].BalanceMinor == 483750
	})

	// Cross-till return: Till 2 refunds part of Till 1's sale A via the hub.
	waitFor(t, "till 1 synced", 5*time.Second, func() bool { return t1.Status().Pending == 0 })
	shift2, err := t2.OpenShift(ctx, musa, 500_000)
	must(err)
	rs, err := t2.LookupForReturn(ctx, saleA.ReceiptNo)
	must(err)
	if rs.Source != "hub" || rs.Lines[0].ReturnableMilli != 2000 {
		t.Fatalf("lookup: source=%s returnable=%d", rs.Source, rs.Lines[0].ReturnableMilli)
	}
	retIn := terminal.ReturnInput{ReceiptNo: saleA.ReceiptNo, Reason: "Duplicate copy",
		Lines:   []terminal.ReturnLineInput{{SaleLineID: rs.Lines[0].ID, QtyMilli: 1000, Restock: true}},
		Refunds: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 400000}}}
	if _, err := t2.RecordReturn(ctx, kemi, nil, retIn); !errors.Is(err, terminal.ErrApprovalRequired) {
		t.Fatalf("cashier refund without approval: %v", err)
	}
	ret1, err := t2.RecordReturn(ctx, musa, nil, retIn)
	must(err)
	if ret1.TotalMinor != 400000 || ret1.TaxMinor != 27907 {
		t.Fatalf("return total=%d tax=%d", ret1.TotalMinor, ret1.TaxMinor)
	}
	retIn.Lines[0].QtyMilli = 2000
	if _, err := t2.RecordReturn(ctx, musa, nil, retIn); err == nil {
		t.Fatal("over-return accepted")
	}

	// Credit sale returned to the account (damaged, not restocked).
	waitFor(t, "till 2 synced", 5*time.Second, func() bool { return t2.Status().Pending == 0 })
	rsC, err := t2.LookupForReturn(ctx, saleC.ReceiptNo)
	must(err)
	_, err = t2.RecordReturn(ctx, musa, nil, terminal.ReturnInput{ReceiptNo: saleC.ReceiptNo, Reason: "Damaged",
		Lines:   []terminal.ReturnLineInput{{SaleLineID: rsC.Lines[0].ID, QtyMilli: 1000, Restock: false}},
		Refunds: []terminal.PaymentInput{{MethodID: credit.ID, AmountMinor: 483750}}})
	must(err)
	waitFor(t, "balance cleared", 5*time.Second, func() bool {
		cs, _ := t1.SearchCustomers(ctx, "chidi", 5)
		return len(cs) == 1 && cs[0].BalanceMinor == 0
	})

	// Cash movements: paid-out needs approval.
	if _, err := t1.AddCashMovement(ctx, kemi, nil, protocol.CashPaidOut, 120000, "Fuel for generator"); !errors.Is(err, terminal.ErrApprovalRequired) {
		t.Fatalf("paid out without approval: %v", err)
	}
	_, err = t1.AddCashMovement(ctx, kemi, &musa, protocol.CashPaidOut, 120000, "Fuel for generator")
	must(err)
	_, err = t1.AddCashMovement(ctx, kemi, nil, protocol.CashPaidIn, 50000, "Change from bank")
	must(err)

	// Hub goes offline: Till 1 can still refund its own sale B.
	sw.down.Store(true)
	rsB, err := t1.LookupForReturn(ctx, saleB.ReceiptNo)
	must(err)
	if rsB.Source != "local" || !rsB.Partial {
		t.Fatalf("offline lookup source=%s partial=%v", rsB.Source, rsB.Partial)
	}
	_, err = t1.RecordReturn(ctx, musa, nil, terminal.ReturnInput{ReceiptNo: saleB.ReceiptNo, Reason: "Changed mind",
		Lines:   []terminal.ReturnLineInput{{SaleLineID: rsB.Lines[0].ID, QtyMilli: 1000, Restock: true}},
		Refunds: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 483750}}})
	must(err)
	// Another till's receipt can't be looked up offline.
	if _, err := t1.LookupForReturn(ctx, ret1.ReturnNo); err == nil {
		t.Fatal("offline lookup of unknown receipt succeeded")
	}

	// Hold / recall.
	id, err := t1.HoldSale(ctx, kemi, terminal.HeldSale{Label: "Lady in blue", Payload: `{"cart":[]}`, ItemCount: 2, TotalMinor: 900000})
	must(err)
	held, _ := t1.HeldSales(ctx)
	if len(held) != 1 {
		t.Fatalf("held=%d", len(held))
	}
	got, err := t1.TakeHeldSale(ctx, id)
	must(err)
	if got.Payload != `{"cart":[]}` {
		t.Fatal("held payload mismatch")
	}
	if _, err := t1.TakeHeldSale(ctx, id); err == nil {
		t.Fatal("held sale recalled twice")
	}

	// Close Till 1's shift (offline) with a short drawer.
	// Expected = float 10,000 + sale A 8,000 + sale B 4,837.50 - refund B 4,837.50 + in 500 - out 1,200
	const expected = 1_000_000 + 800_000 + 483_750 - 483_750 + 50_000 - 120_000
	cur, _ := t1.CurrentShift(ctx)
	if cur.Summary.ExpectedCash != expected {
		t.Fatalf("X report expected cash %d, want %d (%+v)", cur.Summary.ExpectedCash, expected, cur.Summary)
	}
	closed, err := t1.CloseShift(ctx, kemi, expected-50000, "Short by 500")
	must(err)
	if closed.Summary.Variance != -50000 || closed.Summary.SalesCount != 3 || closed.Summary.ReturnsCount != 1 || closed.ID != shift.ID {
		t.Fatalf("Z report: %+v", closed.Summary)
	}
	if s, _ := t1.CurrentShift(ctx); s != nil {
		t.Fatal("shift still open after close")
	}

	// Back online: everything converges at the hub.
	sw.down.Store(false)
	t1.Nudge()
	waitFor(t, "till 1 drained", 20*time.Second, func() bool { return t1.Status().Pending == 0 })
	r := h.DB().R()
	var level, variance, returns, movements int64
	var status string
	r.QueryRow("SELECT qty_milli FROM stock_levels").Scan(&level)
	r.QueryRow("SELECT status, variance_minor FROM shifts WHERE id=?", shift.ID).Scan(&status, &variance)
	r.QueryRow("SELECT count(*) FROM returns").Scan(&returns)
	r.QueryRow("SELECT count(*) FROM cash_movements").Scan(&movements)
	// 40 - 2 (A) - 1 (B) - 1 (C) + 1 (A return) + 1 (B return) = 38; C return not restocked.
	if level != 38_000 || status != "closed" || variance != -50000 || returns != 3 || movements != 2 {
		t.Fatalf("hub: level=%d shift=%s variance=%d returns=%d movements=%d", level, status, variance, returns, movements)
	}
	var shiftOpen int
	r.QueryRow("SELECT count(*) FROM shifts WHERE id=? AND status='open'", shift2.ID).Scan(&shiftOpen)
	if shiftOpen != 1 {
		t.Fatal("till 2 shift not recorded at hub")
	}
	// Stock view on Till 2 agrees.
	waitFor(t, "stock on till 2", 5*time.Second, func() bool {
		it, _ := t2.LookupCode(ctx, "BOOK")
		return it.StockMilli == 38_000
	})
}
