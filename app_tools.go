package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/appdata"
	"github.com/onumahkalusamuel/zebiventor/internal/documents"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- A4 documents ----------

// Document kinds PrintDocument understands.
const (
	DocPurchaseOrder     = "purchase_order"
	DocTransfer          = "transfer"
	DocSupplierStatement = "supplier_statement"
	DocPaymentAdvice     = "payment_advice"
)

func (a *App) hubGet(path string, out any) error {
	return a.term.Admin(a.ctx, http.MethodGet, path, nil, out)
}

func (a *App) supplier(id string) *protocol.Supplier {
	var list []protocol.Supplier
	if err := a.hubGet("/api/admin/suppliers", &list); err != nil {
		return nil
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// PrintDocument opens an A4 document in the system browser, where it can be
// printed or saved as PDF. It returns the file's path.
func (a *App) PrintDocument(kind, id string) (string, error) {
	if _, err := a.currentUser(); err != nil {
		return "", err
	}
	store := documents.StoreFromSettings(a.term.Settings(a.ctx))
	cur := a.term.Currency(a.ctx)
	var doc documents.Doc
	esc := url.PathEscape(id)
	switch kind {
	case DocPurchaseOrder:
		var o protocol.PurchaseOrder
		if err := a.hubGet("/api/admin/purchase-orders/"+esc, &o); err != nil {
			return "", err
		}
		doc = documents.PurchaseOrder(&o, a.supplier(o.SupplierID), store, cur)
	case DocTransfer:
		var t protocol.StockTransfer
		if err := a.hubGet("/api/admin/transfers/"+esc, &t); err != nil {
			return "", err
		}
		doc = documents.TransferNote(&t, store)
	case DocSupplierStatement:
		var st protocol.SupplierStatement
		if err := a.hubGet("/api/admin/suppliers/"+esc+"/statement", &st); err != nil {
			return "", err
		}
		doc = documents.SupplierStatement(&st, store, cur)
	case DocPaymentAdvice:
		var p protocol.SupplierPayment
		if err := a.hubGet("/api/admin/supplier-payments/"+esc, &p); err != nil {
			return "", err
		}
		doc = documents.PaymentAdvice(&p, a.supplier(p.SupplierID), store, cur)
	default:
		return "", fmt.Errorf("unknown document %q", kind)
	}
	html, err := doc.HTML()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(a.dataDir, "documents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := doc.Title
	if doc.Number != "" {
		name += " " + doc.Number
	} else if len(doc.Party) > 0 {
		name += " " + doc.Party[0]
	}
	path := filepath.Join(dir, safeFileName(name)+".html")
	if err := os.WriteFile(path, html, 0o644); err != nil {
		return "", err
	}
	runtime.BrowserOpenURL(a.ctx, fileURL(path))
	a.log.Info("document opened", "kind", kind, "path", path)
	return path, nil
}

func safeFileName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
}

// fileURL turns a local path into a file:// URL (file:///C:/... on Windows).
func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// ---------- diagnostics ----------

// Diagnostics is what support needs to know about this computer.
type Diagnostics struct {
	Version     string           `json:"version"`
	OS          string           `json:"os"`
	Mode        string           `json:"mode"`
	DataDir     string           `json:"data_dir"`
	LogFile     string           `json:"log_file"`
	Terminal    string           `json:"terminal"`
	HubURL      string           `json:"hub_url"`
	HubRunning  bool             `json:"hub_running"`
	HubError    string           `json:"hub_error"`
	Sync        terminal.Status  `json:"sync"`
	Printer     PrinterConfig    `json:"printer"`
	Printers    []string         `json:"printers"`
	PrintersErr string           `json:"printers_error"`
	Databases   map[string]int64 `json:"database_bytes"`
	Time        string           `json:"time"`
}

// GetDiagnostics describes this install for troubleshooting.
func (a *App) GetDiagnostics() Diagnostics {
	d := Diagnostics{Version: appdata.Version, OS: goruntime.GOOS + "/" + goruntime.GOARCH, Mode: a.cfg.Mode, DataDir: a.dataDir,
		Databases: map[string]int64{}, Time: time.Now().Format(time.RFC3339)}
	if a.logFile != nil {
		d.LogFile = a.logFile.Path()
	}
	if a.term != nil {
		p, _ := a.term.Pairing(a.ctx)
		d.Terminal, d.HubURL = p.TerminalName, p.HubURL
		d.Sync = a.term.Status()
		d.Printer = a.GetPrinterConfig()
	}
	a.mu.Lock()
	d.HubRunning, d.HubError = a.hubCtl != nil, a.hubErr
	a.mu.Unlock()
	if list, err := a.ListPrinters(); err != nil {
		d.PrintersErr = err.Error()
	} else {
		d.Printers = list
	}
	_ = filepath.WalkDir(a.dataDir, func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() && (strings.HasSuffix(p, ".db") || strings.HasSuffix(p, ".db-wal")) {
			if info, err := e.Info(); err == nil {
				rel, _ := filepath.Rel(a.dataDir, p)
				d.Databases[filepath.ToSlash(rel)] = info.Size()
			}
		}
		return nil
	})
	return d
}

// SaveDiagnostics writes a zip of the logs and settings (no sales or
// customer data) for sending to support. It returns the path, or "" if the
// user cancelled.
func (a *App) SaveDiagnostics() (string, error) {
	name := "zebiventor-diagnostics-" + time.Now().Format("20060102-1504") + ".zip"
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: name,
		Filters:         []runtime.FileFilter{{DisplayName: "Zip files (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	add := func(name string, r io.Reader) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		return err
	}
	diag, _ := json.MarshalIndent(a.GetDiagnostics(), "", "  ")
	errs := []error{add("diagnostics.json", strings.NewReader(string(diag)))}
	logs, _ := filepath.Glob(filepath.Join(a.dataDir, "logs", "*"))
	for _, p := range append(logs, filepath.Join(a.dataDir, "app.json")) {
		src, err := os.Open(p)
		if err != nil {
			continue
		}
		errs = append(errs, add(filepath.Base(p), src))
		src.Close()
	}
	errs = append(errs, zw.Close(), f.Close())
	if err := errors.Join(errs...); err != nil {
		return "", err
	}
	a.log.Info("diagnostics saved", "path", path)
	return path, nil
}

// OpenDataFolder shows the data folder (databases, logs, backups) in the
// file manager.
func (a *App) OpenDataFolder() {
	runtime.BrowserOpenURL(a.ctx, fileURL(a.dataDir))
}
