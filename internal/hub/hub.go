// Package hub is the store's system of record. One hub runs per shop, either
// headless on a dedicated PC (cmd/zebihub, optionally as an OS service) or
// embedded inside one of the tills. Terminals sell from their own local
// database and sync with the hub whenever it is reachable.
package hub

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/store"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DefaultPort is the TCP port the hub listens on.
const DefaultPort = 7420

// Config configures a hub.
type Config struct {
	DataDir string
	Version string
	Logger  *slog.Logger
	// BackupKeep is how many daily backups to retain (default 14).
	BackupKeep int
}

// Hub is the central store server.
type Hub struct {
	cfg    Config
	db     *store.DB
	log    *slog.Logger
	notify *notifier

	seenMu sync.Mutex
	seen   map[string]time.Time // terminal id -> last persisted last_seen_at

	pairLimiter  *limiter
	loginLimiter *limiter

	bgCancel context.CancelFunc
	bgWG     sync.WaitGroup
}

// Open opens (creating and migrating if needed) the hub database in cfg.DataDir.
func Open(ctx context.Context, cfg Config) (*Hub, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.BackupKeep <= 0 {
		cfg.BackupKeep = 14
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "hub.db"), store.Options{})
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, migrations, "migrations"); err != nil {
		db.Close()
		return nil, err
	}
	h := &Hub{
		cfg:          cfg,
		db:           db,
		log:          cfg.Logger.With("component", "hub"),
		notify:       newNotifier(),
		seen:         map[string]time.Time{},
		pairLimiter:  newLimiter(10, time.Minute),
		loginLimiter: newLimiter(20, time.Minute),
	}
	bg, cancel := context.WithCancel(context.Background())
	h.bgCancel = cancel
	h.bgWG.Add(1)
	go h.maintenance(bg)
	return h, nil
}

// Close stops background work and closes the database.
func (h *Hub) Close() error {
	h.bgCancel()
	h.bgWG.Wait()
	return h.db.Close()
}

// DB exposes the underlying store (used by tests and tools).
func (h *Hub) DB() *store.DB { return h.db }

// write runs fn on the writer queue and wakes long-polling terminals on success.
func (h *Hub) write(ctx context.Context, fn store.TxFunc) error {
	err := h.db.Write(ctx, fn)
	if err == nil {
		h.notify.broadcast()
	}
	return err
}

// ---------- setup ----------

// SetupInput is the first-run configuration of a store.
type SetupInput struct {
	StoreName     string `json:"store_name"`
	BusinessType  string `json:"business_type"` // general, bookshop, grocery, pharmacy, distributor...
	LocationName  string `json:"location_name"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	CurrencyCode  string `json:"currency_code"`
	CurrencySym   string `json:"currency_symbol"`
	AdminName     string `json:"admin_name"`
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

// IsSetup reports whether first-run setup has been completed.
func (h *Hub) IsSetup(ctx context.Context) bool {
	v, _ := h.Setting(ctx, "setup_complete")
	return v == "1"
}

// Setup performs first-run initialisation. It fails if already set up.
func (h *Hub) Setup(ctx context.Context, in SetupInput) error {
	in.StoreName = strings.TrimSpace(in.StoreName)
	in.AdminUsername = strings.TrimSpace(in.AdminUsername)
	if in.StoreName == "" || in.AdminUsername == "" || in.AdminName == "" {
		return errors.New("store name, admin name and admin username are required")
	}
	if len(in.AdminPassword) < 6 {
		return errors.New("admin password must be at least 6 characters")
	}
	if in.LocationName == "" {
		in.LocationName = "Main Store"
	}
	if in.BusinessType == "" {
		in.BusinessType = "general"
	}
	if in.CurrencyCode == "" {
		in.CurrencyCode, in.CurrencySym = "NGN", "₦"
	}
	if in.CurrencySym == "" {
		in.CurrencySym = in.CurrencyCode
	}
	pw, err := bcrypt.GenerateFromPassword([]byte(in.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var done string
		_ = tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='setup_complete'").Scan(&done)
		if done == "1" {
			return errors.New("hub is already set up")
		}
		now := ids.NowMs()
		settings := map[string]string{
			"hub_id":          ids.New(),
			"store_name":      in.StoreName,
			"business_type":   in.BusinessType,
			"store_address":   in.Address,
			"store_phone":     in.Phone,
			"store_email":     "",
			"tax_id":          "",
			"currency_code":   in.CurrencyCode,
			"currency_symbol": in.CurrencySym,
			"currency_digits": "2",
			"receipt_header":  "",
			"receipt_footer":  "Thank you for your patronage!",
			"next_sku":        "100001",
		}
		keys := make([]string, 0, len(settings))
		for k := range settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := putSetting(ctx, tx, k, settings[k], now); err != nil {
				return err
			}
		}

		locID := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO locations(id,name,code,active,created_at,updated_at) VALUES(?,?,?,1,?,?)`,
			locID, in.LocationName, "MAIN", now, now); err != nil {
			return err
		}
		if err := recordChange(ctx, tx, protocol.EntityLocation, locID); err != nil {
			return err
		}
		if err := putSetting(ctx, tx, "default_location_id", locID, now); err != nil {
			return err
		}

		adminID := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,name,username,role,password_hash,active,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`,
			adminID, in.AdminName, in.AdminUsername, protocol.RoleAdmin, string(pw), now, now); err != nil {
			return err
		}
		if err := recordChange(ctx, tx, protocol.EntityUser, adminID); err != nil {
			return err
		}

		methods := []protocol.PaymentMethod{
			{Name: "Cash", Kind: protocol.PayCash, OpensDrawer: true, SortOrder: 1},
			{Name: "POS / Card", Kind: protocol.PayCard, SortOrder: 2},
			{Name: "Bank Transfer", Kind: protocol.PayTransfer, RequiresReference: true, SortOrder: 3},
		}
		for _, m := range methods {
			m.ID, m.Active = ids.New(), true
			if err := upsertPaymentMethod(ctx, tx, m, now); err != nil {
				return err
			}
		}

		catID := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO categories(id,name,active,created_at,updated_at) VALUES(?,?,1,?,?)`, catID, "General", now, now); err != nil {
			return err
		}
		if err := recordChange(ctx, tx, protocol.EntityCategory, catID); err != nil {
			return err
		}

		audit(ctx, tx, adminID, "", "hub.setup", "", "", map[string]any{"store_name": in.StoreName})
		return putSetting(ctx, tx, "setup_complete", "1", now)
	})
}

// ---------- settings ----------

// Setting returns a single setting value ("" if missing).
func (h *Hub) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := h.db.R().QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// Info returns public hub information.
func (h *Hub) Info(ctx context.Context) protocol.HubInfo {
	id, _ := h.Setting(ctx, "hub_id")
	name, _ := h.Setting(ctx, "store_name")
	return protocol.HubInfo{
		HubID:           id,
		StoreName:       name,
		Version:         h.cfg.Version,
		ProtocolVersion: protocol.Version,
		SetupComplete:   h.IsSetup(ctx),
		Rev:             h.currentRev(ctx),
		ServerTime:      ids.NowMs(),
	}
}

func (h *Hub) currentRev(ctx context.Context) int64 {
	var rev sql.NullInt64
	_ = h.db.R().QueryRowContext(ctx, "SELECT seq FROM sqlite_sequence WHERE name='changelog'").Scan(&rev)
	return rev.Int64
}

func putSetting(ctx context.Context, tx *sql.Tx, key, value string, now int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, value, now)
	if err != nil {
		return err
	}
	if privateSettings[key] || strings.HasPrefix(key, "seq.") {
		return nil
	}
	return recordChange(ctx, tx, protocol.EntitySetting, key)
}

// privateSettings never leave the hub.
var privateSettings = map[string]bool{"next_sku": true, "last_backup_at": true}

// recordChange bumps the revision of an entity so terminals pull it.
func recordChange(ctx context.Context, tx *sql.Tx, entity, id string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM changelog WHERE entity=? AND entity_id=?", entity, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO changelog(entity, entity_id) VALUES(?,?)", entity, id)
	return err
}

func audit(ctx context.Context, tx *sql.Tx, userID, terminalID, action, entity, entityID string, details any) {
	b, _ := jsonMarshal(details)
	_, _ = tx.ExecContext(ctx, `INSERT INTO audit_log(id,at,user_id,terminal_id,action,entity,entity_id,details) VALUES(?,?,?,?,?,?,?,?)`,
		ids.New(), ids.NowMs(), userID, terminalID, action, entity, entityID, string(b))
}

// ---------- maintenance ----------

func (h *Hub) maintenance(ctx context.Context) {
	defer h.bgWG.Done()
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	h.runMaintenance(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			h.runMaintenance(ctx)
		}
	}
}

func (h *Hub) runMaintenance(ctx context.Context) {
	now := ids.NowMs()
	_ = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM pairing_codes WHERE expires_at < ?", now-24*3600*1000)
		return err
	})
	if !h.IsSetup(ctx) {
		return
	}
	last, _ := h.Setting(ctx, "last_backup_at")
	var lastMs int64
	fmt.Sscan(last, &lastMs)
	if now-lastMs < 20*3600*1000 {
		return
	}
	if err := h.Backup(ctx); err != nil {
		h.log.Error("scheduled backup failed", "err", err)
	}
}

// Backup writes a consistent snapshot to <DataDir>/backups and prunes old ones.
func (h *Hub) Backup(ctx context.Context) error {
	dir := filepath.Join(h.cfg.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dir, "hub-"+time.Now().Format("20060102-150405")+".db")
	if err := h.db.Backup(ctx, dst); err != nil {
		return err
	}
	_ = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return putSetting(ctx, tx, "last_backup_at", fmt.Sprint(ids.NowMs()), ids.NowMs())
	})
	h.log.Info("backup written", "path", dst)

	matches, _ := filepath.Glob(filepath.Join(dir, "hub-*.db"))
	sort.Strings(matches)
	for len(matches) > h.cfg.BackupKeep {
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
	return nil
}

// ---------- notifier ----------

// notifier lets long-polling pulls sleep until the next committed change.
type notifier struct {
	mu sync.Mutex
	ch chan struct{}
}

func newNotifier() *notifier { return &notifier{ch: make(chan struct{})} }

func (n *notifier) wait() <-chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.ch
}

func (n *notifier) broadcast() {
	n.mu.Lock()
	close(n.ch)
	n.ch = make(chan struct{})
	n.mu.Unlock()
}

// ---------- rate limiter ----------

// limiter is a fixed-window failure counter keyed by client address.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().Add(-l.window)
	h := l.hits[key]
	for len(h) > 0 && h[0].Before(cut) {
		h = h[1:]
	}
	l.hits[key] = h
	return len(h) >= l.max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	l.hits[key] = append(l.hits[key], time.Now())
	l.mu.Unlock()
}
