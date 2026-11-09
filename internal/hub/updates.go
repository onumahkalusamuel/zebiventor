package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/update"
)

// UpdatePackage is a release the hub holds for tills to download.
type UpdatePackage = protocol.UpdatePackage

// UpdateStatus is shown in Settings → Updates.
type UpdateStatus struct {
	HubVersion     string          `json:"hub_version"`
	Packages       []UpdatePackage `json:"packages"`
	FeedURL        string          `json:"feed_url"`
	DefaultFeed    string          `json:"default_feed"`
	LastCheckAt    int64           `json:"last_check_at"`
	LastCheckError string          `json:"last_check_error"`
	SigningKey     bool            `json:"signing_key"` // false on developer builds
}

type updateStore struct {
	mu   sync.RWMutex
	pkgs map[string]*UpdatePackage // by platform
}

func (h *Hub) updatesDir() string { return filepath.Join(h.cfg.DataDir, "updates") }

func platformDir(platform string) string { return strings.ReplaceAll(platform, "/", "-") }

// loadUpdates reads the packages kept on disk.
func (h *Hub) loadUpdates() {
	h.upd.mu.Lock()
	defer h.upd.mu.Unlock()
	h.upd.pkgs = map[string]*UpdatePackage{}
	dirs, _ := os.ReadDir(h.updatesDir())
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(h.updatesDir(), d.Name(), "package.json"))
		if err != nil {
			continue
		}
		var p UpdatePackage
		if json.Unmarshal(b, &p) == nil && p.Manifest.Validate() == nil {
			h.upd.pkgs[p.Manifest.Platform] = &p
		}
	}
}

// AvailableUpdate returns the package held for platform, if any.
func (h *Hub) AvailableUpdate(platform string) (*UpdatePackage, string) {
	h.upd.mu.RLock()
	defer h.upd.mu.RUnlock()
	p := h.upd.pkgs[platform]
	if p == nil {
		return nil, ""
	}
	return p, filepath.Join(h.updatesDir(), platformDir(platform), p.Manifest.File)
}

// storeUpdate keeps a verified release, replacing an older one for the same
// platform. file must already have been checked against m.
func (h *Hub) storeUpdate(m update.Manifest, raw, sig []byte, signed bool, source string, file io.Reader) error {
	if cur, _ := h.AvailableUpdate(m.Platform); cur != nil && update.Compare(m.Version, cur.Manifest.Version) < 0 {
		return invalid("a newer version (%s) is already loaded", cur.Manifest.Version)
	}
	if err := os.MkdirAll(h.updatesDir(), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(h.updatesDir(), ".incoming-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	f, err := os.Create(filepath.Join(tmp, m.File))
	if err != nil {
		return err
	}
	_, err = io.Copy(f, file)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	// Check again what actually landed on disk.
	if f, err = os.Open(filepath.Join(tmp, m.File)); err != nil {
		return err
	}
	err = m.CheckFile(f)
	f.Close()
	if err != nil {
		return err
	}
	p := UpdatePackage{Manifest: m, Raw: string(raw), Signature: string(sig), Signed: signed, StoredAt: ids.NowMs(), Source: source}
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), b, 0o644); err != nil {
		return err
	}
	h.upd.mu.Lock()
	dst := filepath.Join(h.updatesDir(), platformDir(m.Platform))
	old := dst + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			h.upd.mu.Unlock()
			return err
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		h.upd.mu.Unlock()
		return err
	}
	_ = os.RemoveAll(old)
	if h.upd.pkgs == nil {
		h.upd.pkgs = map[string]*UpdatePackage{}
	}
	h.upd.pkgs[m.Platform] = &p
	versions := map[string]string{}
	for k, v := range h.upd.pkgs {
		versions[k] = v.Manifest.Version
	}
	h.upd.mu.Unlock()
	// A synced setting, so tills hear about it with their next pull.
	vb, _ := json.Marshal(versions)
	_ = h.write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		return putSetting(ctx, tx, "available_updates", string(vb), ids.NowMs())
	})
	h.log.Info("update stored", "version", m.Version, "platform", m.Platform, "source", source, "signed", signed)
	return nil
}

// LoadUpdatePackage stores an update package (zip) an admin loaded by hand.
func (h *Hub) LoadUpdatePackage(ctx context.Context, by *Session, r io.Reader) (*UpdatePackage, error) {
	if !by.Can(adminRoles...) {
		return nil, ErrForbidden
	}
	tmp, err := os.CreateTemp(h.cfg.DataDir, "upload-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, err := io.Copy(tmp, r)
	if err != nil {
		return nil, invalid("could not receive the update file: %v", err)
	}
	pkg, err := update.OpenPackage(tmp, size)
	if err != nil {
		return nil, invalid("%v", err)
	}
	rc, err := pkg.File.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if err := h.storeUpdate(pkg.Manifest, pkg.Raw, pkg.Sig, pkg.Signed, "upload", rc); err != nil {
		return nil, err
	}
	_ = h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		audit(ctx, tx, by.UserID, "", "update.load", "", "", map[string]any{"version": pkg.Manifest.Version, "platform": pkg.Manifest.Platform, "signed": pkg.Signed})
		return nil
	})
	p, _ := h.AvailableUpdate(pkg.Manifest.Platform)
	return p, nil
}

func (h *Hub) feedURL(ctx context.Context) string {
	if v, _ := h.Setting(ctx, "update_feed_url"); v != "" {
		return v
	}
	return update.DefaultFeed
}

// UpdateStatus describes the updates the hub holds.
func (h *Hub) UpdateStatus(ctx context.Context) UpdateStatus {
	st := UpdateStatus{HubVersion: h.cfg.Version, DefaultFeed: update.DefaultFeed, SigningKey: update.PublicKey != ""}
	st.FeedURL, _ = h.Setting(ctx, "update_feed_url")
	st.LastCheckError, _ = h.Setting(ctx, "update_last_check_error")
	v, _ := h.Setting(ctx, "update_last_check_at")
	fmt.Sscan(v, &st.LastCheckAt)
	h.upd.mu.RLock()
	for _, p := range h.upd.pkgs {
		st.Packages = append(st.Packages, *p)
	}
	h.upd.mu.RUnlock()
	return st
}

// SaveUpdateFeed sets where the hub looks for releases ("" = built-in default).
func (h *Hub) SaveUpdateFeed(ctx context.Context, by *Session, feed string) error {
	if !by.Can(adminRoles...) {
		return ErrForbidden
	}
	feed = strings.TrimSpace(feed)
	if feed != "" {
		u, err := url.Parse(feed)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return invalid("the update address must start with https://")
		}
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		audit(ctx, tx, by.UserID, "", "update.feed", "", "", map[string]string{"feed_url": feed})
		return putSetting(ctx, tx, "update_feed_url", feed, ids.NowMs())
	})
}

var feedClient = &http.Client{Timeout: 30 * time.Minute}

// CheckForUpdates downloads a newer release from the feed, if there is one.
// It returns the version found ("" if nothing new).
func (h *Hub) CheckForUpdates(ctx context.Context) (string, error) {
	feed := h.feedURL(ctx)
	if feed == "" {
		return "", invalid("no update address is set; load an update file instead")
	}
	found, err := h.checkFeed(ctx, feed)
	msg := ""
	if err != nil {
		msg = err.Error()
		h.log.Warn("update check failed", "feed", feed, "err", err)
	}
	_ = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		if err := putSetting(ctx, tx, "update_last_check_at", fmt.Sprint(now), now); err != nil {
			return err
		}
		return putSetting(ctx, tx, "update_last_check_error", msg, now)
	})
	return found, err
}

func (h *Hub) checkFeed(ctx context.Context, feed string) (string, error) {
	base := strings.TrimSuffix(feed, "/") + "/"
	get := func(name string, limit int64) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+name, nil)
		if err != nil {
			return nil, err
		}
		resp, err := feedClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("could not reach the update site: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("update site answered %s for %s", resp.Status, name)
		}
		return io.ReadAll(io.LimitReader(resp.Body, limit))
	}
	raw, err := get(update.ManifestName, 1<<20)
	if err != nil {
		return "", err
	}
	sig, err := get(update.SignatureName, 4096)
	if err != nil && update.PublicKey != "" {
		return "", err
	}
	m, signed, err := update.ParseManifest(raw, sig)
	if err != nil {
		return "", err
	}
	if cur, _ := h.AvailableUpdate(m.Platform); cur != nil && update.Compare(m.Version, cur.Manifest.Version) <= 0 {
		return "", nil
	}
	if update.Compare(m.Version, h.cfg.Version) <= 0 {
		return "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+url.PathEscape(m.File), nil)
	if err != nil {
		return "", err
	}
	resp, err := feedClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading the update failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update site answered %s for %s", resp.Status, m.File)
	}
	if err := h.storeUpdate(m, raw, sig, signed, "feed", io.LimitReader(resp.Body, m.Size+1)); err != nil {
		var ve validationError
		if errors.As(err, &ve) {
			return "", nil
		}
		return "", err
	}
	return m.Version, nil
}

// maybeCheckForUpdates runs from maintenance: at most every 6 hours.
func (h *Hub) maybeCheckForUpdates(ctx context.Context) {
	if h.feedURL(ctx) == "" {
		return
	}
	var last int64
	v, _ := h.Setting(ctx, "update_last_check_at")
	fmt.Sscan(v, &last)
	if ids.NowMs()-last < 6*3600*1000 {
		return
	}
	_, _ = h.CheckForUpdates(ctx)
}
