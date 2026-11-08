// Package terminal is the till side of the system. Every sale is committed
// to the till's own SQLite database first (so selling never waits on the
// network) together with an outbox entry; a background syncer pushes the
// outbox to the hub and pulls master data (catalogue, prices, users, stock)
// down whenever the hub is reachable.
package terminal

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/store"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Config configures a terminal.
type Config struct {
	DataDir  string
	Version  string
	Logger   *slog.Logger
	OnStatus func(Status) // called (from a goroutine) whenever sync status changes
	// Discover, if set, is used to re-locate the hub (by id) when its address changes.
	Discover func(ctx context.Context, hubID string) (url string, ok bool)
}

// Pairing is this till's identity at the hub.
type Pairing struct {
	HubURL        string `json:"hub_url"`
	HubID         string `json:"hub_id"`
	StoreName     string `json:"store_name"`
	TerminalID    string `json:"terminal_id"`
	TerminalName  string `json:"terminal_name"`
	ReceiptPrefix string `json:"receipt_prefix"`
	LocationID    string `json:"location_id"`
	token         string
}

// User is a signed-in staff member.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Terminal is a till.
type Terminal struct {
	cfg    Config
	db     *store.DB
	log    *slog.Logger
	client *client

	mu      sync.Mutex
	status  Status
	session string // hub session token for admin calls (memory only)

	wake     chan struct{}
	syncStop context.CancelFunc
	syncWG   sync.WaitGroup
}

// Open opens (creating if needed) the till database in cfg.DataDir.
func Open(ctx context.Context, cfg Config) (*Terminal, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "terminal.db"), store.Options{})
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, migrations, "migrations"); err != nil {
		db.Close()
		return nil, err
	}
	t := &Terminal{
		cfg:    cfg,
		db:     db,
		log:    cfg.Logger.With("component", "terminal"),
		client: newClient(),
		wake:   make(chan struct{}, 1),
	}
	if p, ok := t.Pairing(ctx); ok {
		t.status.Paired = true
		t.status.HubURL = p.HubURL
	}
	t.refreshCounts(ctx)
	return t, nil
}

// DB exposes the local store.
func (t *Terminal) DB() *store.DB { return t.db }

// Close stops syncing and closes the database.
func (t *Terminal) Close() error {
	t.StopSync()
	return t.db.Close()
}

func (t *Terminal) kv(ctx context.Context, key string) string {
	var v string
	_ = t.db.R().QueryRowContext(ctx, "SELECT value FROM kv WHERE key=?", key).Scan(&v)
	return v
}

func setKV(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO kv(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

// Pairing returns the stored pairing, if any.
func (t *Terminal) Pairing(ctx context.Context) (Pairing, bool) {
	p := Pairing{
		HubURL:        t.kv(ctx, "hub_url"),
		HubID:         t.kv(ctx, "hub_id"),
		StoreName:     t.kv(ctx, "store_name"),
		TerminalID:    t.kv(ctx, "terminal_id"),
		TerminalName:  t.kv(ctx, "terminal_name"),
		ReceiptPrefix: t.kv(ctx, "receipt_prefix"),
		LocationID:    t.kv(ctx, "location_id"),
		token:         t.kv(ctx, "token"),
	}
	return p, p.token != "" && p.HubURL != ""
}

// HubInfo fetches public info from a hub URL (used before pairing).
func (t *Terminal) HubInfo(ctx context.Context, hubURL string) (protocol.HubInfo, error) {
	var info protocol.HubInfo
	err := t.client.do(ctx, 5*time.Second, http.MethodGet, NormalizeHubURL(hubURL)+"/api/hub/info", "", "", nil, &info)
	return info, err
}

// Pair redeems a pairing code at the hub and stores the resulting identity.
// Pairing a till that already belongs to a different hub is refused, since
// it would mix two stores' data in one database.
func (t *Terminal) Pair(ctx context.Context, hubURL, code, name string) (Pairing, error) {
	hubURL = NormalizeHubURL(hubURL)
	if hubURL == "" {
		return Pairing{}, errors.New("enter the hub address")
	}
	info, err := t.HubInfo(ctx, hubURL)
	if err != nil {
		return Pairing{}, err
	}
	if !info.SetupComplete {
		return Pairing{}, errors.New("that hub has not been set up yet")
	}
	if prev := t.kv(ctx, "hub_id"); prev != "" && prev != info.HubID {
		return Pairing{}, errors.New("this till already belongs to a different store; reset it before pairing elsewhere")
	}
	var resp protocol.PairResponse
	err = t.client.do(ctx, 10*time.Second, http.MethodPost, hubURL+"/api/pair", "", "",
		protocol.PairRequest{Code: strings.TrimSpace(code), TerminalName: name, AppVersion: t.cfg.Version}, &resp)
	if err != nil {
		return Pairing{}, err
	}
	err = t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		for k, v := range map[string]string{
			"hub_url": hubURL, "hub_id": resp.HubID, "store_name": resp.StoreName, "terminal_id": resp.TerminalID,
			"terminal_name": resp.TerminalName, "receipt_prefix": resp.ReceiptPrefix, "location_id": resp.LocationID, "token": resp.Token,
		} {
			if err := setKV(ctx, tx, k, v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Pairing{}, err
	}
	t.updateStatus(func(s *Status) { s.Paired, s.Revoked, s.HubURL, s.LastError = true, false, hubURL, "" })
	p, _ := t.Pairing(ctx)
	return p, nil
}

// SetHubURL changes the hub address (e.g. after the hub PC got a new IP).
func (t *Terminal) SetHubURL(ctx context.Context, hubURL string) error {
	hubURL = NormalizeHubURL(hubURL)
	p, ok := t.Pairing(ctx)
	if !ok {
		return errors.New("this till is not paired")
	}
	info, err := t.HubInfo(ctx, hubURL)
	if err != nil {
		return err
	}
	if info.HubID != p.HubID {
		return errors.New("that address belongs to a different hub")
	}
	if err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error { return setKV(ctx, tx, "hub_url", hubURL) }); err != nil {
		return err
	}
	t.updateStatus(func(s *Status) { s.HubURL = hubURL })
	t.Nudge()
	return nil
}

// ---------- users ----------

var ErrBadLogin = errors.New("incorrect username or password")

// Login verifies credentials against the locally synced user list, so staff
// can sign in while the hub is offline. When the hub is reachable it also
// opens a hub session used for admin actions.
func (t *Terminal) Login(ctx context.Context, username, secret string) (User, error) {
	var u User
	var pw, pin string
	var active bool
	err := t.db.R().QueryRowContext(ctx, `SELECT id,name,username,role,password_hash,pin_hash,active FROM users WHERE username=?`,
		strings.TrimSpace(username)).Scan(&u.ID, &u.Name, &u.Username, &u.Role, &pw, &pin, &active)
	if err != nil || !active || secret == "" {
		return User{}, ErrBadLogin
	}
	okPw := pw != "" && bcrypt.CompareHashAndPassword([]byte(pw), []byte(secret)) == nil
	okPin := !okPw && pin != "" && bcrypt.CompareHashAndPassword([]byte(pin), []byte(secret)) == nil
	if !okPw && !okPin {
		return User{}, ErrBadLogin
	}

	t.mu.Lock()
	t.session = ""
	t.mu.Unlock()
	if p, ok := t.Pairing(ctx); ok {
		var resp protocol.LoginResponse
		err := t.client.do(ctx, 5*time.Second, http.MethodPost, p.HubURL+"/api/auth/login", p.token, "",
			protocol.LoginRequest{Username: username, Password: secret}, &resp)
		if err == nil {
			t.mu.Lock()
			t.session = resp.SessionToken
			t.mu.Unlock()
		} else {
			t.log.Info("hub session unavailable; continuing offline", "err", err)
		}
	}
	return u, nil
}

// Logout drops the hub session.
func (t *Terminal) Logout(ctx context.Context) {
	t.mu.Lock()
	sess := t.session
	t.session = ""
	t.mu.Unlock()
	if p, ok := t.Pairing(ctx); ok && sess != "" {
		_ = t.client.do(ctx, 3*time.Second, http.MethodPost, p.HubURL+"/api/auth/logout", p.token, sess, nil, nil)
	}
}

// HasHubSession reports whether admin actions are currently possible.
func (t *Terminal) HasHubSession() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.session != ""
}

// ErrNeedsHub is returned for admin actions while the hub is unreachable.
var ErrNeedsHub = errors.New("this action needs a connection to the hub; check the network, then sign out and in again")

// Admin performs an authenticated admin call at the hub.
func (t *Terminal) Admin(ctx context.Context, method, path string, body, out any) error {
	p, ok := t.Pairing(ctx)
	if !ok {
		return errors.New("this till is not paired")
	}
	t.mu.Lock()
	sess := t.session
	t.mu.Unlock()
	if sess == "" {
		return ErrNeedsHub
	}
	err := t.client.do(ctx, 15*time.Second, method, p.HubURL+path, p.token, sess, body, out)
	if IsOffline(err) {
		return ErrNeedsHub
	}
	if err == nil {
		t.Nudge() // pull the change straight back
	}
	return err
}

// GetKV reads a till-local setting.
func (t *Terminal) GetKV(ctx context.Context, key string) string { return t.kv(ctx, "local."+key) }

// SetKV stores a till-local setting (e.g. printer configuration).
func (t *Terminal) SetKV(ctx context.Context, key, value string) error {
	return t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error { return setKV(ctx, tx, "local."+key, value) })
}
