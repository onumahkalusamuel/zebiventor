package terminal_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// switchable lets a test take the hub "offline" without closing the server.
type switchable struct {
	h    http.Handler
	down atomic.Bool
}

func (s *switchable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.down.Load() {
		// Simulate an unreachable host by hijacking and dropping the connection.
		if hj, ok := w.(http.Hijacker); ok {
			c, _, _ := hj.Hijack()
			c.Close()
			return
		}
	}
	s.h.ServeHTTP(w, r)
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestFiveTillsOfflineFirst(t *testing.T) {
	ctx := context.Background()

	h, err := hub.Open(ctx, hub.Config{DataDir: t.TempDir(), Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Setup(ctx, hub.SetupInput{StoreName: "Test Books", BusinessType: "bookshop", AdminName: "Ada",
		AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	sw := &switchable{h: h.Handler()}
	srv := httptest.NewServer(sw)
	defer srv.Close()

	// Catalogue: a book with opening stock 100 and a service that isn't stock-tracked.
	bookID, err := h.SaveProduct(ctx, hub.SystemSession, protocol.ProductInput{
		Product:  protocol.Product{Name: "Things Fall Apart", TrackStock: true, Attributes: map[string]string{"isbn": "9780385474542", "author": "Chinua Achebe"}},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 350000}, Barcodes: []string{"9780385474542"}, OpeningQtyMilli: 100_000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = bookID

	// Pair five tills.
	const tills = 5
	terms := make([]*terminal.Terminal, tills)
	for i := range tills {
		code, err := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
		if err != nil {
			t.Fatal(err)
		}
		tm, err := terminal.Open(ctx, terminal.Config{DataDir: t.TempDir(), Version: "test", Logger: quiet})
		if err != nil {
			t.Fatal(err)
		}
		defer tm.Close()
		p, err := tm.Pair(ctx, srv.URL, code.Code, fmt.Sprintf("Till %d", i+1))
		if err != nil {
			t.Fatal(err)
		}
		if p.ReceiptPrefix != fmt.Sprintf("T%02d", i+1) {
			t.Fatalf("prefix %s", p.ReceiptPrefix)
		}
		tm.StartSync()
		terms[i] = tm
	}

	// A used code cannot be reused.
	code, _ := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
	extra, _ := terminal.Open(ctx, terminal.Config{DataDir: t.TempDir(), Logger: quiet})
	defer extra.Close()
	if _, err := extra.Pair(ctx, srv.URL, code.Code, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := extra.Pair(ctx, srv.URL, code.Code, "y"); err == nil {
		t.Fatal("reused pairing code was accepted")
	}

	// Every till receives catalogue, users and payment methods.
	for i, tm := range terms {
		waitFor(t, fmt.Sprintf("till %d catalogue", i+1), 5*time.Second, func() bool {
			it, _ := tm.LookupCode(ctx, "9780385474542")
			return it != nil && it.StockMilli == 100_000 && tm.HasUsers(ctx)
		})
	}
	admin, err := terms[0].Login(ctx, "admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !terms[0].HasHubSession() {
		t.Fatal("expected a hub session while online")
	}
	methods, _ := terms[0].PaymentMethods(ctx)
	var cash, transfer protocol.PaymentMethod
	for _, m := range methods {
		switch m.Kind {
		case protocol.PayCash:
			cash = m
		case protocol.PayTransfer:
			transfer = m
		}
	}
	item, _ := terms[0].LookupCode(ctx, "9780385474542")

	// Search by author (an attribute) works.
	found, _ := terms[0].SearchItems(ctx, "achebe", 10)
	if len(found) != 1 {
		t.Fatalf("search by author: %d results", len(found))
	}

	sell := func(tm *terminal.Terminal, n int) {
		var wg sync.WaitGroup
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := tm.RecordSale(ctx, admin, terminal.SaleInput{
					Lines:    []terminal.CartLine{{VariantID: item.VariantID, QtyMilli: 1000}},
					Payments: []terminal.PaymentInput{{MethodID: cash.ID, AmountMinor: 500000}},
				})
				if err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	}

	// Phase 1: all five tills sell concurrently while online.
	var wg sync.WaitGroup
	for _, tm := range terms {
		wg.Add(1)
		go func() { defer wg.Done(); sell(tm, 4) }()
	}
	wg.Wait()
	waitFor(t, "online sales to sync", 10*time.Second, func() bool {
		for _, tm := range terms {
			if tm.Status().Pending != 0 {
				return false
			}
		}
		return true
	})

	// Phase 2: hub goes down; tills keep selling locally.
	sw.down.Store(true)
	for _, tm := range terms {
		wg.Add(1)
		go func() { defer wg.Done(); sell(tm, 3) }()
	}
	wg.Wait()
	for i, tm := range terms {
		if p := tm.Status().Pending; p != 3 {
			t.Fatalf("till %d: %d pending while offline, want 3", i+1, p)
		}
	}
	// Local stock view already reflects the till's own unsynced sales.
	it, _ := terms[0].LookupCode(ctx, "9780385474542")
	if it.StockMilli > (100-20-3)*1000 {
		t.Fatalf("offline stock view %d did not subtract local sales", it.StockMilli)
	}

	// Validation still applies offline.
	if _, err := terms[0].RecordSale(ctx, admin, terminal.SaleInput{
		Lines:    []terminal.CartLine{{VariantID: item.VariantID, QtyMilli: 1000}},
		Payments: []terminal.PaymentInput{{MethodID: transfer.ID, AmountMinor: 350000}},
	}); err == nil {
		t.Fatal("transfer without reference accepted")
	}
	if _, err := terms[0].RecordSale(ctx, admin, terminal.SaleInput{
		Lines:    []terminal.CartLine{{VariantID: item.VariantID, QtyMilli: 1000}},
		Payments: []terminal.PaymentInput{{MethodID: transfer.ID, AmountMinor: 400000, Reference: "x"}},
	}); err == nil {
		t.Fatal("non-cash overpayment accepted")
	}
	// Split payment: part transfer, part cash with change.
	split, err := terms[0].RecordSale(ctx, admin, terminal.SaleInput{
		Lines:    []terminal.CartLine{{VariantID: item.VariantID, QtyMilli: 2000}},
		Payments: []terminal.PaymentInput{{MethodID: transfer.ID, AmountMinor: 500000, Reference: "TRF123"}, {MethodID: cash.ID, AmountMinor: 300000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if split.TotalMinor != 700000 || split.ChangeMinor != 100000 {
		t.Fatalf("split sale total=%d change=%d", split.TotalMinor, split.ChangeMinor)
	}

	// Phase 3: hub returns; everything drains and stock converges everywhere.
	sw.down.Store(false)
	for _, tm := range terms {
		tm.Nudge()
	}
	// 5 tills * (4 online + 3 offline) = 35 units, plus 2 in the split sale.
	const wantStock = (100 - 35 - 2) * 1000
	waitFor(t, "convergence after reconnect", 40*time.Second, func() bool {
		for _, tm := range terms {
			it, _ := tm.LookupCode(ctx, "9780385474542")
			if tm.Status().Pending != 0 || it.StockMilli != wantStock {
				return false
			}
		}
		return true
	})

	var sales, lines, movements int
	var level int64
	r := h.DB().R()
	r.QueryRow("SELECT count(*) FROM sales").Scan(&sales)
	r.QueryRow("SELECT count(*) FROM sale_lines").Scan(&lines)
	r.QueryRow("SELECT count(*) FROM stock_movements WHERE reason='sale'").Scan(&movements)
	r.QueryRow("SELECT qty_milli FROM stock_levels").Scan(&level)
	if sales != 36 || lines != 36 || movements != 36 || level != wantStock {
		t.Fatalf("hub: sales=%d lines=%d movements=%d level=%d", sales, lines, movements, level)
	}

	// Idempotency: re-sending already-applied ops must not double count.
	if err := terms[0].DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE outbox SET status='pending'")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	terms[0].Nudge()
	waitFor(t, "re-push", 10*time.Second, func() bool { return terms[0].Status().Pending == 0 })
	r.QueryRow("SELECT count(*) FROM sales").Scan(&sales)
	r.QueryRow("SELECT qty_milli FROM stock_levels").Scan(&level)
	if sales != 36 || level != wantStock {
		t.Fatalf("after duplicate push: sales=%d level=%d", sales, level)
	}

	// Admin round trip through the hub: price change reaches every till.
	var pd []terminal.ProductDetail
	pd, _ = terms[0].ListProducts(ctx, "things", 10)
	in := protocol.ProductInput{Product: pd[0].Product}
	v := pd[0].Variants[0]
	v.PriceMinor = 400000
	in.Variants = []protocol.VariantInput{{Variant: v.Variant, Barcodes: v.Barcodes}}
	if err := terms[0].Admin(ctx, http.MethodPost, "/api/admin/products", in, nil); err != nil {
		t.Fatal(err)
	}
	for i, tm := range terms {
		waitFor(t, fmt.Sprintf("till %d price update", i+1), 5*time.Second, func() bool {
			it, _ := tm.LookupCode(ctx, "9780385474542")
			return it != nil && it.PriceMinor == 400000
		})
	}

	// A revoked till stops syncing and says so.
	terminals, _ := h.ListTerminals(ctx)
	if err := h.RevokeTerminal(ctx, hub.SystemSession, terminals[4].ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "revocation noticed", 40*time.Second, func() bool { return terms[4].Status().Revoked })
}
