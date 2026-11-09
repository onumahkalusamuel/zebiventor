package terminal_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

func TestReturnWithoutReceiptAndStoreCredit(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	if _, err := r.h.SaveUser(r.ctx, hub.SystemSession, protocol.UserInput{Name: "Kemi", Username: "kemi", Role: protocol.RoleCashier, Pin: "4321", Active: true}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "cashier reaches till", 5*time.Second, func() bool {
		_, err := r.tm.Authenticate(r.ctx, "kemi", "4321")
		return err == nil
	})
	kemi, _ := r.tm.Authenticate(r.ctx, "kemi", "4321")
	if _, err := r.tm.OpenShift(r.ctx, r.user, 0); err != nil {
		t.Fatal(err)
	}
	cust, err := r.tm.SaveCustomer(r.ctx, protocol.Customer{Name: "Bola Ade", Phone: "08030000001", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "customer at hub", 5*time.Second, func() bool { return r.tm.Status().Pending == 0 })

	today, _ := r.tm.NoReceiptPrice(r.ctx, r.item.VariantID) // grocery: VAT included
	if today != 250000 {
		t.Fatalf("today's price %d", today)
	}
	line := terminal.NoReceiptLine{VariantID: r.item.VariantID, QtyMilli: 2000, Restock: true}
	in := terminal.NoReceiptInput{Lines: []terminal.NoReceiptLine{line}, Reason: "Changed mind",
		Refunds: []terminal.PaymentInput{{MethodID: terminal.StoreCreditMethodID, AmountMinor: 500000}}}

	// A cashier always needs a manager, whatever the approval settings.
	if err := r.h.SaveSettings(r.ctx, hub.SystemSession, map[string]string{"approval_refund": "0"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "setting", 5*time.Second, func() bool { return r.tm.Settings(r.ctx)["approval_refund"] == "0" })
	if _, err := r.tm.RecordReturnNoReceipt(r.ctx, kemi, nil, in); !errors.Is(err, terminal.ErrApprovalRequired) {
		t.Fatalf("cashier without approval: %v", err)
	}
	// More than today's price: refused.
	bad := in
	bad.Lines = []terminal.NoReceiptLine{{VariantID: r.item.VariantID, QtyMilli: 1000, UnitRefundMinor: 260000}}
	if _, err := r.tm.RecordReturnNoReceipt(r.ctx, kemi, &r.user, bad); err == nil {
		t.Fatal("refund above today's price accepted")
	}
	// Store credit needs a customer.
	if _, err := r.tm.RecordReturnNoReceipt(r.ctx, kemi, &r.user, in); err == nil {
		t.Fatal("store credit without a customer accepted")
	}
	in.CustomerID = cust.ID
	r.sw.down.Store(true) // works offline
	ret, err := r.tm.RecordReturnNoReceipt(r.ctx, kemi, &r.user, in)
	if err != nil {
		t.Fatal(err)
	}
	if ret.SaleID != "" || ret.ApprovedBy != r.user.ID || ret.TotalMinor != 500000 {
		t.Fatalf("return %+v", ret)
	}
	// The credit can be spent at once, even offline.
	found, _ := r.tm.SearchCustomers(r.ctx, "bola", 5)
	if len(found) != 1 || found[0].BalanceMinor != -500000 {
		t.Fatalf("customer balance after store credit: %+v", found)
	}
	sale := terminal.SaleInput{CustomerID: cust.ID, Lines: []terminal.CartLine{{VariantID: r.item.VariantID, QtyMilli: 3000}}}
	sale.Payments = []terminal.PaymentInput{{MethodID: terminal.StoreCreditMethodID, AmountMinor: 600000}, {MethodID: r.cash.ID, AmountMinor: 150000}}
	if _, err := r.tm.RecordSale(r.ctx, kemi, nil, sale); err == nil {
		t.Fatal("spent more store credit than the customer has")
	}
	sale.Payments = []terminal.PaymentInput{{MethodID: terminal.StoreCreditMethodID, AmountMinor: 500000}, {MethodID: r.cash.ID, AmountMinor: 250000}}
	if _, err := r.tm.RecordSale(r.ctx, kemi, nil, sale); err != nil {
		t.Fatal(err)
	}

	r.sw.down.Store(false)
	r.tm.Nudge()
	waitFor(t, "return and sale reach hub", 10*time.Second, func() bool { return r.tm.Status().Pending == 0 })
	var bal int64
	_ = r.h.DB().R().QueryRow("SELECT balance_minor FROM customers WHERE id=?", cust.ID).Scan(&bal)
	if bal != 0 {
		t.Fatalf("hub balance after spending the credit: %d", bal)
	}
	var stock int64
	_ = r.h.DB().R().QueryRow("SELECT qty_milli FROM stock_levels WHERE variant_id=?", r.item.VariantID).Scan(&stock)
	if stock != 50_000+2_000-3_000 {
		t.Fatalf("stock %d", stock)
	}
	var reason string
	_ = r.h.DB().R().QueryRow("SELECT reason FROM returns WHERE id=?", ret.ID).Scan(&reason)
	if reason != "No receipt: Changed mind" {
		t.Fatalf("reason %q", reason)
	}
}

// serveHub runs a hub's real listener (TLS and plain on one port).
func serveHub(t *testing.T, h *hub.Hub) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	go func() {
		_ = h.Serve(ctx, "127.0.0.1:0", func(a net.Addr) { ready <- a.String() })
	}()
	t.Cleanup(cancel)
	select {
	case addr := <-ready:
		return addr
	case <-time.After(5 * time.Second):
		t.Fatal("hub did not start")
	}
	return ""
}

func newHub(t *testing.T) *hub.Hub {
	t.Helper()
	ctx := context.Background()
	h, err := hub.Open(ctx, hub.Config{DataDir: t.TempDir(), Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	if err := h.Setup(ctx, hub.SetupInput{StoreName: "Secure Shop", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestEncryptedSync(t *testing.T) {
	ctx := context.Background()
	h := newHub(t)
	addr := serveHub(t, h)
	pair := func(dir, name, url string) *terminal.Terminal {
		code, _ := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
		tm, err := terminal.Open(ctx, terminal.Config{DataDir: dir, Version: "test", Logger: quiet})
		if err != nil {
			t.Fatal(err)
		}
		info, err := tm.HubInfo(ctx, url)
		if err != nil || info.TLSFingerprint != h.TLSFingerprint() {
			t.Fatalf("hub info over TLS: %v %q", err, info.TLSFingerprint)
		}
		if _, err := tm.Pair(ctx, url, code.Code, name); err != nil {
			t.Fatal(err)
		}
		tm.StartSync()
		return tm
	}

	// A new till pairs encrypted and pins the hub's certificate.
	dirA := t.TempDir()
	a := pair(dirA, "Till A", addr)
	defer func() { a.Close() }()
	st := a.Status()
	if !st.Encrypted || !strings.HasPrefix(st.HubURL, "https://") || st.SecurityCode != terminal.SecurityCode(h.TLSFingerprint()) {
		t.Fatalf("status after pairing: %+v", st)
	}
	waitFor(t, "users over TLS", 5*time.Second, func() bool { return a.HasUsers(ctx) })
	waitFor(t, "hub records encrypted till", 5*time.Second, func() bool {
		list, _ := h.ListTerminals(ctx)
		return len(list) == 1 && list[0].Encrypted
	})

	// A till paired before encryption (plain address, nothing pinned)
	// upgrades itself.
	dirB := t.TempDir()
	b := pair(dirB, "Till B", addr)
	b.Close()
	{
		tb, _ := terminal.Open(ctx, terminal.Config{DataDir: dirB, Logger: quiet})
		_ = tb.DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "UPDATE kv SET value=? WHERE key='hub_url'", "http://"+addr)
			if err == nil {
				_, err = tx.ExecContext(ctx, "UPDATE kv SET value='' WHERE key='hub_cert'")
			}
			return err
		})
		tb.Close()
	}
	b, _ = terminal.Open(ctx, terminal.Config{DataDir: dirB, Version: "test", Logger: quiet})
	defer func() { b.Close() }()
	if b.Status().Encrypted {
		t.Fatal("legacy till should start unencrypted")
	}
	b.StartSync()
	waitFor(t, "legacy till upgrades to TLS", 5*time.Second, func() bool { return b.Status().Encrypted })

	// Admin requires encryption: plain requests are refused with a clear code.
	if err := h.SaveSettings(ctx, hub.SystemSession, map[string]string{"require_encryption": "1"}); err != nil {
		t.Fatal(err)
	}
	var tok string
	_ = a.DB().R().QueryRow("SELECT value FROM kv WHERE key='token'").Scan(&tok)
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/api/sync/pull?since=0", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var ae protocol.APIError
	_ = json.NewDecoder(resp.Body).Decode(&ae)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUpgradeRequired || ae.Code != "tls" {
		t.Fatalf("plain request with encryption required: %d %+v", resp.StatusCode, ae)
	}
	// Encrypted tills carry on.
	if _, err := h.SaveProduct(ctx, hub.SystemSession, protocol.ProductInput{Product: protocol.Product{Name: "Pen", Unit: "pc"},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 10000}, Barcodes: []string{"333"}}}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "product over TLS", 5*time.Second, func() bool { it, _ := a.LookupCode(ctx, "333"); return it != nil })

	// Something else answering at the hub's address (another certificate)
	// is refused, with a message saying what to do.
	other := newHub(t)
	otherAddr := serveHub(t, other)
	_ = a.DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE kv SET value=? WHERE key='hub_url'", "https://"+otherAddr)
		return err
	})
	a.Close()
	a, _ = terminal.Open(ctx, terminal.Config{DataDir: dirA, Version: "test", Logger: quiet})
	a.StartSync()
	waitFor(t, "impostor refused", 5*time.Second, func() bool { return strings.Contains(a.Status().LastError, "certificate has changed") })
	list, _ := other.ListTerminals(ctx)
	if len(list) != 0 {
		t.Fatal("till talked to a hub with another certificate")
	}
}

func TestBranchStockWorkOffline(t *testing.T) {
	var branch, main string
	r := newRig(t, "test", "test", func(h *hub.Hub) string {
		main, _ = h.Setting(context.Background(), "default_location_id")
		branch, _ = h.SaveLocation(context.Background(), hub.SystemSession, protocol.Location{Name: "Ikeja branch", Active: true})
		return branch
	})
	sup, err := r.h.SaveSupplier(r.ctx, hub.SystemSession, protocol.Supplier{Name: "Nestle Distributors"})
	if err != nil {
		t.Fatal(err)
	}
	poID, err := r.h.SavePurchaseOrder(r.ctx, hub.SystemSession, protocol.PurchaseOrderInput{SupplierID: sup, LocationID: branch, Submit: true,
		Lines: []protocol.PurchaseOrderLine{{VariantID: r.item.VariantID, QtyOrderedMilli: 5_000, UnitCostMinor: 180000}}})
	if err != nil {
		t.Fatal(err)
	}
	var due []terminal.ExpectedDelivery
	waitFor(t, "order expected at branch", 5*time.Second, func() bool {
		due, _ = r.tm.ExpectedDeliveries(r.ctx)
		return len(due) == 1
	})
	locs, _ := r.tm.Locations(r.ctx)
	if len(locs) != 1 || locs[0].ID != main {
		t.Fatalf("locations to send to: %+v", locs)
	}

	// Internet down at the branch: the delivery arrives (4 of 5) and 2 are
	// sent on to the main store.
	r.sw.down.Store(true)
	in := terminal.DeliveryInput{SupplierRef: "INV-77"}
	in.Lines = append(in.Lines, terminal.DeliveryLine{POLineID: due[0].Order.Lines[0].ID, QtyMilli: 4_000})
	if err := r.tm.ReceiveDelivery(r.ctx, r.user, poID, in); err != nil {
		t.Fatal(err)
	}
	if err := r.tm.ReceiveDelivery(r.ctx, r.user, poID, in); err == nil {
		t.Fatal("same order booked twice while offline")
	}
	sent, err := r.tm.SendTransfer(r.ctx, r.user, terminal.SendTransferInput{ToLocationID: main, Note: "for the shop",
		Lines: []protocol.StockTransferLine{{VariantID: r.item.VariantID, QtyMilli: 2_000}}})
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := r.tm.OutgoingTransfers(r.ctx); len(out) != 1 || r.tm.Status().Pending != 2 {
		t.Fatalf("offline: outgoing %d pending %d", len(out), r.tm.Status().Pending)
	}

	r.sw.down.Store(false)
	r.tm.Nudge()
	waitFor(t, "branch work reaches hub", 10*time.Second, func() bool { return r.tm.Status().Pending == 0 && r.tm.Status().Rejected == 0 })
	po, _ := r.h.GetPurchaseOrder(r.ctx, hub.SystemSession, poID)
	if po.Status != protocol.POPartial || po.Lines[0].QtyReceivedMilli != 4_000 || len(po.Receipts) != 1 || po.Receipts[0].SupplierRef != "INV-77" {
		t.Fatalf("order at hub: %+v", po)
	}
	tr, err := r.h.GetTransfer(r.ctx, hub.SystemSession, sent.ID)
	if err != nil || tr.TransferNo == "" || tr.FromLocationID != branch || tr.Status != protocol.TransferInTransit {
		t.Fatalf("transfer at hub: %+v %v", tr, err)
	}
	var atBranch int64
	_ = r.h.DB().R().QueryRow("SELECT qty_milli FROM stock_levels WHERE location_id=?", branch).Scan(&atBranch)
	if atBranch != 2_000 {
		t.Fatalf("branch stock %d, want 2000", atBranch)
	}
	waitFor(t, "till learns the transfer number", 5*time.Second, func() bool {
		out, _ := r.tm.OutgoingTransfers(r.ctx)
		return len(out) == 0
	})
	// The partly delivered order is still expected for the rest.
	due, _ = r.tm.ExpectedDeliveries(r.ctx)
	if len(due) != 1 || due[0].ReceivedHere || due[0].Order.Lines[0].QtyReceivedMilli != 4_000 {
		t.Fatalf("still due: %+v", due)
	}
}

// Reports read daily summaries for whole-day periods and raw sale lines
// otherwise: both must agree, for every grouping.
func TestReportSummariesMatchRaw(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	if _, err := r.h.SaveUser(r.ctx, hub.SystemSession, protocol.UserInput{Name: "Kemi", Username: "kemi", Role: protocol.RoleCashier, Pin: "4321", Active: true}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "cashier", 5*time.Second, func() bool { _, err := r.tm.Authenticate(r.ctx, "kemi", "4321"); return err == nil })
	kemi, _ := r.tm.Authenticate(r.ctx, "kemi", "4321")
	if _, err := r.tm.OpenShift(r.ctx, r.user, 0); err != nil {
		t.Fatal(err)
	}
	cust, _ := r.tm.SaveCustomer(r.ctx, protocol.Customer{Name: "Bola", Active: true})
	var receipts []string
	for i, u := range []terminal.User{r.user, kemi, kemi, r.user} {
		in := terminal.SaleInput{Lines: []terminal.CartLine{{VariantID: r.item.VariantID, QtyMilli: int64(1000 + i*1000)}},
			Payments: []terminal.PaymentInput{{MethodID: r.cash.ID, AmountMinor: 2_000_000}}}
		if i == 2 {
			in.CustomerID, in.DiscountMinor = cust.ID, 10000
		}
		s, err := r.tm.RecordSale(r.ctx, u, &r.user, in)
		if err != nil {
			t.Fatal(err)
		}
		receipts = append(receipts, s.ReceiptNo)
	}
	waitFor(t, "sales", 5*time.Second, func() bool { return r.tm.Status().Pending == 0 })
	rs, _ := r.tm.LookupForReturn(r.ctx, receipts[1])
	if _, err := r.tm.RecordReturn(r.ctx, r.user, nil, terminal.ReturnInput{ReceiptNo: receipts[1], Reason: "x",
		Lines:   []terminal.ReturnLineInput{{SaleLineID: rs.Lines[0].ID, QtyMilli: 1000, Restock: true}},
		Refunds: []terminal.PaymentInput{{MethodID: r.cash.ID, AmountMinor: rs.Lines[0].PaidMinor / 2}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.tm.RecordReturnNoReceipt(r.ctx, r.user, nil, terminal.NoReceiptInput{Reason: "y", CustomerID: cust.ID,
		Lines:   []terminal.NoReceiptLine{{VariantID: r.item.VariantID, QtyMilli: 1000}},
		Refunds: []terminal.PaymentInput{{MethodID: terminal.StoreCreditMethodID, AmountMinor: 250000}}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "returns", 5*time.Second, func() bool { return r.tm.Status().Pending == 0 })

	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from, to := day.UnixMilli(), day.AddDate(0, 0, 1).UnixMilli()
	for _, g := range []string{"day", "hour", "weekday", "month", "weekhour", "product", "category", "cashier", "till", "customer", "location"} {
		sum, err := r.h.SalesReport(r.ctx, hub.SystemSession, from, to, g)
		if err != nil {
			t.Fatalf("%s summary: %v", g, err)
		}
		raw, err := r.h.SalesReport(r.ctx, hub.SystemSession, from+1, to-1, g)
		if err != nil {
			t.Fatalf("%s raw: %v", g, err)
		}
		norm := func(rep *protocol.SalesReport) string {
			b, _ := json.Marshal(rep.Rows)
			return string(b)
		}
		sortRows := func(rep *protocol.SalesReport) {
			sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].Key < rep.Rows[j].Key })
		}
		sortRows(sum)
		sortRows(raw)
		if norm(sum) != norm(raw) || sum.Totals != raw.Totals {
			t.Errorf("%s: summary and raw differ\nsummary %s %+v\nraw     %s %+v", g, norm(sum), sum.Totals, norm(raw), raw.Totals)
		}
		pj := func(rep *protocol.SalesReport) string {
			sort.Slice(rep.Payments, func(i, j int) bool { return rep.Payments[i].MethodName < rep.Payments[j].MethodName })
			b, _ := json.Marshal(rep.Payments)
			return string(b)
		}
		if pj(sum) != pj(raw) {
			t.Errorf("%s payments differ: %s vs %s", g, pj(sum), pj(raw))
		}
	}
}

// An admin till exports the store; the file restores as a working store on
// another "PC" (a fresh folder), pictures included.
func TestExportAndRestoreElsewhere(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	pid, _ := r.h.DB().R().Query("SELECT id FROM products LIMIT 1")
	var productID string
	for pid.Next() {
		pid.Scan(&productID)
	}
	pid.Close()
	if _, err := r.h.SetProductImage(r.ctx, hub.SystemSession, productID, demoJPEG()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.tm.OpenShift(r.ctx, r.user, 0); err != nil {
		t.Fatal(err)
	}
	r.sell(t)
	waitFor(t, "sale", 5*time.Second, func() bool { return r.hubSales() == 1 && r.tm.Status().Pending == 0 })

	out := filepath.Join(t.TempDir(), "store.zebi.db")
	f, _ := os.Create(out)
	if err := r.tm.AdminDownload(r.ctx, "/api/admin/backups/export", f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	info, err := hub.InspectBackup(r.ctx, out, "")
	if err != nil || info.Problem != "" || info.Sales != 1 || info.StoreName != "Test Store" {
		t.Fatalf("export: %+v %v", info, err)
	}
	// "Another PC": restore into an empty folder and open it.
	dir := t.TempDir()
	if err := hub.RestoreFile(r.ctx, dir, out); err != nil {
		t.Fatal(err)
	}
	h2, err := hub.Open(r.ctx, hub.Config{DataDir: dir, Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer h2.Close()
	var n, imgs int
	_ = h2.DB().R().QueryRow("SELECT count(*) FROM sales").Scan(&n)
	_ = h2.DB().R().QueryRow("SELECT count(*) FROM images").Scan(&imgs)
	if n != 1 || imgs != 1 {
		t.Fatalf("restored store: %d sales, %d pictures", n, imgs)
	}
	var tid string
	_ = h2.DB().R().QueryRow("SELECT id FROM terminals LIMIT 1").Scan(&tid)
	if _, err := h2.Login(r.ctx, &hub.Terminal{ID: tid}, "", protocol.LoginRequest{Username: "admin", Password: "secret123"}); err != nil {
		t.Fatalf("admin can't sign in on the restored store: %v", err)
	}
}

func demoJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}
