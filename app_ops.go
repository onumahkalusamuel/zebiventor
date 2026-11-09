package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/appdata"
	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/templates"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
	"github.com/onumahkalusamuel/zebiventor/internal/update"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- shop templates ----------

// ShopTemplates lists the kinds of shop the set-up can start from.
func (a *App) ShopTemplates() []templates.Template { return templates.All() }

// ---------- stock arriving at this till ----------

// IncomingTransfers lists stock on its way to this till's location (works offline).
func (a *App) IncomingTransfers() ([]terminal.IncomingTransfer, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.IncomingTransfers(a.ctx)
}

// ReceiveTransferHere books a transfer in at this till; it reaches the hub
// with the next sync, so it works while the hub is unreachable.
func (a *App) ReceiveTransferHere(id string, in protocol.TransferReceipt) error {
	u, err := a.currentUser()
	if err != nil {
		return err
	}
	return a.term.ReceiveTransfer(a.ctx, u, id, in)
}

// TillLocations lists where this till can send stock (works offline).
func (a *App) TillLocations() ([]terminal.Location, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.Locations(a.ctx)
}

// SendTransferHere sends stock from this till's location (queued, works offline).
func (a *App) SendTransferHere(in terminal.SendTransferInput) (*protocol.StockTransfer, error) {
	u, err := a.currentUser()
	if err != nil {
		return nil, err
	}
	return a.term.SendTransfer(a.ctx, u, in)
}

// OutgoingTransfers lists transfers sent here that the hub hasn't numbered yet.
func (a *App) OutgoingTransfers() ([]protocol.StockTransfer, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.OutgoingTransfers(a.ctx)
}

// ExpectedDeliveries lists purchase orders due at this till's location (works offline).
func (a *App) ExpectedDeliveries() ([]terminal.ExpectedDelivery, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.ExpectedDeliveries(a.ctx)
}

// ReceiveDeliveryHere books in a delivery against a purchase order (queued, works offline).
func (a *App) ReceiveDeliveryHere(poID string, in terminal.DeliveryInput) error {
	u, err := a.currentUser()
	if err != nil {
		return err
	}
	return a.term.ReceiveDelivery(a.ctx, u, poID, in)
}

// ---------- product pictures ----------

// assetHandler serves product pictures to the screens at /img/<hash>.
func (a *App) assetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash, ok := strings.CutPrefix(r.URL.Path, "/img/")
		if !ok || a.term == nil {
			http.NotFound(w, r)
			return
		}
		data, err := a.term.ProductImage(r.Context(), hash)
		if err != nil || data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		_, _ = w.Write(data)
	})
}

// SetProductImage uploads a picture (base64) for a product; "" removes it.
func (a *App) SetProductImage(productID, base64Data string) (string, error) {
	if _, err := a.currentUser(protocol.RoleAdmin, protocol.RoleManager, protocol.RoleInventory); err != nil {
		return "", err
	}
	path := "/api/admin/products/" + url.PathEscape(productID) + "/image"
	if base64Data == "" {
		return "", a.term.Admin(a.ctx, http.MethodDelete, path, nil, nil)
	}
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", errors.New("the picture could not be read")
	}
	var out struct {
		ImageHash string `json:"image_hash"`
	}
	if err := a.term.AdminUpload(a.ctx, path, bytes.NewReader(data), &out); err != nil {
		return "", err
	}
	a.term.Nudge()
	return out.ImageHash, nil
}

// ---------- backups & restore (on the PC that runs the hub) ----------

// ChooseFolder asks for a folder (a second drive, USB stick or network share).
func (a *App) ChooseFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}

func (a *App) localHub() (*hub.Hub, error) {
	if _, err := a.currentUser(protocol.RoleAdmin); err != nil {
		return nil, err
	}
	a.mu.Lock()
	h := a.hub
	a.mu.Unlock()
	if h == nil {
		return nil, errors.New("restoring is done on the PC that runs the hub (on a headless hub: zebihub restore FILE)")
	}
	return h, nil
}

// ChooseBackupFile asks for a backup to restore and describes it.
func (a *App) ChooseBackupFile() (*hub.BackupInfo, error) {
	h, err := a.localHub()
	if err != nil {
		return nil, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose a backup to restore",
		DefaultDirectory: filepath.Join(a.dataDir, "hub", "backups"),
		Filters:          []runtime.FileFilter{{DisplayName: "Zebiventor backups (*.db)", Pattern: "*.db"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.inspect(h, path)
}

// InspectBackupFile describes a backup listed in Settings.
func (a *App) InspectBackupFile(path string) (*hub.BackupInfo, error) {
	h, err := a.localHub()
	if err != nil {
		return nil, err
	}
	return a.inspect(h, path)
}

func (a *App) inspect(h *hub.Hub, path string) (*hub.BackupInfo, error) {
	cur, _ := h.Setting(a.ctx, "hub_id")
	info, err := hub.InspectBackup(a.ctx, path, cur)
	return &info, err
}

// RestoreBackup replaces the store's data with a backup. The current data
// is backed up first. Tills keep selling meanwhile, then catch up.
func (a *App) RestoreBackup(path string) (string, error) {
	h, err := a.localHub()
	if err != nil {
		return "", err
	}
	info, err := a.inspect(h, path)
	if err != nil {
		return "", err
	}
	if info.Problem != "" {
		return "", errors.New(info.Problem)
	}
	safety, err := h.SafetyBackup(a.ctx)
	if err != nil {
		return "", fmt.Errorf("could not save the current data first, so nothing was changed: %w", err)
	}
	a.log.Warn("restoring hub from backup", "backup", path, "safety_copy", safety)
	a.stopHub()
	restoreErr := hub.RestoreFile(a.ctx, filepath.Join(a.dataDir, "hub"), path)
	if restoreErr != nil {
		a.log.Error("restore failed; starting the hub on the current data", "err", restoreErr)
	}
	// The old listener may take a moment to let go of the port.
	var startErr error
	for range 10 {
		if startErr = a.startHub(); startErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if startErr != nil {
		a.mu.Lock()
		a.hubErr = startErr.Error()
		a.mu.Unlock()
	}
	if restoreErr != nil {
		return "", restoreErr
	}
	a.term.Nudge()
	a.log.Info("restore complete", "backup", path)
	return safety, startErr
}

// ---------- moving a store to another PC ----------

// ExportStoreData saves a complete copy of the store (every record and
// picture) to a file the person chooses, to restore on another PC. Any
// admin till can do it; the hub makes the copy.
func (a *App) ExportStoreData() (string, error) {
	if _, err := a.currentUser(protocol.RoleAdmin); err != nil {
		return "", err
	}
	name := safeFileName(a.term.Settings(a.ctx)["store_name"])
	if name == "" {
		name = "Store"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save the store's data",
		DefaultFilename: name + " " + time.Now().Format("2006-01-02") + ".zebi.db",
		Filters:         []runtime.FileFilter{{DisplayName: "Zebiventor store data (*.db)", Pattern: "*.db"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	err = a.term.AdminDownload(a.ctx, "/api/admin/backups/export", f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	a.log.Info("store data exported", "path", path)
	return path, nil
}

// ChooseStoreFile asks for a store data file (export or backup) during
// first-run setup and describes it.
func (a *App) ChooseStoreFile() (*hub.BackupInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose the store's data file",
		Filters: []runtime.FileFilter{{DisplayName: "Zebiventor store data (*.db)", Pattern: "*.db"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	info, err := hub.InspectBackup(a.ctx, path, "")
	return &info, err
}

// SetupFromBackup sets this PC up as the hub of a store restored from a
// data file, and as its first till. Staff sign in with their usual names.
func (a *App) SetupFromBackup(path, terminalName string) error {
	if a.term == nil {
		return errors.New("local database unavailable")
	}
	if _, ok := a.term.Pairing(a.ctx); ok {
		return errors.New("this computer is already set up")
	}
	a.mu.Lock()
	running := a.hub != nil
	a.mu.Unlock()
	if running {
		return errors.New("a store is already open on this computer")
	}
	dir := filepath.Join(a.dataDir, "hub")
	if _, err := os.Stat(filepath.Join(dir, "hub.db")); err == nil {
		return errors.New("this computer already has store data; restore it from Settings → Backups instead")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := hub.RestoreFile(a.ctx, dir, path); err != nil {
		return err
	}
	h, err := a.openHub()
	if err != nil {
		return err
	}
	if err := a.startHub(); err != nil {
		return err
	}
	if terminalName == "" {
		terminalName = "This PC"
	}
	code, err := h.CreatePairingCode(a.ctx, "", protocol.PairingCodeRequest{TerminalName: terminalName})
	if err != nil {
		return err
	}
	if _, err := a.term.Pair(a.ctx, fmt.Sprintf("127.0.0.1:%d", a.cfg.HubPort), code.Code, terminalName); err != nil {
		return err
	}
	a.cfg.Mode = ModeHost
	if err := a.saveConfig(); err != nil {
		return err
	}
	a.log.Info("store restored from data file", "path", path)
	return a.afterPair()
}

// ---------- updates ----------

// UpdateState is the till's view of updates, for the banner and Settings.
type UpdateState struct {
	Current     string `json:"current"`
	Available   string `json:"available"` // newer version the hub holds ("" = none)
	Notes       string `json:"notes"`
	Signed      bool   `json:"signed"`
	Downloading bool   `json:"downloading"`
	Done        int64  `json:"done"`
	Total       int64  `json:"total"`
	Ready       string `json:"ready"` // downloaded installer, verified
	Error       string `json:"error"`
	CanInstall  bool   `json:"can_install"` // this OS can run the installer
}

type updater struct {
	mu sync.Mutex
	st UpdateState
}

func (a *App) updateSnapshot() UpdateState {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	st := a.upd.st
	st.Current, st.CanInstall = appdata.Version, goruntime.GOOS == "windows"
	return st
}

func (a *App) setUpdate(fn func(*UpdateState)) {
	a.upd.mu.Lock()
	fn(&a.upd.st)
	st := a.upd.st
	a.upd.mu.Unlock()
	st.Current, st.CanInstall = appdata.Version, goruntime.GOOS == "windows"
	runtime.EventsEmit(a.ctx, "update:state", st)
}

// GetUpdateState asks the hub what it holds for this till.
func (a *App) GetUpdateState() UpdateState {
	pkg, err := a.term.UpdateOffer(a.ctx)
	a.setUpdate(func(s *UpdateState) {
		if err != nil {
			if !s.Downloading {
				s.Error = err.Error()
			}
			return
		}
		s.Error = ""
		if pkg == nil || update.Compare(pkg.Manifest.Version, appdata.Version) <= 0 {
			s.Available, s.Notes = "", ""
			return
		}
		s.Available, s.Notes, s.Signed = pkg.Manifest.Version, pkg.Manifest.Notes, pkg.Signed
	})
	return a.updateSnapshot()
}

// DownloadUpdate fetches the hub's update in the background; progress
// arrives as "update:state" events.
func (a *App) DownloadUpdate() error {
	if _, err := a.currentUser(protocol.RoleAdmin, protocol.RoleManager); err != nil {
		return err
	}
	a.upd.mu.Lock()
	if a.upd.st.Downloading {
		a.upd.mu.Unlock()
		return nil
	}
	a.upd.st.Downloading, a.upd.st.Error, a.upd.st.Done, a.upd.st.Total = true, "", 0, 0
	a.upd.mu.Unlock()
	go func() {
		last := time.Now()
		path, m, err := a.term.DownloadUpdate(a.ctx, filepath.Join(a.dataDir, "updates"), func(done, total int64) {
			if time.Since(last) > 200*time.Millisecond || done == total {
				last = time.Now()
				a.setUpdate(func(s *UpdateState) { s.Done, s.Total = done, total })
			}
		})
		a.setUpdate(func(s *UpdateState) {
			s.Downloading = false
			if err != nil {
				s.Error = err.Error()
				return
			}
			s.Ready, s.Available = path, m.Version
		})
		if err != nil {
			a.log.Warn("update download failed", "err", err)
		}
	}()
	return nil
}

// InstallUpdate runs the downloaded installer and closes the app; the
// installer starts it again when done. Sales still waiting to sync are kept.
func (a *App) InstallUpdate() error {
	if _, err := a.currentUser(protocol.RoleAdmin, protocol.RoleManager); err != nil {
		return err
	}
	st := a.updateSnapshot()
	if st.Ready == "" {
		return errors.New("download the update first")
	}
	if _, err := os.Stat(st.Ready); err != nil {
		return errors.New("the downloaded update is missing; download it again")
	}
	if goruntime.GOOS != "windows" {
		runtime.BrowserOpenURL(a.ctx, fileURL(filepath.Dir(st.Ready)))
		return errors.New("automatic install is for Windows; the installer's folder has been opened")
	}
	cmd := exec.Command(st.Ready, "/S")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the installer: %w", err)
	}
	a.log.Info("installing update; closing", "version", st.Available)
	go func() {
		time.Sleep(500 * time.Millisecond)
		runtime.Quit(a.ctx)
	}()
	return nil
}

// LoadUpdateFile sends an update package (zip) chosen on this PC to the hub,
// for when the hub PC has no internet: download it anywhere, bring it on a
// USB stick, load it here.
func (a *App) LoadUpdateFile() (*hub.UpdateStatus, error) {
	if _, err := a.currentUser(protocol.RoleAdmin); err != nil {
		return nil, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose a Zebiventor update file",
		Filters: []runtime.FileFilter{{DisplayName: "Update files (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var st hub.UpdateStatus
	if err := a.term.AdminUpload(a.ctx, "/api/admin/update/upload", f, &st); err != nil {
		return nil, err
	}
	a.log.Info("update file loaded into hub", "path", path)
	a.term.Nudge()
	return &st, nil
}
