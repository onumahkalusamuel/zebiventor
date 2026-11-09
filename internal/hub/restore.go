package hub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
)

// BackupInfo describes a backup file before it is restored.
type BackupInfo struct {
	Path          string `json:"path"`
	StoreName     string `json:"store_name"`
	HubID         string `json:"hub_id"`
	SameStore     bool   `json:"same_store"`
	TakenAt       int64  `json:"taken_at"` // last activity recorded in it
	Products      int64  `json:"products"`
	Sales         int64  `json:"sales"`
	LastSaleAt    int64  `json:"last_sale_at"`
	SchemaVersion int    `json:"schema_version"`
	Problem       string `json:"problem"` // non-empty: cannot be restored
}

// SchemaVersion is the newest hub database version this build understands.
func SchemaVersion() int {
	entries, _ := fs.ReadDir(migrations, "migrations")
	v := 0
	for _, e := range entries {
		num, _, _ := strings.Cut(e.Name(), "_")
		if n, err := strconv.Atoi(num); err == nil && n > v {
			v = n
		}
	}
	return v
}

// InspectBackup checks a backup file and summarises what it holds.
// currentHubID is the running store's id ("" if none).
func InspectBackup(ctx context.Context, path, currentHubID string) (BackupInfo, error) {
	info := BackupInfo{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		return info, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(ON)")
	if err != nil {
		return info, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		info.Problem = "This is not a Zebiventor backup (or it is damaged)."
		return info, nil
	}
	if check != "ok" {
		info.Problem = "This backup is damaged: " + check
		return info, nil
	}
	_ = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&info.SchemaVersion)
	setting := func(k string) string {
		var v string
		_ = db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", k).Scan(&v)
		return v
	}
	info.HubID, info.StoreName = setting("hub_id"), setting("store_name")
	if info.HubID == "" || setting("setup_complete") != "1" {
		info.Problem = "This is not a Zebiventor store backup."
		return info, nil
	}
	if info.SchemaVersion > SchemaVersion() {
		info.Problem = "This backup was made by a newer version of Zebiventor. Update this PC first."
		return info, nil
	}
	info.SameStore = currentHubID == "" || currentHubID == info.HubID
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM products").Scan(&info.Products)
	_ = db.QueryRowContext(ctx, "SELECT count(*), COALESCE(max(created_at),0) FROM sales").Scan(&info.Sales, &info.LastSaleAt)
	_ = db.QueryRowContext(ctx, "SELECT COALESCE(max(received_at),0) FROM inbox").Scan(&info.TakenAt)
	if t := st.ModTime().UnixMilli(); info.TakenAt == 0 || info.TakenAt > t {
		info.TakenAt = t
	}
	return info, nil
}

// SafetyBackup snapshots the current data before a restore replaces it.
func (h *Hub) SafetyBackup(ctx context.Context) (string, error) {
	dir := h.backupFolder()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "before-restore-"+ids.New()[:8]+".db")
	return dst, h.db.Backup(ctx, dst)
}

// RestoreFile replaces the hub database in dataDir with a backup. The hub
// must be closed. The restored store gets a new data epoch: tills notice,
// download everything again and re-send what the backup is missing.
func RestoreFile(ctx context.Context, dataDir, backup string) error {
	info, err := InspectBackup(ctx, backup, "")
	if err != nil {
		return err
	}
	if info.Problem != "" {
		return errors.New(info.Problem)
	}
	live := filepath.Join(dataDir, "hub.db")
	tmp := live + ".restoring"
	_ = os.Remove(tmp)
	if err := copyFile(backup, tmp); err != nil {
		return fmt.Errorf("copying the backup: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+tmp)
	if err != nil {
		return err
	}
	epoch := fmt.Sprintf("%s:%d", ids.New(), info.TakenAt)
	_, err = db.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES('data_epoch',?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, epoch, ids.NowMs())
	if err == nil {
		// Restored sessions may belong to people who have since signed out.
		_, err = db.ExecContext(ctx, "DELETE FROM sessions")
	}
	if cerr := db.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("preparing the backup: %w", err)
	}
	for _, sfx := range []string{"-wal", "-shm"} {
		if err := os.Remove(live + sfx); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("the hub database is still in use (%w); stop the hub first", err)
		}
	}
	return os.Rename(tmp, live)
}

// ensureEpoch gives the store a data epoch if it has none. The epoch is
// "<random id>:<ms>" where ms is the last activity in the restored backup.
func (h *Hub) ensureEpoch(ctx context.Context) error {
	if v, _ := h.Setting(ctx, "data_epoch"); v != "" {
		h.epoch = v
		return nil
	}
	h.epoch = ids.New() + ":0"
	return h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return putSetting(ctx, tx, "data_epoch", h.epoch, ids.NowMs())
	})
}
