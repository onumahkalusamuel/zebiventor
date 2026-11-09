package documents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

var naira = money.Currency{Code: "NGN", Symbol: "₦", Digits: 2}

func TestPurchaseOrder(t *testing.T) {
	store := StoreFromSettings(map[string]string{"store_name": "Kalu Books", "store_address": "12 Allen Ave\nIkeja", "tax_id": "1234"})
	o := &protocol.PurchaseOrder{PONo: "PO-000007", SupplierName: "Stationers", LocationName: "Shop", Status: protocol.POOrdered, OrderedAt: 1_700_000_000_000,
		Note: "Deliver before 10am",
		Lines: []protocol.PurchaseOrderLine{
			{ProductName: "Things Fall Apart", VariantName: "Default", SKU: "TFA", QtyOrderedMilli: 12_000, UnitCostMinor: 315000},
			{ProductName: "Rice <bag>", VariantName: "50kg", QtyOrderedMilli: 2_500, UnitCostMinor: 100, Unit: "kg"},
		}}
	html, err := PurchaseOrder(o, &protocol.Supplier{Name: "Stationers", Address: "Lagos", Phone: "0801"}, store, naira).HTML()
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{"PO-000007", "Kalu Books", "TIN: 1234", "Ikeja", "Things Fall Apart", "37,800.00", "₦37,802.50",
		"2.5 kg", "Rice &lt;bag&gt; – 50kg", "Deliver before 10am", "window.print()"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(s, "Things Fall Apart – Default") {
		t.Error("default variant name should be hidden")
	}
	if dir := os.Getenv("DOCUMENT_PREVIEW_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "po.html"), html, 0o644)
	}
}

func TestTransferNoteAndStatement(t *testing.T) {
	tr := &protocol.StockTransfer{TransferNo: "TR-000002", FromLocationName: "Shop", ToLocationName: "Warehouse", Status: protocol.TransferInTransit,
		Lines: []protocol.StockTransferLine{{ProductName: "Pen", QtyMilli: 8_000}}}
	html, err := TransferNote(tr, Store{Name: "S"}).HTML()
	if err != nil || !strings.Contains(string(html), "Qty received") || !strings.Contains(string(html), "Carried by") {
		t.Fatalf("in-transit note: %v", err)
	}
	tr.Status, tr.Lines[0].ReceivedQtyMilli, tr.ReceiveNote = protocol.TransferReceived, 7_000, "one damp"
	html, _ = TransferNote(tr, Store{Name: "S"}).HTML()
	if !strings.Contains(string(html), "On receipt: one damp") || strings.Contains(string(html), "Carried by") {
		t.Fatal("received note should show receipt, not signature boxes")
	}

	st := &protocol.SupplierStatement{Supplier: protocol.Supplier{Name: "Stationers", BalanceMinor: 2_000_00, OverdueMinor: 500_00, TermsDays: 30},
		Entries: []protocol.SupplierLedgerEntry{
			{Kind: "bill", RefNo: "BILL-000001", AmountMinor: 5_000_00, BalanceMinor: 5_000_00},
			{Kind: "payment", RefNo: "SP-000001", AmountMinor: -3_000_00, BalanceMinor: 2_000_00},
		},
		Bills: []protocol.SupplierBill{{BillNo: "BILL-000001", SupplierRef: "INV-9", AmountMinor: 5_000_00, PaidMinor: 3_000_00}}}
	html, _ = SupplierStatement(st, Store{Name: "S"}, naira).HTML()
	for _, want := range []string{"₦2,000.00", "of which overdue", "BILL-000001 (INV-9): ₦2,000.00", "30 days", "3,000.00"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("statement missing %q", want)
		}
	}
	if dir := os.Getenv("DOCUMENT_PREVIEW_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "statement.html"), html, 0o644)
	}
}

func TestPDF(t *testing.T) {
	store := StoreFromSettings(map[string]string{"store_name": "Zebi Books & Stationery", "store_address": "12 Allen Avenue, Ikeja, Lagos", "store_phone": "0803 000 0000"})
	o := &protocol.PurchaseOrder{PONo: "PO-000007", SupplierName: "Lagos Book Distributors", LocationName: "Main Store", Status: protocol.POOrdered,
		OrderedAt: 1_700_000_000_000, Note: "Deliver before 10am.\nCall Tunde on arrival."}
	for i := range 60 { // enough for a second page
		o.Lines = append(o.Lines, protocol.PurchaseOrderLine{ProductName: "Half of a Yellow Sun — a very long title that has to wrap inside its column " + itoa(i),
			SKU: "HYS" + itoa(i), QtyOrderedMilli: 12_000, UnitCostMinor: 455000})
	}
	doc := PurchaseOrder(o, &protocol.Supplier{Name: "Lagos Book Distributors", ContactName: "Tunde Bello", Phone: "0802"}, store, naira)
	b, err := doc.PDF()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "%PDF-") || len(b) < 10_000 {
		t.Fatalf("not a PDF (%d bytes)", len(b))
	}
	if n := strings.Count(string(b), "/Type /Page\n"); n < 2 {
		t.Logf("pages marker count %d", n)
	}
	if dir := os.Getenv("DOCUMENT_PREVIEW_DIR"); dir != "" {
		os.WriteFile(filepath.Join(dir, "po.pdf"), b, 0o644)
	}
	// Every kind renders.
	st := &protocol.SupplierStatement{Supplier: protocol.Supplier{Name: "S"}, Entries: []protocol.SupplierLedgerEntry{{Kind: "bill", RefNo: "B1", AmountMinor: 100, BalanceMinor: 100}}}
	tr := &protocol.StockTransfer{TransferNo: "TR-1", Lines: []protocol.StockTransferLine{{ProductName: "Pen", QtyMilli: 1000}}}
	p := &protocol.SupplierPayment{PaymentNo: "SP-1", AmountMinor: 100}
	for _, d := range []Doc{SupplierStatement(st, store, naira), TransferNote(tr, store), PaymentAdvice(p, nil, store, naira)} {
		if _, err := d.PDF(); err != nil {
			t.Fatalf("%s: %v", d.Title, err)
		}
	}
}
