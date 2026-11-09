package hub_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

func openHub(t *testing.T) (*hub.Hub, func(error)) {
	t.Helper()
	ctx := context.Background()
	h, err := hub.Open(ctx, hub.Config{DataDir: t.TempDir(), Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(h.Setup(ctx, hub.SetupInput{StoreName: "Shop", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}))
	return h, must
}

func TestTwoStepTransfers(t *testing.T) {
	ctx := context.Background()
	h, must := openHub(t)
	sys := hub.SystemSession
	_, err := h.SaveProduct(ctx, sys, protocol.ProductInput{
		Product:  protocol.Product{Name: "Pen", TrackStock: true},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{SKU: "PEN", PriceMinor: 2000, CostMinor: 1000}, OpeningQtyMilli: 20_000}},
	})
	must(err)
	locs, err := h.ListLocations(ctx, sys)
	must(err)
	shop := locs[0].ID
	wh, err := h.SaveLocation(ctx, sys, protocol.Location{Name: "Warehouse"})
	must(err)
	rep, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Query: "PEN"})
	pen := rep.Rows[0].VariantID
	qty := func(loc string) int64 {
		r, err := h.StockReport(ctx, sys, hub.StockReportFilter{LocationID: loc, Query: "PEN"})
		must(err)
		return r.Rows[0].QtyMilli
	}

	// Sent: leaves the shop, not yet at the warehouse.
	id, err := h.TransferStock(ctx, sys, hub.TransferInput{FromLocationID: shop, ToLocationID: wh, Lines: []protocol.StockTransferLine{{VariantID: pen, QtyMilli: 8_000}}})
	must(err)
	if a, b := qty(shop), qty(wh); a != 12_000 || b != 0 {
		t.Fatalf("in transit: shop=%d warehouse=%d", a, b)
	}
	tr, err := h.GetTransfer(ctx, sys, id)
	must(err)
	if tr.Status != protocol.TransferInTransit || tr.CostMinor != 8_000 || tr.Lines[0].ReceivedQtyMilli != 0 {
		t.Fatalf("transfer = %+v", tr)
	}
	if err := h.ReceiveTransfer(ctx, sys, id, protocol.TransferReceipt{Lines: []protocol.StockTransferLine{{VariantID: pen, ReceivedQtyMilli: 9_000}}}); err == nil {
		t.Fatal("receiving more than was sent must fail")
	}
	// One went missing on the way.
	must(h.ReceiveTransfer(ctx, sys, id, protocol.TransferReceipt{Note: "one box damp", Lines: []protocol.StockTransferLine{{VariantID: pen, ReceivedQtyMilli: 7_000}}}))
	if a, b := qty(shop), qty(wh); a != 12_000 || b != 7_000 {
		t.Fatalf("received: shop=%d warehouse=%d", a, b)
	}
	tr, _ = h.GetTransfer(ctx, sys, id)
	if tr.Status != protocol.TransferReceived || tr.Lines[0].ReceivedQtyMilli != 7_000 || tr.ReceiveNote != "one box damp" {
		t.Fatalf("after receipt = %+v", tr)
	}
	if err := h.ReceiveTransfer(ctx, sys, id, protocol.TransferReceipt{}); err == nil {
		t.Fatal("a transfer can be received only once")
	}
	if err := h.CancelTransfer(ctx, sys, id); err == nil {
		t.Fatal("a received transfer cannot be cancelled")
	}

	// Cancelled: the goods go back where they came from.
	id2, err := h.TransferStock(ctx, sys, hub.TransferInput{FromLocationID: wh, ToLocationID: shop, Lines: []protocol.StockTransferLine{{VariantID: pen, QtyMilli: 2_000}}})
	must(err)
	if qty(wh) != 5_000 {
		t.Fatalf("warehouse after send = %d", qty(wh))
	}
	must(h.CancelTransfer(ctx, sys, id2))
	if a, b := qty(shop), qty(wh); a != 12_000 || b != 7_000 {
		t.Fatalf("after cancel: shop=%d warehouse=%d", a, b)
	}
	open, err := h.ListTransfers(ctx, sys, protocol.TransferInTransit)
	must(err)
	if len(open) != 0 {
		t.Fatalf("%d transfers still in transit", len(open))
	}
	// One step for goods moved within the building.
	_, err = h.TransferStock(ctx, sys, hub.TransferInput{FromLocationID: shop, ToLocationID: wh, ReceiveNow: true, Lines: []protocol.StockTransferLine{{VariantID: pen, QtyMilli: 1_000}}})
	must(err)
	if a, b := qty(shop), qty(wh); a != 11_000 || b != 8_000 {
		t.Fatalf("one step: shop=%d warehouse=%d", a, b)
	}
}

func TestSupplierPayables(t *testing.T) {
	ctx := context.Background()
	h, must := openHub(t)
	sys := hub.SystemSession
	_, err := h.SaveProduct(ctx, sys, protocol.ProductInput{
		Product:  protocol.Product{Name: "Pen", TrackStock: true},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{SKU: "PEN", PriceMinor: 2000, CostMinor: 1000}}},
	})
	must(err)
	rep, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Query: "PEN"})
	pen := rep.Rows[0].VariantID
	sup, err := h.SaveSupplier(ctx, sys, protocol.Supplier{Name: "Stationers", TermsDays: 30})
	must(err)
	balance := func() (int64, int64) {
		list, err := h.ListSuppliers(ctx, sys)
		must(err)
		return list[0].BalanceMinor, list[0].OverdueMinor
	}

	// A delivery, billed at its value; the due date follows the terms.
	grn, err := h.ReceiveGoods(ctx, sys, protocol.GoodsReceiptInput{SupplierID: sup, SupplierRef: "INV-77",
		Lines: []protocol.GoodsReceiptLine{{VariantID: pen, QtyMilli: 10_000, UnitCostMinor: 1500}}})
	must(err)
	b1, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{GRNID: grn, SupplierRef: "INV-77"})
	must(err)
	bill, _, err := h.GetBill(ctx, sys, b1)
	must(err)
	if bill.AmountMinor != 15_000 || bill.BillNo != "BILL-000001" || bill.SupplierID != sup || bill.DueDate-bill.BillDate != 30*24*3600*1000 {
		t.Fatalf("bill = %+v", bill)
	}
	if _, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{GRNID: grn}); err == nil {
		t.Fatal("a delivery must not be billed twice")
	}
	if _, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{SupplierID: sup, SupplierRef: "inv-77", AmountMinor: 100}); err == nil {
		t.Fatal("the same supplier invoice must not be entered twice")
	}
	g, err := h.GetGoodsReceipt(ctx, sys, grn)
	must(err)
	if g.BillNo != "BILL-000001" {
		t.Fatalf("delivery should show its bill, got %q", g.BillNo)
	}
	// An older bill, already 45 days overdue.
	old := time.Now().AddDate(0, 0, -75)
	oldDay := time.Date(old.Year(), old.Month(), old.Day(), 0, 0, 0, 0, time.Local).UnixMilli()
	b0, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{SupplierID: sup, SupplierRef: "INV-12", BillDate: oldDay, AmountMinor: 4_000})
	must(err)
	if bal, overdue := balance(); bal != 19_000 || overdue != 4_000 {
		t.Fatalf("balance=%d overdue=%d", bal, overdue)
	}
	ap, err := h.PayablesReport(ctx, sys)
	must(err)
	if ap.Totals.NotDueMinor != 15_000 || ap.Totals.Days31To60 != 4_000 || ap.Totals.TotalMinor != 19_000 {
		t.Fatalf("ageing = %+v", ap.Totals)
	}

	// Paying 10,000 clears the oldest bill first, then part of the next.
	p1, err := h.PaySupplier(ctx, sys, protocol.SupplierPaymentInput{SupplierID: sup, Method: "Bank transfer", Reference: "TRF-1", AmountMinor: 10_000})
	must(err)
	pay, err := h.GetSupplierPayment(ctx, sys, p1)
	must(err)
	if pay.PaymentNo != "SP-000001" || len(pay.Allocations) != 2 || pay.Allocations[0].BillID != b0 || pay.Allocations[0].AmountMinor != 4_000 ||
		pay.Allocations[1].AmountMinor != 6_000 {
		t.Fatalf("allocations = %+v", pay.Allocations)
	}
	old0, _, _ := h.GetBill(ctx, sys, b0)
	if old0.Status != protocol.BillPaid {
		t.Fatalf("oldest bill = %s", old0.Status)
	}
	if err := h.VoidBill(ctx, sys, b1, "wrong"); err == nil {
		t.Fatal("a part-paid bill must not be voidable")
	}
	// Overpaying leaves a credit that the next bill uses up.
	_, err = h.PaySupplier(ctx, sys, protocol.SupplierPaymentInput{SupplierID: sup, Method: "Cash", AmountMinor: 12_000})
	must(err)
	if bal, _ := balance(); bal != -3_000 {
		t.Fatalf("balance after overpaying = %d", bal)
	}
	ap, _ = h.PayablesReport(ctx, sys)
	if ap.Totals.TotalMinor != 0 || ap.Totals.CreditMinor != 3_000 {
		t.Fatalf("payables after overpaying = %+v", ap.Totals)
	}
	b2, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{SupplierID: sup, SupplierRef: "INV-80", AmountMinor: 5_000})
	must(err)
	bill2, pays, _ := h.GetBill(ctx, sys, b2)
	if bill2.PaidMinor != 3_000 || bill2.Status != protocol.BillOpen || len(pays) != 1 {
		t.Fatalf("credit not applied: %+v (%d payments)", bill2, len(pays))
	}
	// Void an unpaid bill.
	b3, err := h.CreateBill(ctx, sys, protocol.SupplierBillInput{SupplierID: sup, AmountMinor: 700})
	must(err)
	must(h.VoidBill(ctx, sys, b3, "entered twice"))
	st, err := h.SupplierStatement(ctx, sys, sup)
	must(err)
	last := st.Entries[len(st.Entries)-1]
	if last.Kind != "void" || last.BalanceMinor != 2_000 || st.Supplier.BalanceMinor != 2_000 || len(st.Bills) != 1 {
		t.Fatalf("statement: last=%+v balance=%d open=%d", last, st.Supplier.BalanceMinor, len(st.Bills))
	}
	// The opening bill was dated 75 days ago, so it is first on the statement.
	if st.Entries[0].RefNo != "BILL-000002" {
		t.Fatalf("statement order: first = %s", st.Entries[0].RefNo)
	}
}
