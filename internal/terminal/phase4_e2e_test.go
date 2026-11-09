package terminal_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
	"github.com/onumahkalusamuel/zebiventor/internal/update"
)

// swappable serves whichever hub is current (a restore reopens it) and can
// go "offline".
type swappable struct {
	h    atomic.Pointer[http.Handler]
	down atomic.Bool
}

func (s *swappable) set(h http.Handler) { s.h.Store(&h) }
func (s *swappable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.down.Load() {
		if hj, ok := w.(http.Hijacker); ok {
			c, _, _ := hj.Hijack()
			c.Close()
			return
		}
	}
	(*s.h.Load()).ServeHTTP(w, r)
}

type rig struct {
	ctx  context.Context
	dir  string
	h    *hub.Hub
	sw   *swappable
	srv  *httptest.Server
	tm   *terminal.Terminal
	user terminal.User
	cash protocol.PaymentMethod
	item *terminal.Item
}

func newRig(t *testing.T, hubVersion, tillVersion string, location func(*hub.Hub) string) *rig {
	t.Helper()
	r := &rig{ctx: context.Background(), dir: t.TempDir(), sw: &swappable{}}
	var err error
	if r.h, err = hub.Open(r.ctx, hub.Config{DataDir: r.dir, Version: hubVersion, Logger: quiet}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.h.Close() })
	if err := r.h.Setup(r.ctx, hub.SetupInput{StoreName: "Test Store", BusinessType: "grocery", AdminName: "Ada",
		AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.h.SaveProduct(r.ctx, hub.SystemSession, protocol.ProductInput{
		Product:  protocol.Product{Name: "Peak Milk 400g", TrackStock: true, Unit: "pc"},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 250000}, Barcodes: []string{"111"}, OpeningQtyMilli: 50_000}},
	}); err != nil {
		t.Fatal(err)
	}
	r.sw.set(r.h.Handler())
	r.srv = httptest.NewServer(r.sw)
	t.Cleanup(r.srv.Close)

	req := protocol.PairingCodeRequest{}
	if location != nil {
		req.LocationID = location(r.h)
	}
	code, err := r.h.CreatePairingCode(r.ctx, "", req)
	if err != nil {
		t.Fatal(err)
	}
	if r.tm, err = terminal.Open(r.ctx, terminal.Config{DataDir: t.TempDir(), Version: tillVersion, Logger: quiet}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.tm.Close() })
	if _, err := r.tm.Pair(r.ctx, r.srv.URL, code.Code, "Till 1"); err != nil {
		t.Fatal(err)
	}
	r.tm.StartSync()
	waitFor(t, "catalogue", 5*time.Second, func() bool {
		r.item, _ = r.tm.LookupCode(r.ctx, "111")
		return r.item != nil && r.tm.HasUsers(r.ctx)
	})
	if r.user, err = r.tm.Login(r.ctx, "admin", "secret123"); err != nil {
		t.Fatal(err)
	}
	methods, _ := r.tm.PaymentMethods(r.ctx)
	for _, m := range methods {
		if m.Kind == protocol.PayCash {
			r.cash = m
		}
	}
	return r
}

func (r *rig) sell(t *testing.T) {
	t.Helper()
	if _, err := r.tm.RecordSale(r.ctx, r.user, nil, terminal.SaleInput{
		Lines:    []terminal.CartLine{{VariantID: r.item.VariantID, QtyMilli: 1000}},
		Payments: []terminal.PaymentInput{{MethodID: r.cash.ID, AmountMinor: 250000}},
	}); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) hubSales() int {
	var n int
	_ = r.h.DB().R().QueryRow("SELECT count(*) FROM sales").Scan(&n)
	return n
}

func TestShopTemplateApplied(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	st := map[string]string{}
	for _, k := range []string{"business_type", "tax_mode", "default_tax_bp", "units", "product_fields"} {
		st[k], _ = r.h.Setting(r.ctx, k)
	}
	if st["business_type"] != "grocery" || st["tax_mode"] != "inclusive" || st["default_tax_bp"] != "750" {
		t.Fatalf("grocery settings: %v", st)
	}
	var units []string
	_ = json.Unmarshal([]byte(st["units"]), &units)
	if len(units) < 5 || units[1] != "kg" {
		t.Fatalf("units %v", units)
	}
	var cats int
	_ = r.h.DB().R().QueryRow("SELECT count(*) FROM categories").Scan(&cats)
	if cats < 9 {
		t.Fatalf("grocery categories: %d", cats)
	}
	// Switching to pharmacy adds what's missing, removes nothing, keeps tax.
	res, err := r.h.ApplyTemplate(r.ctx, hub.SystemSession, "pharmacy", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Categories) != 6 || len(res.PaymentMethods) != 1 || res.PaymentMethods[0] != "HMO / Insurance" {
		t.Fatalf("pharmacy added %+v", res)
	}
	var after int
	_ = r.h.DB().R().QueryRow("SELECT count(*) FROM categories").Scan(&after)
	if after != cats+6 {
		t.Fatalf("categories %d -> %d", cats, after)
	}
	if v, _ := r.h.Setting(r.ctx, "tax_mode"); v != "inclusive" {
		t.Fatalf("tax mode changed to %s without asking", v)
	}
	// The till receives the new product details.
	waitFor(t, "fields reach till", 5*time.Second, func() bool {
		return bytes.Contains([]byte(r.tm.Settings(r.ctx)["product_fields"]), []byte("nafdac_no"))
	})
	if err := r.h.SaveSettings(r.ctx, hub.SystemSession, map[string]string{"units": "[]"}); err == nil {
		t.Fatal("empty unit list accepted")
	}
}

func TestBackupCopyAndRestore(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	if _, err := r.tm.OpenShift(r.ctx, r.user, 0); err != nil {
		t.Fatal(err)
	}
	r.sell(t)
	waitFor(t, "first sale at hub", 5*time.Second, func() bool { return r.hubSales() == 1 && r.tm.Status().Pending == 0 })

	// Only on the hub PC: a warning until a second location is set.
	if st := r.h.BackupStatus(r.ctx); st.Warning == "" {
		t.Fatal("no warning before any backup")
	}
	usb := t.TempDir()
	if err := r.h.SaveBackupSettings(r.ctx, hub.SystemSession, filepath.Join(usb, "missing"), 14); err == nil {
		t.Fatal("a folder that doesn't exist was accepted")
	}
	if err := r.h.SaveBackupSettings(r.ctx, hub.SystemSession, usb, 14); err != nil {
		t.Fatal(err)
	}
	backup, err := r.h.Backup(r.ctx)
	if err != nil {
		t.Fatal(err)
	}
	st := r.h.BackupStatus(r.ctx)
	if st.Warning != "" || st.CopyLastAt == 0 || len(st.Files) != 2 {
		t.Fatalf("after backup: warning=%q copies=%d files=%d", st.Warning, st.CopyLastAt, len(st.Files))
	}
	copies, _ := filepath.Glob(filepath.Join(usb, "Zebiventor backups", "*", "hub-*.db"))
	if len(copies) != 1 {
		t.Fatalf("copies on the second drive: %v", copies)
	}

	// Two more sales reach the hub after the backup.
	time.Sleep(20 * time.Millisecond)
	r.sell(t)
	r.sell(t)
	waitFor(t, "later sales at hub", 5*time.Second, func() bool { return r.hubSales() == 3 && r.tm.Status().Pending == 0 })

	// The hub PC's disk "fails": restore from the copy on the second drive.
	cur, _ := r.h.Setting(r.ctx, "hub_id")
	info, err := hub.InspectBackup(r.ctx, copies[0], cur)
	if err != nil || info.Problem != "" || !info.SameStore || info.Sales != 1 {
		t.Fatalf("inspect: %+v %v", info, err)
	}
	if _, err := r.h.SafetyBackup(r.ctx); err != nil {
		t.Fatal(err)
	}
	r.h.Close()
	if err := hub.RestoreFile(r.ctx, r.dir, copies[0]); err != nil {
		t.Fatal(err)
	}
	if r.h, err = hub.Open(r.ctx, hub.Config{DataDir: r.dir, Version: "test", Logger: quiet}); err != nil {
		t.Fatal(err)
	}
	r.sw.set(r.h.Handler())
	r.srv.CloseClientConnections() // as the old hub's server shutting down would
	if n := r.hubSales(); n != 1 {
		t.Fatalf("restored hub has %d sales, want 1", n)
	}
	// The till notices, re-sends the two sales the backup was missing, and
	// downloads everything again.
	waitFor(t, "till re-sends after restore", 10*time.Second, func() bool { return r.hubSales() == 3 && r.tm.Status().Pending == 0 })
	if r.tm.Status().Resent < 2 {
		t.Fatalf("resent %d", r.tm.Status().Resent)
	}
	var stock int64
	_ = r.h.DB().R().QueryRow("SELECT qty_milli FROM stock_levels").Scan(&stock)
	if stock != 47_000 {
		t.Fatalf("hub stock after catch-up %d, want 47000", stock)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal(err)
	}
	// A file that isn't a backup is refused.
	junk := filepath.Join(t.TempDir(), "junk.db")
	_ = os.WriteFile(junk, []byte("not a database"), 0o644)
	if info, _ := hub.InspectBackup(r.ctx, junk, cur); info.Problem == "" {
		t.Fatal("junk accepted as a backup")
	}
}

func TestReceiveTransferAtBranchOffline(t *testing.T) {
	var branch, main string
	r := newRig(t, "test", "test", func(h *hub.Hub) string {
		main, _ = h.Setting(context.Background(), "default_location_id")
		branch, _ = h.SaveLocation(context.Background(), hub.SystemSession, protocol.Location{Name: "Ikeja branch", Active: true})
		return branch
	})
	_, err := r.h.TransferStock(r.ctx, hub.SystemSession, hub.TransferInput{FromLocationID: main, ToLocationID: branch,
		Lines: []protocol.StockTransferLine{{VariantID: r.item.VariantID, QtyMilli: 5_000}}})
	if err != nil {
		t.Fatal(err)
	}
	var incoming []terminal.IncomingTransfer
	waitFor(t, "transfer reaches branch till", 5*time.Second, func() bool {
		incoming, _ = r.tm.IncomingTransfers(r.ctx)
		return len(incoming) == 1
	})
	tr := incoming[0]
	if tr.Lines[0].QtyMilli != 5_000 || tr.ToLocationName != "Ikeja branch" {
		t.Fatalf("incoming %+v", tr)
	}

	// The branch's internet is down when the van arrives: 4 of 5 arrive.
	r.sw.down.Store(true)
	cashier := terminal.User{ID: "x", Role: protocol.RoleCashier}
	if err := r.tm.ReceiveTransfer(r.ctx, cashier, tr.ID, protocol.TransferReceipt{}); err == nil {
		t.Fatal("a cashier received stock")
	}
	receipt := protocol.TransferReceipt{Note: "one carton damp", Lines: []protocol.StockTransferLine{{VariantID: r.item.VariantID, ReceivedQtyMilli: 4_000}}}
	if err := r.tm.ReceiveTransfer(r.ctx, r.user, tr.ID, receipt); err != nil {
		t.Fatal(err)
	}
	if err := r.tm.ReceiveTransfer(r.ctx, r.user, tr.ID, receipt); err == nil {
		t.Fatal("received twice")
	}
	incoming, _ = r.tm.IncomingTransfers(r.ctx)
	if len(incoming) != 1 || !incoming[0].ReceivedHere || r.tm.Status().Pending != 1 {
		t.Fatalf("offline: %+v pending=%d", incoming, r.tm.Status().Pending)
	}

	// Back online: the hub books it in.
	r.sw.down.Store(false)
	r.tm.Nudge()
	waitFor(t, "receipt reaches hub", 10*time.Second, func() bool {
		got, _ := r.h.GetTransfer(r.ctx, hub.SystemSession, tr.ID)
		return got != nil && got.Status == protocol.TransferReceived
	})
	got, _ := r.h.GetTransfer(r.ctx, hub.SystemSession, tr.ID)
	if got.Lines[0].ReceivedQtyMilli != 4_000 || got.ReceiveNote != "one carton damp" {
		t.Fatalf("hub transfer %+v", got)
	}
	var atBranch int64
	_ = r.h.DB().R().QueryRow("SELECT qty_milli FROM stock_levels WHERE location_id=?", branch).Scan(&atBranch)
	if atBranch != 4_000 {
		t.Fatalf("branch stock %d", atBranch)
	}
	waitFor(t, "till drops received transfer", 5*time.Second, func() bool {
		incoming, _ = r.tm.IncomingTransfers(r.ctx)
		return len(incoming) == 0
	})
}

func TestVersionCheckAndLANUpdate(t *testing.T) {
	r := newRig(t, "0.4.0", "0.3.9", nil)

	// An update loaded into the hub (by hand: a developer build accepts unsigned).
	installer := bytes.Repeat([]byte("installer bytes "), 50_000)
	sum := sha256.Sum256(installer)
	m := update.Manifest{Version: "0.4.0", Platform: terminal.Platform(), File: "zebiventor-0.4.0-setup.exe",
		SHA256: hex.EncodeToString(sum[:]), Size: int64(len(installer))}
	raw, _ := json.Marshal(m)
	var pkg bytes.Buffer
	if err := update.BuildPackage(&pkg, raw, nil, m.File, bytes.NewReader(installer)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.h.LoadUpdatePackage(r.ctx, hub.SystemSession, &pkg); err != nil {
		t.Fatal(err)
	}
	r.tm.Nudge()
	waitFor(t, "till learns of update", 5*time.Second, func() bool {
		s := r.tm.Status()
		return s.HubVersion == "0.4.0" && s.UpdateVersion == "0.4.0"
	})
	var got []int64
	path, man, err := r.tm.DownloadUpdate(r.ctx, t.TempDir(), func(done, total int64) { got = append(got, done) })
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !bytes.Equal(b, installer) || man.Version != "0.4.0" || len(got) == 0 || got[len(got)-1] != int64(len(installer)) {
		t.Fatalf("download: %d bytes, progress %v", len(b), got)
	}

	// A till on another sync protocol is told clearly, but can still update.
	code, _ := r.h.CreatePairingCode(r.ctx, "", protocol.PairingCodeRequest{})
	pair, err := r.h.Pair(r.ctx, "127.0.0.1", protocol.PairRequest{Code: code.Code, TerminalName: "Future till"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(path string) (*http.Response, protocol.APIError) {
		req, _ := http.NewRequest(http.MethodGet, r.srv.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+pair.Token)
		req.Header.Set("X-Zebi-Protocol", strconv.Itoa(protocol.Version+1))
		req.Header.Set("X-Zebi-Platform", terminal.Platform())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var e protocol.APIError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return resp, e
	}
	resp, e := call("/api/sync/pull?since=0")
	if resp.StatusCode != http.StatusConflict || e.Code != "version" || !bytes.Contains([]byte(e.Error), []byte("Update the hub")) {
		t.Fatalf("mismatch: %d %+v", resp.StatusCode, e)
	}
	if resp.Header.Get("X-Zebi-Update") != "0.4.0" {
		t.Fatalf("update header %q", resp.Header.Get("X-Zebi-Update"))
	}
	if resp, _ := call("/api/update?platform=" + terminal.Platform()); resp.StatusCode != http.StatusOK {
		t.Fatalf("update endpoint refused a mismatched till: %d", resp.StatusCode)
	}
	// The hub recorded the real till's version.
	list, _ := r.h.ListTerminals(r.ctx)
	if list[0].AppVersion != "0.3.9" {
		t.Fatalf("till version at hub %q", list[0].AppVersion)
	}
}

// A hub database put back by hand from an older copy (no restore, so no
// new epoch): tills notice the hub is behind them and catch up anyway.
func TestHubPutBackByHand(t *testing.T) {
	r := newRig(t, "test", "test", nil)
	if _, err := r.tm.OpenShift(r.ctx, r.user, 0); err != nil {
		t.Fatal(err)
	}
	r.sell(t)
	waitFor(t, "sale at hub", 5*time.Second, func() bool { return r.hubSales() == 1 && r.tm.Status().Pending == 0 })
	old, err := r.h.Backup(r.ctx)
	if err != nil {
		t.Fatal(err)
	}
	r.sell(t)
	r.sell(t)
	if _, err := r.h.SaveProduct(r.ctx, hub.SystemSession, protocol.ProductInput{
		Product:  protocol.Product{Name: "Milo 500g", TrackStock: true, Unit: "pc"},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 300000}, Barcodes: []string{"222"}}},
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "more at hub", 5*time.Second, func() bool {
		it, _ := r.tm.LookupCode(r.ctx, "222")
		return r.hubSales() == 3 && r.tm.Status().Pending == 0 && it != nil
	})

	r.h.Close()
	for _, sfx := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(filepath.Join(r.dir, "hub.db"+sfx))
	}
	b, _ := os.ReadFile(old)
	if err := os.WriteFile(filepath.Join(r.dir, "hub.db"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if r.h, err = hub.Open(r.ctx, hub.Config{DataDir: r.dir, Version: "test", Logger: quiet}); err != nil {
		t.Fatal(err)
	}
	r.sw.set(r.h.Handler())
	r.srv.CloseClientConnections()
	waitFor(t, "till re-sends to the older hub", 10*time.Second, func() bool { return r.hubSales() == 3 && r.tm.Status().Pending == 0 })
	// The product added after the copy is gone at the hub; the till retires it.
	waitFor(t, "till retires product the hub no longer has", 5*time.Second, func() bool {
		it, _ := r.tm.LookupCode(r.ctx, "222")
		return it == nil
	})
	if it, _ := r.tm.LookupCode(r.ctx, "111"); it == nil {
		t.Fatal("product the hub still has was retired")
	}
	waitFor(t, "till re-downloads", 5*time.Second, func() bool {
		rev := r.tm.Status().Rev
		var hubRev int64
		_ = r.h.DB().R().QueryRow("SELECT seq FROM sqlite_sequence WHERE name='changelog'").Scan(&hubRev)
		return rev == hubRev
	})
}
