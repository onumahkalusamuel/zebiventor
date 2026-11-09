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
