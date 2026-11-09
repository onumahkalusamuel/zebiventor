package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Status is the till's sync state, shown in the UI status bar.
type Status struct {
	Paired     bool   `json:"paired"`
	Online     bool   `json:"online"`
	Revoked    bool   `json:"revoked"`
	HubURL     string `json:"hub_url"`
	LastSyncAt int64  `json:"last_sync_at"`
	LastError  string `json:"last_error"`
	Pending    int    `json:"pending"`  // ops not yet at the hub
	Rejected   int    `json:"rejected"` // ops the hub refused (needs attention)
	Rev        int64  `json:"rev"`
	// Versions: the hub's, a newer one the hub holds for this till, and a
	// message when the two can't sync until one is updated.
	HubVersion    string `json:"hub_version"`
	UpdateVersion string `json:"update_version"`
	VersionError  string `json:"version_error"`
	// Restored: the hub was restored from a backup and this till re-sent
	// what the backup was missing (count of records re-sent).
	Resent int `json:"resent"`
}

// Status returns the current sync status.
func (t *Terminal) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func (t *Terminal) updateStatus(fn func(*Status)) {
	t.mu.Lock()
	before := t.status
	fn(&t.status)
	after := t.status
	t.mu.Unlock()
	if before != after && t.cfg.OnStatus != nil {
		t.cfg.OnStatus(after)
	}
}

func (t *Terminal) refreshCounts(ctx context.Context) {
	var pending, rejected int
	t.db.R().QueryRowContext(ctx, "SELECT count(*) FROM outbox WHERE status='pending'").Scan(&pending)
	t.db.R().QueryRowContext(ctx, "SELECT count(*) FROM outbox WHERE status='rejected'").Scan(&rejected)
	rev, _ := strconv.ParseInt(t.kv(ctx, "last_rev"), 10, 64)
	t.updateStatus(func(s *Status) { s.Pending, s.Rejected, s.Rev = pending, rejected, rev })
}

// Nudge wakes the pusher (e.g. right after a sale is recorded).
func (t *Terminal) Nudge() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// StartSync launches the background push and pull loops. Safe to call again
// after pairing; a running syncer is restarted.
func (t *Terminal) StartSync() {
	t.StopSync()
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.syncStop = cancel
	t.mu.Unlock()
	t.syncWG.Add(2)
	go t.pullLoop(ctx)
	go t.pushLoop(ctx)
}

// StopSync stops the background loops and waits for them.
func (t *Terminal) StopSync() {
	t.mu.Lock()
	stop := t.syncStop
	t.syncStop = nil
	t.mu.Unlock()
	if stop != nil {
		stop()
		t.syncWG.Wait()
	}
}

type backoff struct{ n int }

func (b *backoff) next() time.Duration {
	b.n++
	d := time.Second << min(b.n-1, 4) // 1s,2s,4s,8s, then 10s: LAN outages are usually short
	return min(d, 10*time.Second)
}
func (b *backoff) reset() { b.n = 0 }

func sleep(ctx context.Context, d time.Duration) bool {
	tm := time.NewTimer(d)
	defer tm.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-tm.C:
		return true
	}
}

func (t *Terminal) onSyncError(ctx context.Context, err error, fails int) {
	if isVersionMismatch(err) {
		t.updateStatus(func(s *Status) { s.Online, s.VersionError, s.LastError = true, err.Error(), err.Error() })
		return
	}
	if isTerminalRevoked(err) {
		t.updateStatus(func(s *Status) {
			s.Online, s.Revoked, s.LastError = false, true, "This till was disconnected from the hub. Ask an admin to pair it again."
		})
		return
	}
	t.updateStatus(func(s *Status) { s.Online, s.LastError = !IsOffline(err), err.Error() })
	// The hub may have moved to a new IP (DHCP). Look for it by id.
	if IsOffline(err) && fails > 0 && fails%3 == 0 && t.cfg.Discover != nil {
		p, _ := t.Pairing(ctx)
		if url, ok := t.cfg.Discover(ctx, p.HubID); ok && url != p.HubURL {
			t.log.Info("hub found at new address", "url", url)
			_ = t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error { return setKV(ctx, tx, "hub_url", url) })
			t.updateStatus(func(s *Status) { s.HubURL = url })
		}
	}
}

func (t *Terminal) markOnline() {
	t.updateStatus(func(s *Status) {
		s.Online, s.Revoked, s.LastError, s.VersionError, s.LastSyncAt = true, false, "", "", ids.NowMs()
	})
}

// ---------- pull ----------

func (t *Terminal) pullLoop(ctx context.Context) {
	defer t.syncWG.Done()
	var bo backoff
	fails := 0
	for ctx.Err() == nil {
		p, ok := t.Pairing(ctx)
		if !ok {
			if !sleep(ctx, 2*time.Second) {
				return
			}
			continue
		}
		since, _ := strconv.ParseInt(t.kv(ctx, "last_rev"), 10, 64)
		// Don't long-poll on the first round (startup, after an error, or
		// initial download) so the status turns "online" immediately.
		wait := 25
		if since == 0 || !t.Status().Online || fails > 0 {
			wait = 0
		}
		var resp protocol.PullResponse
		err := t.client.do(ctx, time.Duration(wait+15)*time.Second, http.MethodGet,
			fmt.Sprintf("%s/api/sync/pull?since=%d&wait=%d", p.HubURL, since, wait), p.token, "", nil, &resp)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fails++
			t.onSyncError(ctx, err, fails)
			if isTerminalRevoked(err) || isVersionMismatch(err) {
				if !sleep(ctx, time.Minute) {
					return
				}
				continue
			}
			if !sleep(ctx, bo.next()) {
				return
			}
			continue
		}
		fails = 0
		bo.reset()
		if t.hubRestored(ctx) {
			continue // start again from the beginning
		}
		if resp.Rev > since {
			if err := t.applyPull(ctx, &resp); err != nil {
				t.log.Error("applying pulled changes failed", "err", err)
				t.updateStatus(func(s *Status) { s.LastError = "could not save data from hub: " + err.Error() })
				if !sleep(ctx, bo.next()) {
					return
				}
				continue
			}
		}
		t.markOnline()
		t.updateStatus(func(s *Status) { s.Rev = resp.Rev })
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// applyPull upserts everything in resp and advances last_rev atomically.
func (t *Terminal) applyPull(ctx context.Context, r *protocol.PullResponse) error {
	return t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		exec := func(q string, args ...any) error {
			_, err := tx.ExecContext(ctx, q, args...)
			return err
		}
		for _, s := range r.Settings {
			if err := exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", s.Key, s.Value); err != nil {
				return err
			}
			if s.Key == "store_name" {
				if err := setKV(ctx, tx, "store_name", s.Value); err != nil {
					return err
				}
			}
		}
		for _, l := range r.Locations {
			if err := exec(`INSERT INTO locations(id,name,code,active) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, code=excluded.code, active=excluded.active`, l.ID, l.Name, l.Code, boolInt(l.Active)); err != nil {
				return err
			}
		}
		for _, u := range r.Users {
			if err := exec(`INSERT INTO users(id,name,username,role,password_hash,pin_hash,active) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, username=excluded.username, role=excluded.role, password_hash=excluded.password_hash,
				pin_hash=excluded.pin_hash, active=excluded.active`, u.ID, u.Name, u.Username, u.Role, u.PasswordHash, u.PinHash, boolInt(u.Active)); err != nil {
				return err
			}
		}
		for _, c := range r.Categories {
			if err := exec(`INSERT INTO categories(id,parent_id,name,active) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET
				parent_id=excluded.parent_id, name=excluded.name, active=excluded.active`, c.ID, c.ParentID, c.Name, boolInt(c.Active)); err != nil {
				return err
			}
		}
		for _, p := range r.Products {
			attrs, _ := json.Marshal(p.Attributes)
			if err := exec(`INSERT INTO products(id,name,category_id,description,attributes,unit,allow_fraction,track_stock,tax_rate_bp,active,updated_at,search)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, category_id=excluded.category_id,
				description=excluded.description, attributes=excluded.attributes, unit=excluded.unit, allow_fraction=excluded.allow_fraction,
				track_stock=excluded.track_stock, tax_rate_bp=excluded.tax_rate_bp, active=excluded.active, updated_at=excluded.updated_at, search=excluded.search`,
				p.ID, p.Name, p.CategoryID, p.Description, string(attrs), p.Unit, boolInt(p.AllowFraction), boolInt(p.TrackStock),
				p.TaxRateBP, boolInt(p.Active), p.UpdatedAt, searchText(p)); err != nil {
				return err
			}
		}
		for _, v := range r.Variants {
			if err := exec(`INSERT INTO variants(id,product_id,name,sku,price_minor,cost_minor,active,reorder_level_milli) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
				product_id=excluded.product_id, name=excluded.name, sku=excluded.sku, price_minor=excluded.price_minor,
				cost_minor=excluded.cost_minor, active=excluded.active, reorder_level_milli=excluded.reorder_level_milli`,
				v.ID, v.ProductID, v.Name, v.SKU, v.PriceMinor, v.CostMinor, boolInt(v.Active), v.ReorderLevelMilli); err != nil {
				return err
			}
		}
		for _, b := range r.Barcodes {
			if err := exec(`INSERT INTO barcodes(code,variant_id,pack_qty_milli,deleted) VALUES(?,?,?,?) ON CONFLICT(code) DO UPDATE SET
				variant_id=excluded.variant_id, pack_qty_milli=excluded.pack_qty_milli, deleted=excluded.deleted`,
				b.Code, b.VariantID, b.PackQtyMilli, boolInt(b.Deleted)); err != nil {
				return err
			}
		}
		for _, m := range r.PaymentMethods {
			if err := exec(`INSERT INTO payment_methods(id,name,kind,requires_reference,opens_drawer,sort_order,active) VALUES(?,?,?,?,?,?,?)
				ON CONFLICT(id) DO UPDATE SET name=excluded.name, kind=excluded.kind, requires_reference=excluded.requires_reference,
				opens_drawer=excluded.opens_drawer, sort_order=excluded.sort_order, active=excluded.active`,
				m.ID, m.Name, m.Kind, boolInt(m.RequiresReference), boolInt(m.OpensDrawer), m.SortOrder, boolInt(m.Active)); err != nil {
				return err
			}
		}
		for _, s := range r.StockLevels {
			if err := exec(`INSERT INTO stock_levels(variant_id,location_id,qty_milli) VALUES(?,?,?) ON CONFLICT(variant_id,location_id) DO UPDATE SET
				qty_milli=excluded.qty_milli`, s.VariantID, s.LocationID, s.QtyMilli); err != nil {
				return err
			}
		}
		for _, c := range r.Customers {
			// Details: last write wins (a newer local edit is still queued).
			// Balance: always the hub's.
			if err := exec(`INSERT INTO customers(id,name,phone,email,notes,active,balance_minor,updated_at,search) VALUES(?,?,?,?,?,?,?,?,?)
				ON CONFLICT(id) DO UPDATE SET balance_minor=excluded.balance_minor,
				name=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.name ELSE customers.name END,
				phone=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.phone ELSE customers.phone END,
				email=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.email ELSE customers.email END,
				notes=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.notes ELSE customers.notes END,
				active=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.active ELSE customers.active END,
				search=CASE WHEN excluded.updated_at>=customers.updated_at THEN excluded.search ELSE customers.search END,
				updated_at=max(excluded.updated_at, customers.updated_at)`,
				c.ID, c.Name, c.Phone, c.Email, c.Notes, boolInt(c.Active), c.BalanceMinor, c.UpdatedAt, customerSearch(c)); err != nil {
				return err
			}
		}
		if err := saveTransfers(ctx, tx, r.Transfers); err != nil {
			return err
		}
		return setKV(ctx, tx, "last_rev", strconv.FormatInt(r.Rev, 10))
	})
}

func searchText(p protocol.Product) string {
	parts := []string{p.Name, p.Description}
	for _, v := range p.Attributes {
		parts = append(parts, v)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// ---------- push ----------

func (t *Terminal) pushLoop(ctx context.Context) {
	defer t.syncWG.Done()
	var bo backoff
	fails := 0
	for ctx.Err() == nil {
		n, err := t.pushOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fails++
			t.onSyncError(ctx, err, fails)
			// Back off, but a new sale (Nudge) retries immediately.
			wait := bo.next()
			if isVersionMismatch(err) {
				wait = time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-t.wake:
			case <-time.After(wait):
			}
			continue
		}
		fails = 0
		bo.reset()
		if n > 0 {
			continue // more may be waiting
		}
		select {
		case <-ctx.Done():
			return
		case <-t.wake:
		case <-time.After(15 * time.Second):
		}
	}
}

// pushOnce sends up to 100 pending ops and records the hub's verdicts.
func (t *Terminal) pushOnce(ctx context.Context) (int, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return 0, nil
	}
	rows, err := t.db.R().QueryContext(ctx, "SELECT id,kind,payload,created_at FROM outbox WHERE status='pending' ORDER BY id LIMIT 100")
	if err != nil {
		return 0, err
	}
	var ops []protocol.Op
	for rows.Next() {
		var op protocol.Op
		var payload string
		if err := rows.Scan(&op.ID, &op.Kind, &payload, &op.CreatedAt); err != nil {
			rows.Close()
			return 0, err
		}
		op.Payload = json.RawMessage(payload)
		ops = append(ops, op)
	}
	rows.Close()
	if len(ops) == 0 {
		t.refreshCounts(ctx)
		return 0, nil
	}

	var resp protocol.PushResponse
	pushErr := t.client.do(ctx, 60*time.Second, http.MethodPost, p.HubURL+"/api/sync/push", p.token, "", protocol.PushRequest{Ops: ops}, &resp)
	// The hub may have applied some ops before failing; record whatever came back.
	if len(resp.Results) > 0 {
		now := ids.NowMs()
		err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			for _, r := range resp.Results {
				switch r.Status {
				case protocol.OpApplied, protocol.OpDuplicate:
					if _, err := tx.ExecContext(ctx, "UPDATE outbox SET status='sent', sent_at=?, attempts=attempts+1, last_error='' WHERE id=?", now, r.ID); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, `UPDATE sales SET synced_at=? WHERE id=(SELECT ref_id FROM outbox WHERE id=? AND kind=?)`,
						now, r.ID, protocol.OpSaleCreate); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, `UPDATE returns SET synced_at=? WHERE id=(SELECT ref_id FROM outbox WHERE id=? AND kind=?)`,
						now, r.ID, protocol.OpReturnCreate); err != nil {
						return err
					}
					if _, err := tx.ExecContext(ctx, `UPDATE account_payments SET synced_at=? WHERE id=(SELECT ref_id FROM outbox WHERE id=? AND kind=?)`,
						now, r.ID, protocol.OpAccountPayment); err != nil {
						return err
					}
				case protocol.OpRejected:
					if _, err := tx.ExecContext(ctx, "UPDATE outbox SET status='rejected', attempts=attempts+1, last_error=? WHERE id=?", r.Error, r.ID); err != nil {
						return err
					}
					// A refused receipt can be done again (or by the hub).
					if _, err := tx.ExecContext(ctx, `UPDATE transfers SET received_locally=0 WHERE id=(SELECT ref_id FROM outbox WHERE id=? AND kind=?)`,
						r.ID, protocol.OpTransferIn); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	if pushErr != nil {
		_ = t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "UPDATE outbox SET attempts=attempts+1, last_error=? WHERE status='pending' AND id=?", pushErr.Error(), ops[0].ID)
			return err
		})
		t.refreshCounts(ctx)
		return 0, pushErr
	}
	t.markOnline()
	t.refreshCounts(ctx)
	return len(resp.Results), nil
}

// RetryRejected moves rejected ops back to pending (after the cause is fixed at the hub).
func (t *Terminal) RetryRejected(ctx context.Context) error {
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE outbox SET status='pending' WHERE status='rejected'")
		return err
	})
	t.refreshCounts(ctx)
	t.Nudge()
	return err
}
