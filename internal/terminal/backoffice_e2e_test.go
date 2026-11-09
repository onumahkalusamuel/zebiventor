package terminal_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

// Phase 3 end-to-end: purchasing and receiving (weighted average cost),
// counting while trading, transfers, payments on account (offline),
// approval rules, reports and CSV import.
func TestBackOffice(t *testing.T) {
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
	sys := hub.SystemSession
	must(h.Setup(ctx, hub.SetupInput{StoreName: "Shop", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}))
	_, err = h.SaveUser(ctx, sys, protocol.UserInput{Name: "Kemi", Username: "kemi", Role: protocol.RoleCashier, Pin: "1234", Active: true})
	must(err)
	pid, err := h.SaveProduct(ctx, sys, protocol.ProductInput{
		Product:  protocol.Product{Name: "Pen", TrackStock: true},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{SKU: "PEN", PriceMinor: 2000, CostMinor: 1000, ReorderLevelMilli: 12_000}, Barcodes: []string{"111"}, OpeningQtyMilli: 10_000}},
	})
	must(err)
	_, err = h.SaveProduct(ctx, sys, protocol.ProductInput{
		Product:  protocol.Product{Name: "Ruler", TrackStock: true},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{SKU: "RUL", PriceMinor: 500, CostMinor: 200}, OpeningQtyMilli: 4_000}},
	})
	must(err)
	pen := variantBySKU(t, h, "PEN")

	sw := &switchable{h: h.Handler()}
	srv := httptest.NewServer(sw)
	defer srv.Close()
	code, err := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
	must(err)
	tm, err := terminal.Open(ctx, terminal.Config{DataDir: t.TempDir(), Logger: quiet})
	must(err)
	defer tm.Close()
	_, err = tm.Pair(ctx, srv.URL, code.Code, "Front")
	must(err)
	tm.StartSync()
	waitFor(t, "initial sync", 5*time.Second, func() bool { it, _ := tm.LookupCode(ctx, "111"); return it != nil && tm.HasUsers(ctx) })
	pairing, _ := tm.Pairing(ctx)
	mainLoc := pairing.LocationID

	admin, err := tm.Login(ctx, "admin", "secret123")
	must(err)
	if !tm.HasHubSession() {
		t.Fatal("admin should have a hub session")
	}
	kemi, err := tm.Authenticate(ctx, "kemi", "1234")
	must(err)
	_, err = tm.OpenShift(ctx, kemi, 0)
	must(err)
	tillStock := func(sku string) int64 {
		it, _ := tm.LookupCode(ctx, sku)
		if it == nil {
			return -1
		}
		return it.StockMilli
	}
	hubStock := func(loc string) int64 {
		rep, err := h.StockReport(ctx, sys, hub.StockReportFilter{LocationID: loc, Query: "pen"})
		must(err)
		if len(rep.Rows) != 1 {
			t.Fatalf("stock report rows = %d", len(rep.Rows))
		}
		return rep.Rows[0].QtyMilli
	}

	// ---- Purchasing over the HTTP admin API, as the back-office screens do it.
	var sup struct{ ID string }
	must(tm.Admin(ctx, http.MethodPost, "/api/admin/suppliers", protocol.Supplier{Name: "Stationers Ltd"}, &sup))
	var po struct{ ID string }
	must(tm.Admin(ctx, http.MethodPost, "/api/admin/purchase-orders", protocol.PurchaseOrderInput{SupplierID: sup.ID, Submit: true,
		Lines: []protocol.PurchaseOrderLine{{VariantID: pen, QtyOrderedMilli: 20_000, UnitCostMinor: 1600}}}, &po))
	var order protocol.PurchaseOrder
	must(tm.Admin(ctx, http.MethodGet, "/api/admin/purchase-orders/"+po.ID, nil, &order))
	if order.Status != protocol.POOrdered || order.PONo != "PO-000001" || len(order.Lines) != 1 {
		t.Fatalf("order = %+v", order)
	}
	// A partial delivery, then the rest.
	_, err = h.ReceiveGoods(ctx, sys, protocol.GoodsReceiptInput{POID: po.ID, SupplierRef: "INV-1",
		Lines: []protocol.GoodsReceiptLine{{POLineID: order.Lines[0].ID, VariantID: pen, QtyMilli: 15_000, UnitCostMinor: 1600}}})
	must(err)
	got, err := h.GetPurchaseOrder(ctx, sys, po.ID)
	must(err)
	if got.Status != protocol.POPartial || got.Lines[0].QtyReceivedMilli != 15_000 {
		t.Fatalf("after partial delivery: %s %d", got.Status, got.Lines[0].QtyReceivedMilli)
	}
	_, err = h.ReceiveGoods(ctx, sys, protocol.GoodsReceiptInput{POID: po.ID,
		Lines: []protocol.GoodsReceiptLine{{POLineID: order.Lines[0].ID, VariantID: pen, QtyMilli: 5_000, UnitCostMinor: 1600, NewPriceMinor: 2200}}})
	must(err)
	got, _ = h.GetPurchaseOrder(ctx, sys, po.ID)
	if got.Status != protocol.POReceived || len(got.Receipts) != 2 {
		t.Fatalf("after full delivery: %s, %d receipts", got.Status, len(got.Receipts))
	}
	if err := h.SetPurchaseOrderStatus(ctx, sys, po.ID, protocol.POCancelled); err == nil {
		t.Fatal("a received order must not be cancellable")
	}
	// 10 @ 10.00 + 15 @ 16.00 = 25 @ 13.60; + 5 @ 16.00 = 30 @ 14.00.
	waitFor(t, "receipt reaches till", 5*time.Second, func() bool { return tillStock("111") == 30_000 })
	it, _ := tm.LookupCode(ctx, "111")
	if it.PriceMinor != 2200 {
		t.Fatalf("new selling price not applied at till: %d", it.PriceMinor)
	}
	prods, _ := tm.ListProducts(ctx, "pen", 10)
	if c := prods[0].Variants[0].CostMinor; c != 1400 {
		t.Fatalf("weighted average cost = %d, want 1400", c)
	}
	if r := prods[0].Variants[0].ReorderLevelMilli; r != 12_000 {
		t.Fatalf("reorder level at till = %d", r)
	}

	// ---- Sell 2, then count while trading: count 28, sell 1 more, post.
	sell := func(qty int64) *protocol.Sale {
		t.Helper()
		pms, _ := tm.PaymentMethods(ctx)
		s, err := tm.RecordSale(ctx, kemi, nil, terminal.SaleInput{Lines: []terminal.CartLine{{VariantID: pen, QtyMilli: qty}},
			Payments: []terminal.PaymentInput{{MethodID: pms[0].ID, AmountMinor: 2200 * qty / 1000}}})
		must(err)
		return s
	}
	sell(2000)
	waitFor(t, "sale reaches hub", 5*time.Second, func() bool { return hubStock(mainLoc) == 28_000 })
	countID, err := h.CreateStockCount(ctx, sys, hub.StockCountInput{})
	must(err)
	line, err := h.CountEntry(ctx, sys, countID, protocol.StockCountEntry{VariantID: pen, QtyMilli: 27_000, Mode: "set"})
	must(err)
	if line.ExpectedMilli != 28_000 {
		t.Fatalf("expected at count = %d", line.ExpectedMilli)
	}
	_, err = h.CountEntry(ctx, sys, countID, protocol.StockCountEntry{VariantID: pen, QtyMilli: 1_000, Mode: "add"}) // found one more
	must(err)
	time.Sleep(5 * time.Millisecond)
	sell(1000) // sold after it was counted
	waitFor(t, "sale reaches hub", 5*time.Second, func() bool { return hubStock(mainLoc) == 27_000 })
	must(h.PostStockCount(ctx, sys, countID, protocol.StockCountPost{ZeroUncounted: true}))
	// Counted 28 when the ledger said 28: no correction; the later sale stands.
	if s := hubStock(mainLoc); s != 27_000 {
		t.Fatalf("pen after count = %d, want 27000", s)
	}
	c, _ := h.GetStockCount(ctx, sys, countID)
	if c.Status != protocol.CountPosted || len(c.Lines) != 2 {
		t.Fatalf("count = %s with %d lines (ruler should have been zeroed)", c.Status, len(c.Lines))
	}
	rul, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Query: "ruler"})
	if rul.Rows[0].QtyMilli != 0 {
		t.Fatalf("uncounted ruler = %d, want 0", rul.Rows[0].QtyMilli)
	}

	// ---- Transfer 5 to a new warehouse.
	var wh struct{ ID string }
	must(tm.Admin(ctx, http.MethodPost, "/api/admin/locations", protocol.Location{Name: "Warehouse"}, &wh))
	_, err = h.TransferStock(ctx, sys, hub.TransferInput{FromLocationID: mainLoc, ToLocationID: wh.ID, ReceiveNow: true, Lines: []protocol.StockTransferLine{{VariantID: pen, QtyMilli: 5_000}}})
	must(err)
	if a, b := hubStock(mainLoc), hubStock(wh.ID); a != 22_000 || b != 5_000 {
		t.Fatalf("after transfer main=%d warehouse=%d", a, b)
	}
	waitFor(t, "transfer reaches till", 5*time.Second, func() bool { return tillStock("111") == 22_000 })
	low, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Only: "low"})
	if len(low.Rows) != 0 {
		t.Fatalf("22 on hand with reorder level 12 is not low")
	}

	// Ledger: newest first, running balance ends at the current level.
	moves, err := h.StockMovements(ctx, sys, pen, mainLoc, 0)
	must(err)
	if moves[0].BalanceMilli != 22_000 || moves[0].Reason != "transfer_out" || moves[len(moves)-1].Reason != "opening" ||
		moves[len(moves)-1].BalanceMilli != 10_000 {
		t.Fatalf("ledger: first %+v last %+v", moves[0], moves[len(moves)-1])
	}

	// ---- Payments on account, taken offline.
	cust, err := tm.SaveCustomer(ctx, protocol.Customer{Name: "Bola"})
	must(err)
	pms, _ := tm.PaymentMethods(ctx)
	var cash, credit string
	for _, m := range pms {
		switch m.Kind {
		case protocol.PayCash:
			cash = m.ID
		case protocol.PayCredit:
			credit = m.ID
		}
	}
	if credit == "" {
		_, err := h.SavePaymentMethod(ctx, sys, protocol.PaymentMethod{Name: "On account", Kind: protocol.PayCredit, Active: true})
		must(err)
		waitFor(t, "credit method", 5*time.Second, func() bool {
			pms, _ = tm.PaymentMethods(ctx)
			for _, m := range pms {
				if m.Kind == protocol.PayCredit {
					credit = m.ID
				}
			}
			return credit != ""
		})
	}
	_, err = tm.RecordSale(ctx, kemi, nil, terminal.SaleInput{CustomerID: cust.ID, Lines: []terminal.CartLine{{VariantID: pen, QtyMilli: 2000}},
		Payments: []terminal.PaymentInput{{MethodID: credit, AmountMinor: 4400}}})
	must(err)
	sw.down.Store(true)
	if _, err := tm.RecordAccountPayment(ctx, kemi, terminal.AccountPaymentInput{CustomerID: cust.ID, MethodID: credit, AmountMinor: 100}); err == nil {
		t.Fatal("paying an account with credit must fail")
	}
	pay, err := tm.RecordAccountPayment(ctx, kemi, terminal.AccountPaymentInput{CustomerID: cust.ID, MethodID: cash, AmountMinor: 1500})
	must(err)
	if pay.BalanceBefore != 4400 || !strings.HasSuffix(pay.PaymentNo, "-P00001") {
		t.Fatalf("payment = %+v", pay)
	}
	list, _ := tm.SearchCustomers(ctx, "bola", 5)
	if list[0].BalanceMinor != 2900 {
		t.Fatalf("offline balance at till = %d, want 2900", list[0].BalanceMinor)
	}
	shift, _ := tm.CurrentShift(ctx)
	if shift.Summary.CashAccount != 1500 || shift.Summary.AccountCount != 1 {
		t.Fatalf("shift summary account payments: %+v", shift.Summary)
	}
	sw.down.Store(false)
	waitFor(t, "payment reaches hub", 8*time.Second, func() bool {
		st, err := h.CustomerStatement(ctx, sys, cust.ID)
		return err == nil && st.Customer.BalanceMinor == 2900 && len(st.Entries) == 2
	})
	st, _ := h.CustomerStatement(ctx, sys, cust.ID)
	if st.Entries[0].Kind != "sale" || st.Entries[1].Kind != "payment" || st.Entries[1].BalanceMinor != 2900 {
		t.Fatalf("statement = %+v", st.Entries)
	}
	must(h.AdjustCustomerBalance(ctx, sys, cust.ID, protocol.BalanceAdjustment{AmountMinor: -900, Note: "Discount agreed"}))
	waitFor(t, "adjusted balance reaches till", 5*time.Second, func() bool {
		list, _ := tm.SearchCustomers(ctx, "bola", 5)
		return list[0].BalanceMinor == 2000
	})

	// ---- Reports.
	from, to := time.Now().Add(-time.Hour).UnixMilli(), time.Now().Add(time.Hour).UnixMilli()
	rep, err := h.SalesReport(ctx, sys, from, to, "product")
	must(err)
	if len(rep.Rows) != 1 || rep.Rows[0].QtyMilli != 5000 || rep.Rows[0].NetMinor != 11000 || rep.Rows[0].CostMinor != 7000 || rep.Rows[0].ProfitMinor != 4000 {
		t.Fatalf("product report = %+v", rep.Rows)
	}
	var cashNet, creditNet int64
	for _, m := range rep.Payments {
		switch m.Kind {
		case protocol.PayCash:
			cashNet = m.NetMinor
		case protocol.PayCredit:
			creditNet = m.NetMinor
		}
	}
	if cashNet != 6600+1500 || creditNet != 4400 {
		t.Fatalf("payments: cash %d credit %d", cashNet, creditNet)
	}
	for _, g := range []string{"day", "hour", "weekday", "category", "cashier", "till", "customer", "location"} {
		r, err := h.SalesReport(ctx, sys, from, to, g)
		must(err)
		if r.Totals.NetMinor != 11000 {
			t.Fatalf("%s report total = %d", g, r.Totals.NetMinor)
		}
	}
	var viaHTTP protocol.SalesReport
	must(tm.Admin(ctx, http.MethodGet, "/api/admin/reports/sales?group=cashier&from="+itoa(from)+"&to="+itoa(to), nil, &viaHTTP))
	if len(viaHTTP.Rows) != 1 || viaHTTP.Rows[0].Label != "Kemi" {
		t.Fatalf("cashier report = %+v", viaHTTP.Rows)
	}

	// ---- Approval rules: turn off discount approval.
	if _, err := tm.RecordSale(ctx, kemi, nil, terminal.SaleInput{DiscountMinor: 100, Lines: []terminal.CartLine{{VariantID: pen, QtyMilli: 1000}},
		Payments: []terminal.PaymentInput{{MethodID: cash, AmountMinor: 2100}}}); !errors.Is(err, terminal.ErrApprovalRequired) {
		t.Fatalf("discount without approval: %v", err)
	}
	must(tm.Admin(ctx, http.MethodPut, "/api/admin/settings", map[string]string{"approval_discount": "0"}, nil))
	waitFor(t, "rule reaches till", 5*time.Second, func() bool { return !tm.NeedsApproval(ctx, terminal.ApproveDiscount) })
	s, err := tm.RecordSale(ctx, kemi, nil, terminal.SaleInput{DiscountMinor: 100, Lines: []terminal.CartLine{{VariantID: pen, QtyMilli: 1000}},
		Payments: []terminal.PaymentInput{{MethodID: cash, AmountMinor: 2100}}})
	must(err)
	if s.ApprovedBy != "" {
		t.Fatalf("no approver expected, got %q", s.ApprovedBy)
	}
	if !tm.NeedsApproval(ctx, terminal.ApproveRefund) {
		t.Fatal("other rules stay on")
	}

	// ---- CSV import: dry run, then for real.
	csv := "Title,Author,ISBN,Price,Cost,Qty,Category,SKU\n" +
		"Arrow of God,Chinua Achebe,978-0385014809,5000,3500,12,Fiction,\n" +
		"Pen,,,2500,,,,PEN\n" +
		"No Price Book,Someone,,,,,Fiction,\n" +
		"Anthills of the Savannah,Chinua Achebe,9780385260459,\"4,800.50\",,3,Fiction,\n"
	dry, err := h.ImportProducts(ctx, sys, hub.ImportRequest{CSV: csv, DryRun: true})
	must(err)
	if dry.Created != 2 || dry.Updated != 1 || len(dry.Errors) != 1 || dry.Errors[0].Row != 4 {
		t.Fatalf("dry run = %+v", dry)
	}
	if r, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Query: "arrow"}); len(r.Rows) != 0 {
		t.Fatal("dry run must not change anything")
	}
	res, err := h.ImportProducts(ctx, sys, hub.ImportRequest{CSV: csv})
	must(err)
	if res.Created != 2 || res.Updated != 1 || len(res.Category) != 1 {
		t.Fatalf("import = %+v", res)
	}
	waitFor(t, "imported book at till", 5*time.Second, func() bool {
		it, _ := tm.LookupCode(ctx, "9780385014809")
		return it != nil && it.PriceMinor == 500000 && it.StockMilli == 12_000
	})
	it, _ = tm.LookupCode(ctx, "9780385260459")
	if it == nil || it.PriceMinor != 480050 {
		t.Fatalf("decimal price import: %+v", it)
	}
	waitFor(t, "updated price at till", 5*time.Second, func() bool { it, _ := tm.LookupCode(ctx, "111"); return it.PriceMinor == 250000 })
	exp, err := h.ExportProducts(ctx, sys, "")
	must(err)
	if !strings.Contains(exp, "Arrow of God") || !strings.Contains(exp, "author") || !strings.Contains(exp, "5000.00") {
		t.Fatalf("export:\n%s", exp)
	}
	// Round trip: re-importing the export changes nothing but updates every row.
	again, err := h.ImportProducts(ctx, sys, hub.ImportRequest{CSV: exp, DryRun: true})
	must(err)
	if again.Created != 0 || len(again.Errors) != 0 {
		t.Fatalf("re-import of export = %+v", again)
	}

	_ = admin
	_ = pid
}

func variantBySKU(t *testing.T, h *hub.Hub, sku string) string {
	t.Helper()
	rep, err := h.StockReport(context.Background(), hub.SystemSession, hub.StockReportFilter{Query: sku})
	if err != nil || len(rep.Rows) == 0 {
		t.Fatalf("no variant %s: %v", sku, err)
	}
	return rep.Rows[0].VariantID
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
