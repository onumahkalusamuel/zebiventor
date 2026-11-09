package hub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
)

// BackupFile is one backup on disk.
type BackupFile struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	At    int64  `json:"at"`
	Where string `json:"where"` // "local" (hub PC) or "copy" (second location)
}

// BackupStatus is shown in Settings → Backups and drives the warning banner.
type BackupStatus struct {
	LastAt     int64        `json:"last_at"`
	LastError  string       `json:"last_error"`
	Folder     string       `json:"folder"`
	CopyFolder string       `json:"copy_folder"` // second drive, USB or network folder ("" = none)
	CopyLastAt int64        `json:"copy_last_at"`
	CopyError  string       `json:"copy_error"`
	Keep       int          `json:"keep"`
	Warning    string       `json:"warning"` // non-empty when backups need attention
	Files      []BackupFile `json:"files"`
}

const backupStale = 48 * time.Hour

func (h *Hub) backupFolder() string { return filepath.Join(h.cfg.DataDir, "backups") }

func (h *Hub) settingInt(ctx context.Context, key string) int64 {
	v, _ := h.Setting(ctx, key)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func (h *Hub) backupKeep(ctx context.Context) int {
	if n := h.settingInt(ctx, "backup_keep"); n > 0 {
		return int(n)
	}
	return h.cfg.BackupKeep
}

// copyFolder is where backups are copied for this store, inside the chosen
// folder, so one USB drive can hold several stores' backups.
func (h *Hub) copyFolder(ctx context.Context, base string) string {
	name, _ := h.Setting(ctx, "store_name")
	id, _ := h.Setting(ctx, "hub_id")
	if len(id) > 6 {
		id = id[len(id)-6:]
	}
	return filepath.Join(base, "Zebiventor backups", safeName(name)+" "+id)
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		s = "Store"
	}
	return s
}

func (h *Hub) putPrivate(ctx context.Context, kv map[string]string) {
	_ = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		for k, v := range kv {
			if err := putSetting(ctx, tx, k, v, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// Backup writes a consistent snapshot to the hub PC's backup folder, copies
// it to the second location if one is set, and prunes old backups in both.
// It returns the local file's path.
func (h *Hub) Backup(ctx context.Context) (string, error) {
	h.backupMu.Lock()
	defer h.backupMu.Unlock()
	now := ids.NowMs()
	h.putPrivate(ctx, map[string]string{"backup_attempt_at": fmt.Sprint(now)})

	dir := h.backupFolder()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.putPrivate(ctx, map[string]string{"backup_error": err.Error()})
		return "", err
	}
	name := "hub-" + time.Now().Format("20060102-150405") + ".db"
	dst := filepath.Join(dir, name)
	if err := h.db.Backup(ctx, dst); err != nil {
		h.putPrivate(ctx, map[string]string{"backup_error": err.Error()})
		return "", err
	}
	h.putPrivate(ctx, map[string]string{"last_backup_at": fmt.Sprint(ids.NowMs()), "backup_error": ""})
	h.log.Info("backup written", "path", dst)
	keep := h.backupKeep(ctx)
	prune(dir, keep)

	base, _ := h.Setting(ctx, "backup_copy_dir")
	if base == "" {
		return dst, nil
	}
	target := h.copyFolder(ctx, base)
	err := copyFile(dst, filepath.Join(target, name))
	if err != nil {
		err = fmt.Errorf("the backup was saved on this PC but could not be copied to %s: %w", base, err)
		h.putPrivate(ctx, map[string]string{"backup_copy_error": err.Error()})
		h.log.Error("backup copy failed", "err", err)
		return dst, err
	}
	h.putPrivate(ctx, map[string]string{"backup_copy_at": fmt.Sprint(ids.NowMs()), "backup_copy_error": ""})
	prune(target, keep)
	h.log.Info("backup copied", "path", filepath.Join(target, name))
	return dst, nil
}

func prune(dir string, keep int) {
	matches, _ := filepath.Glob(filepath.Join(dir, "hub-*.db"))
	sort.Strings(matches) // timestamped names sort oldest first
	for len(matches) > keep {
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
}

// copyFile copies src into dst via a temporary file, so a half-written copy
// (USB pulled out) never looks like a backup.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// backupDue says whether maintenance should back up now: once a day, and
// hourly retries while the last attempt failed.
func (h *Hub) backupDue(ctx context.Context) bool {
	now := ids.NowMs()
	if now-h.settingInt(ctx, "last_backup_at") >= 20*3600*1000 {
		return now-h.settingInt(ctx, "backup_attempt_at") >= 3600*1000 || h.settingInt(ctx, "last_backup_at") == 0
	}
	if base, _ := h.Setting(ctx, "backup_copy_dir"); base != "" {
		if e, _ := h.Setting(ctx, "backup_copy_error"); e != "" {
			return now-h.settingInt(ctx, "backup_attempt_at") >= 3600*1000
		}
	}
	return false
}

// BackupStatus reports the state of backups.
func (h *Hub) BackupStatus(ctx context.Context) BackupStatus {
	st := BackupStatus{Folder: h.backupFolder(), Keep: h.backupKeep(ctx)}
	st.LastAt = h.settingInt(ctx, "last_backup_at")
	st.LastError, _ = h.Setting(ctx, "backup_error")
	st.CopyFolder, _ = h.Setting(ctx, "backup_copy_dir")
	st.CopyLastAt = h.settingInt(ctx, "backup_copy_at")
	st.CopyError, _ = h.Setting(ctx, "backup_copy_error")
	st.Files = listBackups(st.Folder, "local")
	if st.CopyFolder != "" {
		st.Files = append(st.Files, listBackups(h.copyFolder(ctx, st.CopyFolder), "copy")...)
	}
	sort.Slice(st.Files, func(i, j int) bool { return st.Files[i].At > st.Files[j].At })

	stale := ids.NowMs() - backupStale.Milliseconds()
	switch {
	case !h.IsSetup(ctx):
	case st.LastError != "":
		st.Warning = "The last backup failed: " + st.LastError
	case st.LastAt < stale:
		st.Warning = "No backup has been made for over two days."
	case st.CopyFolder == "":
		st.Warning = "Backups are only kept on the hub PC. Choose a second drive, USB stick or network folder so a failed disk doesn't lose the store's records."
	case st.CopyError != "":
		st.Warning = st.CopyError
	case st.CopyLastAt < stale:
		st.Warning = "Backups have not reached " + st.CopyFolder + " for over two days."
	}
	return st
}

func listBackups(dir, where string) []BackupFile {
	var out []BackupFile
	matches, _ := filepath.Glob(filepath.Join(dir, "hub-*.db"))
	for _, p := range matches {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		at := info.ModTime().UnixMilli()
		// The name holds when it was made; a copied file's date doesn't.
		stamp := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "hub-"), ".db")
		if t, err := time.ParseInLocation("20060102-150405", stamp, time.Local); err == nil {
			at = t.UnixMilli()
		}
		out = append(out, BackupFile{Name: filepath.Base(p), Path: p, Size: info.Size(), At: at, Where: where})
	}
	return out
}

// SaveBackupSettings sets the second backup location and how many to keep.
func (h *Hub) SaveBackupSettings(ctx context.Context, by *Session, copyDir string, keep int) error {
	if !by.Can(adminRoles...) {
		return ErrForbidden
	}
	copyDir = strings.TrimSpace(copyDir)
	if keep < 3 || keep > 365 {
		return invalid("keep between 3 and 365 backups")
	}
	if copyDir != "" {
		if !filepath.IsAbs(copyDir) && !strings.HasPrefix(copyDir, `\\`) {
			return invalid("enter the full path of the folder, for example E:\\ or \\\\server\\share")
		}
		if err := checkWritable(copyDir); err != nil {
			return invalid("the hub PC cannot write to %s: %v", copyDir, err)
		}
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		for k, v := range map[string]string{"backup_copy_dir": copyDir, "backup_keep": strconv.Itoa(keep), "backup_copy_error": ""} {
			if err := putSetting(ctx, tx, k, v, now); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "backup.settings", "", "", map[string]any{"copy_dir": copyDir, "keep": keep})
		return nil
	})
}

func checkWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errors.New("the folder does not exist (is the drive plugged in?)")
		}
		return err
	}
	if !info.IsDir() {
		return errors.New("that is a file, not a folder")
	}
	f, err := os.CreateTemp(dir, ".zebiventor-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
