package terminal_test

// Load test: a big store (50,000 products, 1 million sales over a year)
// with 5 tills syncing at once. Opt-in, since it takes a few minutes:
//
//	ZEBI_LOADTEST=1 go test ./internal/terminal -run TestLoad -v -timeout 30m
//
// ZEBI_LOAD_PRODUCTS / ZEBI_LOAD_SALES change the sizes; ZEBI_LOAD_REPORT
// writes the timings as Markdown.

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
)

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}

type timing struct {
	what  string
	took  time.Duration
	limit time.Duration
	note  string
}

func TestLoad(t *testing.T) {
	if os.Getenv("ZEBI_LOADTEST") == "" {
		t.Skip("set ZEBI_LOADTEST=1 to run the load test")
	}
	nProducts := envInt("ZEBI_LOAD_PRODUCTS", 50_000)
	nSales := envInt("ZEBI_LOAD_SALES", 1_000_000)
	const nTills = 5
	ctx := context.Background()
	var results []timing
	measure := func(what string, limit time.Duration, fn func() string) {
		t.Helper()
		start := time.Now()
		note := fn()
		took := time.Since(start)
		results = append(results, timing{what, took, limit, note})
		flag := ""
		if limit > 0 && took > limit {
			flag = "  ⚠ SLOW"
		}
		t.Logf("%-58s %10s%s %s", what, took.Round(time.Millisecond), flag, note)
	}

	dir := t.TempDir()
	h, err := hub.Open(ctx, hub.Config{DataDir: dir, Version: "load", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { h.Close() }()
	if err := h.Setup(ctx, hub.SetupInput{StoreName: "Big Store", BusinessType: "grocery", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	loc, _ := h.Setting(ctx, "default_location_id")

	// Pair the tills first (sales need their terminal ids).
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()
	terms := make([]*terminal.Terminal, nTills)
	termIDs := make([]string, nTills)
	for i := range nTills {
		code, _ := h.CreatePairingCode(ctx, "", protocol.PairingCodeRequest{})
		tm, err := terminal.Open(ctx, terminal.Config{DataDir: t.TempDir(), Version: "load", Logger: quiet})
		if err != nil {
			t.Fatal(err)
		}
		defer tm.Close()
		p, err := tm.Pair(ctx, srv.URL, code.Code, fmt.Sprintf("Till %d", i+1))
		if err != nil {
			t.Fatal(err)
		}
		terms[i], termIDs[i] = tm, p.TerminalID
	}

	// ---- bulk data, written straight into the hub database ----
	var variants []string
	var cats []string
	rows, _ := h.DB().R().Query("SELECT id FROM categories")
	for rows.Next() {
		var id string
		rows.Scan(&id)
		cats = append(cats, id)
	}
	rows.Close()
	words := strings.Fields("rice beans milk sugar soap tea coffee oil salt flour bread juice water noodles pasta biscuit cream butter yoghurt cereal tomato pepper onion garlic spice detergent tissue nappy lotion yellow sun gold royal premium classic family")
	rng := rand.New(rand.NewPCG(1, 2))
	measure(fmt.Sprintf("Load %d products (setup only)", nProducts), 0, func() string {
		err := h.DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			now := ids.NowMs()
			insP, _ := tx.PrepareContext(ctx, `INSERT INTO products(id,name,category_id,attributes,unit,track_stock,tax_rate_bp,active,created_at,updated_at) VALUES(?,?,?,?,?,1,750,1,?,?)`)
			insV, _ := tx.PrepareContext(ctx, `INSERT INTO variants(id,product_id,name,sku,price_minor,cost_minor,active,created_at,updated_at) VALUES(?,?,'',?,?,?,1,?,?)`)
			insB, _ := tx.PrepareContext(ctx, `INSERT INTO barcodes(code,variant_id) VALUES(?,?)`)
			insS, _ := tx.PrepareContext(ctx, `INSERT INTO stock_levels(variant_id,location_id,qty_milli,updated_at) VALUES(?,?,?,?)`)
			insC, _ := tx.PrepareContext(ctx, `INSERT INTO changelog(entity,entity_id) VALUES(?,?)`)
			for i := range nProducts {
				pid, vid := ids.New(), ids.New()
				w := words[rng.IntN(len(words))]
				name := fmt.Sprintf("%s%s %s %s %d", strings.ToUpper(w[:1]), w[1:], words[rng.IntN(len(words))], words[rng.IntN(len(words))], i)
				price := int64(100+rng.IntN(5000)) * 100
				if _, err := insP.ExecContext(ctx, pid, name, cats[i%len(cats)], fmt.Sprintf(`{"brand":"Brand %d"}`, i%300), "pc", now, now); err != nil {
					return err
				}
				if _, err := insV.ExecContext(ctx, vid, pid, fmt.Sprintf("SKU%06d", i), price, price*7/10, now, now); err != nil {
					return err
				}
				if _, err := insB.ExecContext(ctx, fmt.Sprintf("600%010d", i), vid); err != nil {
					return err
				}
				if _, err := insS.ExecContext(ctx, vid, loc, 1_000_000, now); err != nil {
					return err
				}
				for _, e := range [][2]string{{protocol.EntityProduct, pid}, {protocol.EntityVariant, vid}, {protocol.EntityBarcode, fmt.Sprintf("600%010d", i)}, {protocol.EntityStockLevel, vid + "|" + loc}} {
					if _, err := insC.ExecContext(ctx, e[0], e[1]); err != nil {
						return err
					}
				}
				variants = append(variants, vid)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return ""
	})

	var methodID string
	h.DB().R().QueryRow("SELECT id FROM payment_methods WHERE kind='cash'").Scan(&methodID)
	yearAgo := time.Now().AddDate(-1, 0, 0).UnixMilli()
	span := time.Now().UnixMilli() - yearAgo - 3600_000
	measure(fmt.Sprintf("Load %d sales over a year (setup only)", nSales), 0, func() string {
		const batch = 50_000
		for done := 0; done < nSales; done += batch {
			err := h.DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				insS, _ := tx.PrepareContext(ctx, `INSERT INTO sales(id,receipt_no,terminal_id,location_id,user_id,user_name,subtotal_minor,tax_minor,total_minor,paid_minor,created_at,received_at,tax_mode)
					VALUES(?,?,?,?,'u','Cashier',?,?,?,?,?,?,'inclusive')`)
				insL, _ := tx.PrepareContext(ctx, `INSERT INTO sale_lines(id,sale_id,line_no,variant_id,product_name,sku,qty_milli,unit_price_minor,tax_minor,total_minor,cost_minor)
					VALUES(?,?,?,?,?,?,1000,?,?,?,?)`)
				insP, _ := tx.PrepareContext(ctx, `INSERT INTO sale_payments(id,sale_id,method_id,method_name,kind,amount_minor) VALUES(?,?,?,'Cash','cash',?)`)
				insM, _ := tx.PrepareContext(ctx, `INSERT INTO stock_movements(id,variant_id,location_id,qty_milli,reason,ref_type,ref_id,created_at,received_at) VALUES(?,?,?,-1000,'sale','sale',?,?,?)`)
				insI, _ := tx.PrepareContext(ctx, `INSERT INTO inbox(op_id,terminal_id,kind,status,received_at) VALUES(?,?,'sale.create','applied',?)`)
				for n := done; n < min(done+batch, nSales); n++ {
					at := yearAgo + int64(n)*span/int64(nSales)
					sid, term := ids.New(), n%nTills
					type line struct {
						v     string
						price int64
					}
					lines := make([]line, 1+rng.IntN(3))
					var total int64
					for l := range lines {
						lines[l] = line{variants[rng.IntN(len(variants))], int64(1000 + rng.IntN(9000)*10)}
						total += lines[l].price
					}
					if _, err := insS.ExecContext(ctx, sid, fmt.Sprintf("L%02d-%07d", term+1, n), termIDs[term], loc, total, total*750/10750, total, total, at, at); err != nil {
						return err
					}
					for l, ln := range lines {
						if _, err := insL.ExecContext(ctx, ids.New(), sid, l+1, ln.v, "Item", "SKU", ln.price, ln.price*750/10750, ln.price, ln.price*7/10); err != nil {
							return err
						}
						if _, err := insM.ExecContext(ctx, ids.New(), ln.v, loc, sid, at, at); err != nil {
							return err
						}
					}
					if _, err := insP.ExecContext(ctx, ids.New(), sid, methodID, total); err != nil {
						return err
					}
					if _, err := insI.ExecContext(ctx, ids.New(), termIDs[term], at); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return ""
	})
	measure("Build report summaries for existing sales (one-time, on upgrade)", 0, func() string {
		if err := h.RebuildSummaries(ctx); err != nil {
			t.Fatal(err)
		}
		return ""
	})
	h.DB().Write(ctx, func(ctx context.Context, tx *sql.Tx) error { _, err := tx.ExecContext(ctx, "ANALYZE"); return err })
	fi, _ := os.Stat(filepath.Join(dir, "hub.db"))
	wal, _ := os.Stat(filepath.Join(dir, "hub.db-wal"))
	size := fi.Size()
	if wal != nil {
		size += wal.Size()
	}
	t.Logf("hub database: %.0f MB", float64(size)/1e6)

	// ---- tills: first sync of the whole catalogue ----
	measure(fmt.Sprintf("First sync of %d products to 5 tills at once", nProducts), 3*time.Minute, func() string {
		for _, tm := range terms {
			tm.StartSync()
		}
		last := fmt.Sprintf("600%010d", nProducts-1)
		deadline := time.Now().Add(15 * time.Minute)
		for _, tm := range terms {
			for {
				if it, _ := tm.LookupCode(ctx, last); it != nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("tills did not finish the first sync")
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
		return ""
	})

	tm := terms[0]
	measure("Till: barcode scan (100 lookups)", 500*time.Millisecond, func() string {
		for i := range 100 {
			if it, _ := tm.LookupCode(ctx, fmt.Sprintf("600%010d", (i*977)%nProducts)); it == nil {
				t.Fatal("barcode not found")
			}
		}
		return "per scan: see total/100"
	})
	measure("Till: search \"yellow rice\" (20 searches)", time.Second, func() string {
		var n int
		for range 20 {
			r, err := tm.SearchItems(ctx, "yellow rice", 30)
			if err != nil {
				t.Fatal(err)
			}
			n = len(r)
		}
		return fmt.Sprintf("%d results", n)
	})
	measure("Till: search by brand attribute (20 searches)", time.Second, func() string {
		var n int
		for range 20 {
			r, _ := tm.SearchItems(ctx, "brand 123", 30)
			n = len(r)
		}
		return fmt.Sprintf("%d results", n)
	})

	// ---- 5 tills selling at once ----
	admin, err := tm.Login(ctx, "admin", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	const perTill = 200
	measure(fmt.Sprintf("5 tills × %d sales, recorded and synced", perTill), 30*time.Second, func() string {
		var wg sync.WaitGroup
		var slowest time.Duration
		var mu sync.Mutex
		for i, tmi := range terms {
			if _, err := tmi.OpenShift(ctx, admin, 0); err != nil {
				t.Fatal(err)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := rand.New(rand.NewPCG(uint64(i), 9))
				for range perTill {
					it, _ := tmi.LookupCode(ctx, fmt.Sprintf("600%010d", r.IntN(nProducts)))
					start := time.Now()
					_, err := tmi.RecordSale(ctx, admin, nil, terminal.SaleInput{
						Lines:    []terminal.CartLine{{VariantID: it.VariantID, QtyMilli: 1000}},
						Payments: []terminal.PaymentInput{{MethodID: methodID, AmountMinor: it.PriceMinor * 2}},
					})
					if err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					slowest = max(slowest, time.Since(start))
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		deadline := time.Now().Add(5 * time.Minute)
		for _, tmi := range terms {
			for tmi.Status().Pending > 0 {
				if time.Now().After(deadline) {
					t.Fatal("sales did not sync")
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return fmt.Sprintf("slowest sale at the till %s", slowest.Round(time.Millisecond))
	})

	// ---- hub: reports over a year of sales ----
	sys := hub.SystemSession
	now := time.Now()
	// Whole days, as the app's period picker sends them.
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := midnight.UnixMilli()
	end := midnight.AddDate(0, 0, 1).UnixMilli()
	month := midnight.AddDate(0, 0, -29).UnixMilli()
	year := midnight.AddDate(-1, 0, 0).UnixMilli()
	report := func(name string, from int64, group string) {
		measure("Report: "+name, 2*time.Second, func() string {
			r, err := h.SalesReport(ctx, sys, from, end, group)
			if err != nil {
				t.Fatal(err)
			}
			return fmt.Sprintf("%d rows, %d sales", len(r.Rows), r.Totals.Transactions)
		})
	}
	report("sales today by hour", today, "hour")
	report("last 30 days by day", month, "day")
	report("last 30 days by item", month, "product")
	report("last 30 days by cashier", month, "cashier")
	report("last 12 months by day", year, "day")
	report("last 12 months by category", year, "category")
	report("last 12 months by item", year, "product")
	measure("Report: last 12 months by day, uneven times (no summaries)", 30*time.Second, func() string {
		r, _ := h.SalesReport(ctx, sys, year+1, end-1, "day")
		return fmt.Sprintf("%d rows, %d sales", len(r.Rows), r.Totals.Transactions)
	})
	measure("Report: stock levels (all items)", 2*time.Second, func() string {
		r, err := h.StockReport(ctx, sys, hub.StockReportFilter{})
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d rows", len(r.Rows))
	})
	measure("Report: stock search \"yellow\"", time.Second, func() string {
		r, _ := h.StockReport(ctx, sys, hub.StockReportFilter{Query: "yellow"})
		return fmt.Sprintf("%d rows", len(r.Rows))
	})
	measure("Report: shifts, last 30 days", time.Second, func() string {
		r, _ := h.ShiftReport(ctx, sys, month, end)
		return fmt.Sprintf("%d rows", len(r))
	})
	measure("Hub: look up a receipt for a return", 200*time.Millisecond, func() string {
		_, err := h.LookupSale(ctx, fmt.Sprintf("L03-%07d", nSales/2/nTills*nTills+2))
		if err != nil {
			t.Fatal(err)
		}
		return ""
	})
	measure("Hub: dashboard counts", 500*time.Millisecond, func() string {
		_, err := h.Summary(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ""
	})
	measure("Hub: stock history of one item", 500*time.Millisecond, func() string {
		ms, err := h.StockMovements(ctx, sys, variants[42], loc, 200)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d movements", len(ms))
	})
	measure("Hub: backup (full copy)", time.Minute, func() string {
		p, err := h.Backup(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fi, _ := os.Stat(p)
		return fmt.Sprintf("%.0f MB", float64(fi.Size())/1e6)
	})

	if out := os.Getenv("ZEBI_LOAD_REPORT"); out != "" {
		var b strings.Builder
		fmt.Fprintf(&b, "| What | Time | Target | Notes |\n|---|---:|---:|---|\n")
		sort.SliceStable(results, func(i, j int) bool { return false })
		for _, r := range results {
			lim := "–"
			if r.limit > 0 {
				lim = "< " + r.limit.String()
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.what, r.took.Round(time.Millisecond), lim, r.note)
		}
		fmt.Fprintf(&b, "\nHub database: %.0f MB.\n", float64(size)/1e6)
		_ = os.WriteFile(out, []byte(b.String()), 0o644)
	}
	for _, r := range results {
		if r.limit > 0 && r.took > r.limit {
			t.Errorf("too slow: %s took %s (target %s)", r.what, r.took.Round(time.Millisecond), r.limit)
		}
	}
}
