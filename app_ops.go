package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
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
