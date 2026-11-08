package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/appdata"
	"github.com/onumahkalusamuel/zebiventor/internal/discovery"
	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/money"
	"github.com/onumahkalusamuel/zebiventor/internal/printing"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Modes a desktop install can run in.
const (
	ModeHost = "host" // this computer runs the hub and is also a till
	ModeTill = "till" // this computer is a till connected to a hub elsewhere
)

// AppConfig is persisted to app.json in the data directory.
type AppConfig struct {
	Mode    string `json:"mode"`
	HubPort int    `json:"hub_port"`
}

// App is bound to the frontend; its exported methods are callable from React.
type App struct {
	ctx     context.Context
	dataDir string
	log     *slog.Logger

	mu     sync.Mutex
	cfg    AppConfig
	term   *terminal.Terminal
	hub    *hub.Hub
	hubCtl context.CancelFunc
	hubErr string
	user   *terminal.User

	approvals map[string]approval // one-time manager approvals
}

type approval struct {
	user    terminal.User
	expires time.Time
}

func NewApp() *App {
	return &App{dataDir: appdata.Dir(), log: slog.Default(), approvals: map[string]approval{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		a.log.Error("cannot create data dir", "err", err)
	}
	a.cfg = a.loadConfig()
	t, err := terminal.Open(ctx, terminal.Config{
		DataDir:  filepath.Join(a.dataDir, "till"),
		Version:  appdata.Version,
		Logger:   a.log,
		OnStatus: func(s terminal.Status) { runtime.EventsEmit(a.ctx, "sync:status", s) },
		Discover: findHub,
	})
	if err != nil {
		a.log.Error("cannot open till database", "err", err)
		return
	}
	a.term = t
	if a.cfg.Mode == ModeHost {
		if err := a.startHub(); err != nil {
			a.hubErr = err.Error()
			a.log.Error("hub failed to start", "err", err)
		}
	}
	if _, ok := t.Pairing(ctx); ok {
		t.StartSync()
	}
}

func (a *App) shutdown(context.Context) {
	if a.term != nil {
		a.term.Close()
	}
	a.stopHub()
}

func findHub(ctx context.Context, hubID string) (string, bool) {
	hubs, err := discovery.Browse(ctx, 3*time.Second)
	if err != nil {
		return "", false
	}
	for _, h := range hubs {
		if h.HubID == hubID {
			return h.URL, true
		}
	}
	return "", false
}

func (a *App) loadConfig() AppConfig {
	c := AppConfig{HubPort: hub.DefaultPort}
	b, err := os.ReadFile(filepath.Join(a.dataDir, "app.json"))
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.HubPort == 0 {
		c.HubPort = hub.DefaultPort
	}
	return c
}

func (a *App) saveConfig() error {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	return os.WriteFile(filepath.Join(a.dataDir, "app.json"), b, 0o644)
}

// ---------- embedded hub ----------

func (a *App) openHub() (*hub.Hub, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.hub != nil {
		return a.hub, nil
	}
	h, err := hub.Open(a.ctx, hub.Config{DataDir: filepath.Join(a.dataDir, "hub"), Version: appdata.Version, Logger: a.log})
	if err != nil {
		return nil, err
	}
	a.hub = h
	return h, nil
}

func (a *App) startHub() error {
	h, err := a.openHub()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan error, 1)
	go func() {
		err := h.Serve(ctx, fmt.Sprintf(":%d", a.cfg.HubPort), func(addr net.Addr) {
			info := h.Info(ctx)
			if err := discovery.Advertise(ctx, info.HubID, info.StoreName, addr.(*net.TCPAddr).Port); err != nil {
				a.log.Warn("mDNS advertise failed", "err", err)
			}
			ready <- nil
		})
		if err != nil {
			select {
			case ready <- err:
			default:
				a.log.Error("hub stopped", "err", err)
			}
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			cancel()
			return fmt.Errorf("could not start the hub on port %d (is another hub running?): %w", a.cfg.HubPort, err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		return errors.New("hub did not start in time")
	}
	a.mu.Lock()
	a.hubCtl = cancel
	a.hubErr = ""
	a.mu.Unlock()
	return nil
}

func (a *App) stopHub() {
	a.mu.Lock()
	cancel, h := a.hubCtl, a.hub
	a.hubCtl, a.hub = nil, nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if h != nil {
		h.Close()
	}
}

// ---------- state ----------

// AppState is everything the UI needs to decide what to show.
type AppState struct {
	Version      string            `json:"version"`
	DataDir      string            `json:"data_dir"`
	Mode         string            `json:"mode"`
	Paired       bool              `json:"paired"`
	Pairing      terminal.Pairing  `json:"pairing"`
	User         *terminal.User    `json:"user"`
	Status       terminal.Status   `json:"status"`
	Settings     map[string]string `json:"settings"`
	Currency     money.Currency    `json:"currency"`
	HubRunning   bool              `json:"hub_running"`
	HubError     string            `json:"hub_error"`
	HubPort      int               `json:"hub_port"`
	HubAddresses []string          `json:"hub_addresses"`
	HasSession   bool              `json:"has_session"`
	Error        string            `json:"error"`
}

// GetState returns the current application state.
func (a *App) GetState() AppState {
	s := AppState{Version: appdata.Version, DataDir: a.dataDir, Mode: a.cfg.Mode, HubPort: a.cfg.HubPort, Settings: map[string]string{}}
	if a.term == nil {
		s.Error = "The local database could not be opened. Check disk space and permissions for " + a.dataDir
		return s
	}
	p, ok := a.term.Pairing(a.ctx)
	s.Paired, s.Pairing = ok, p
	s.Status = a.term.Status()
	s.Settings = a.term.Settings(a.ctx)
	s.Currency = a.term.Currency(a.ctx)
	s.HasSession = a.term.HasHubSession()
	a.mu.Lock()
	s.User = a.user
	s.HubRunning = a.hubCtl != nil
	s.HubError = a.hubErr
	a.mu.Unlock()
	if a.cfg.Mode == ModeHost {
		s.HubAddresses = discovery.LANAddresses()
	}
	return s
}

// ---------- setup ----------

// SetupHostInput configures this computer as hub + till.
type SetupHostInput struct {
	hub.SetupInput
	TerminalName string `json:"terminal_name"`
	Port         int    `json:"port"`
	Demo         bool   `json:"demo"`
}

// SetupHost creates the store hub on this computer and pairs this computer as its first till.
func (a *App) SetupHost(in SetupHostInput) error {
	if a.term == nil {
		return errors.New("local database unavailable")
	}
	if _, ok := a.term.Pairing(a.ctx); ok {
		return errors.New("this computer is already set up")
	}
	if in.Port > 0 {
		a.cfg.HubPort = in.Port
	}
	h, err := a.openHub()
	if err != nil {
		return err
	}
	if !h.IsSetup(a.ctx) {
		if err := h.Setup(a.ctx, in.SetupInput); err != nil {
			return err
		}
		if in.Demo {
			if err := h.SeedDemo(a.ctx); err != nil {
				return err
			}
		}
	}
	if err := a.startHub(); err != nil {
		return err
	}
	code, err := h.CreatePairingCode(a.ctx, "", protocol.PairingCodeRequest{TerminalName: in.TerminalName})
	if err != nil {
		return err
	}
	if _, err := a.term.Pair(a.ctx, fmt.Sprintf("http://127.0.0.1:%d", a.cfg.HubPort), code.Code, in.TerminalName); err != nil {
		return err
	}
	a.cfg.Mode = ModeHost
	if err := a.saveConfig(); err != nil {
		return err
	}
	return a.afterPair()
}

// DiscoverHubs looks for hubs on the local network.
func (a *App) DiscoverHubs() ([]discovery.Hub, error) {
	return discovery.Browse(a.ctx, 3*time.Second)
}

// CheckHub verifies an address points at a hub.
func (a *App) CheckHub(url string) (protocol.HubInfo, error) {
	return a.term.HubInfo(a.ctx, url)
}

// JoinHub pairs this computer as a till of an existing hub.
func (a *App) JoinHub(url, code, terminalName string) error {
	if a.term == nil {
		return errors.New("local database unavailable")
	}
	if _, err := a.term.Pair(a.ctx, url, code, terminalName); err != nil {
		return err
	}
	a.cfg.Mode = ModeTill
	if err := a.saveConfig(); err != nil {
		return err
	}
	return a.afterPair()
}

// afterPair starts syncing and waits briefly for users so sign-in works at once.
func (a *App) afterPair() error {
	a.term.StartSync()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if a.term.HasUsers(a.ctx) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("paired, but the first sync is taking a while; sign-in will work once it completes")
}

// SetHubAddress updates where this till finds the hub.
func (a *App) SetHubAddress(url string) error { return a.term.SetHubURL(a.ctx, url) }

// SyncNow triggers an immediate push.
func (a *App) SyncNow() { a.term.Nudge() }

// RetryRejected re-queues ops the hub refused.
func (a *App) RetryRejected() error { return a.term.RetryRejected(a.ctx) }

// ---------- auth ----------

// Login signs a staff member in at this till (works offline).
func (a *App) Login(username, secret string) (terminal.User, error) {
	u, err := a.term.Login(a.ctx, username, secret)
	if err != nil {
		return u, err
	}
	a.mu.Lock()
	a.user = &u
	a.mu.Unlock()
	return u, nil
}

// Logout signs the current user out.
func (a *App) Logout() {
	a.term.Logout(a.ctx)
	a.mu.Lock()
	a.user = nil
	a.mu.Unlock()
}

func (a *App) currentUser(roles ...string) (terminal.User, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.user == nil {
		return terminal.User{}, errors.New("please sign in")
	}
	if len(roles) == 0 {
		return *a.user, nil
	}
	for _, r := range roles {
		if a.user.Role == r {
			return *a.user, nil
		}
	}
	return terminal.User{}, errors.New("you do not have permission to do that")
}

// ---------- selling ----------

// SearchItems searches the local catalogue.
func (a *App) SearchItems(query string) ([]terminal.Item, error) {
	return a.term.SearchItems(a.ctx, query, 60)
}

// LookupCode resolves a scanned barcode or typed SKU.
func (a *App) LookupCode(code string) (*terminal.Item, error) {
	return a.term.LookupCode(a.ctx, code)
}

// PaymentMethods lists tenders.
func (a *App) PaymentMethods() ([]protocol.PaymentMethod, error) {
	return a.term.PaymentMethods(a.ctx)
}

// SaleResult is returned after completing a sale.
type SaleResult struct {
	Sale       *protocol.Sale `json:"sale"`
	Printed    bool           `json:"printed"`
	PrintError string         `json:"print_error"`
}

// CompleteSale records a sale, then prints the receipt if auto-print is on.
// The sale is committed before printing, so a printer fault never loses a sale.
func (a *App) CompleteSale(in terminal.SaleInput) (SaleResult, error) {
	u, err := a.currentUser()
	if err != nil {
		return SaleResult{}, err
	}
	s, err := a.term.RecordSale(a.ctx, u, a.takeApproval(in.ApprovalToken), in)
	if err != nil {
		return SaleResult{}, err
	}
	a.consumeApproval(in.ApprovalToken)
	res := SaleResult{Sale: s}
	pc := a.GetPrinterConfig()
	openDrawer := false
	if pc.OpenDrawer {
		methods, _ := a.term.PaymentMethods(a.ctx)
		for _, p := range s.Payments {
			for _, m := range methods {
				if m.ID == p.MethodID && m.OpensDrawer {
					openDrawer = true
				}
			}
		}
	}
	if pc.AutoPrint && pc.Target.Kind != printing.TransportNone && pc.Target.Kind != "" {
		if err := a.printSale(s, false, openDrawer); err != nil {
			res.PrintError = err.Error()
		} else {
			res.Printed = true
		}
	} else if openDrawer {
		_ = printing.Send(a.ctx, pc.Target, printing.DrawerKick())
	}
	return res, nil
}

// RecentSales lists this till's sales.
func (a *App) RecentSales(query string) ([]terminal.SaleSummary, error) {
	return a.term.RecentSales(a.ctx, query, 200)
}

// GetSale loads one sale.
func (a *App) GetSale(id string) (*protocol.Sale, error) { return a.term.GetSale(a.ctx, id) }

// ---------- printing ----------

// PrinterConfig is this till's receipt printer setup (stored locally).
type PrinterConfig struct {
	Target     printing.Target `json:"target"`
	Paper      string          `json:"paper"` // 80mm | 58mm
	Mode       string          `json:"mode"`  // raster | text
	AutoPrint  bool            `json:"auto_print"`
	OpenDrawer bool            `json:"open_drawer"`
	FeedLines  int             `json:"feed_lines"`
	Cut        bool            `json:"cut"`
}

func defaultPrinterConfig() PrinterConfig {
	return PrinterConfig{Target: printing.Target{Kind: printing.TransportNone}, Paper: "80mm", Mode: "raster",
		AutoPrint: true, OpenDrawer: true, FeedLines: 4, Cut: true}
}

// GetPrinterConfig returns the till's printer settings.
func (a *App) GetPrinterConfig() PrinterConfig {
	c := defaultPrinterConfig()
	if v := a.term.GetKV(a.ctx, "printer"); v != "" {
		_ = json.Unmarshal([]byte(v), &c)
	}
	return c
}

// SavePrinterConfig stores the till's printer settings.
func (a *App) SavePrinterConfig(c PrinterConfig) error {
	if _, err := a.currentUser(protocol.RoleAdmin, protocol.RoleManager); err != nil {
		return err
	}
	if c.FeedLines < 0 || c.FeedLines > 10 {
		c.FeedLines = 4
	}
	b, _ := json.Marshal(c)
	return a.term.SetKV(a.ctx, "printer", string(b))
}

// ListPrinters lists printers installed on this computer.
func (a *App) ListPrinters() ([]string, error) { return printing.ListPrinters() }

func (a *App) receiptFor(s *protocol.Sale, reprint bool) printing.Receipt {
	p, _ := a.term.Pairing(a.ctx)
	return printing.FromSale(s, a.term.Settings(a.ctx), a.term.Currency(a.ctx), p.TerminalName, reprint)
}

func (a *App) render(r printing.Receipt, c PrinterConfig, openDrawer bool) ([]byte, error) {
	paper := printing.PaperByName(c.Paper)
	blocks := printing.Layout(r, paper.Cols)
	opts := printing.JobOptions{Cut: c.Cut, FeedLines: c.FeedLines, OpenDrawer: openDrawer}
	if c.Mode == "text" {
		return printing.EncodeText(blocks, paper.Cols, opts), nil
	}
	img, err := printing.Render(blocks, paper)
	if err != nil {
		return nil, err
	}
	return printing.EncodeRaster(img, opts), nil
}

func (a *App) printSale(s *protocol.Sale, reprint, openDrawer bool) error {
	c := a.GetPrinterConfig()
	job, err := a.render(a.receiptFor(s, reprint), c, openDrawer)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return printing.Send(ctx, c.Target, job)
}

// PrintSale prints (or reprints) a stored sale's receipt.
func (a *App) PrintSale(id string, reprint bool) error {
	s, err := a.term.GetSale(a.ctx, id)
	if err != nil {
		return err
	}
	return a.printSale(s, reprint, false)
}

// ReceiptPreview renders a sale's receipt as a PNG data URL, pixel-identical to the print.
func (a *App) ReceiptPreview(id string, reprint bool) (string, error) {
	s, err := a.term.GetSale(a.ctx, id)
	if err != nil {
		return "", err
	}
	return a.previewReceipt(a.receiptFor(s, reprint))
}

func (a *App) previewReceipt(r printing.Receipt) (string, error) {
	paper := printing.PaperByName(a.GetPrinterConfig().Paper)
	img, err := printing.Render(printing.Layout(r, paper.Cols), paper)
	if err != nil {
		return "", err
	}
	return printing.PNGDataURL(img)
}

func (a *App) testReceipt(c PrinterConfig) printing.Receipt {
	return printing.TestReceipt(a.term.Settings(a.ctx)["store_name"], a.term.Currency(a.ctx), printing.PaperByName(c.Paper))
}

// TestPreview renders the test page with the given (unsaved) settings.
func (a *App) TestPreview(c PrinterConfig) (string, error) {
	paper := printing.PaperByName(c.Paper)
	img, err := printing.Render(printing.Layout(a.testReceipt(c), paper.Cols), paper)
	if err != nil {
		return "", err
	}
	return printing.PNGDataURL(img)
}

// TestPrint prints a test page using the given (possibly unsaved) settings.
func (a *App) TestPrint(c PrinterConfig) error {
	job, err := a.render(a.testReceipt(c), c, false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return printing.Send(ctx, c.Target, job)
}

// OpenDrawer opens the cash drawer without a sale ("no sale"); cashiers need a manager.
func (a *App) OpenDrawer(approvalToken string) error {
	u, err := a.currentUser()
	if err != nil {
		return err
	}
	approver := a.takeApproval(approvalToken)
	if !terminal.IsManager(u.Role) && approver == nil {
		return terminal.ErrApprovalRequired
	}
	a.consumeApproval(approvalToken)
	return printing.Send(a.ctx, a.GetPrinterConfig().Target, printing.DrawerKick())
}

// ---------- catalogue & admin ----------

// ListProducts lists the catalogue with stock for this till's location.
func (a *App) ListProducts(query string) ([]terminal.ProductDetail, error) {
	return a.term.ListProducts(a.ctx, query, 500)
}

// Categories lists categories.
func (a *App) Categories() ([]protocol.Category, error) { return a.term.Categories(a.ctx) }

type idResp struct {
	ID string `json:"id"`
}

// SaveProduct creates/updates a product at the hub.
func (a *App) SaveProduct(in protocol.ProductInput) (string, error) {
	var r idResp
	err := a.term.Admin(a.ctx, http.MethodPost, "/api/admin/products", in, &r)
	return r.ID, err
}

// SaveCategory creates/updates a category at the hub.
func (a *App) SaveCategory(c protocol.Category) (string, error) {
	var r idResp
	err := a.term.Admin(a.ctx, http.MethodPost, "/api/admin/categories", c, &r)
	return r.ID, err
}

// AdjustStock records a stock adjustment or count at the hub.
func (a *App) AdjustStock(req protocol.StockAdjustRequest) error {
	if req.LocationID == "" {
		p, _ := a.term.Pairing(a.ctx)
		req.LocationID = p.LocationID
	}
	return a.term.Admin(a.ctx, http.MethodPost, "/api/admin/stock/adjust", req, nil)
}

// CreatePairingCode issues a code for pairing another till.
func (a *App) CreatePairingCode(terminalName string) (protocol.PairingCode, error) {
	var c protocol.PairingCode
	err := a.term.Admin(a.ctx, http.MethodPost, "/api/admin/pairing-codes", protocol.PairingCodeRequest{TerminalName: terminalName}, &c)
	return c, err
}

// ListTerminals lists the hub's tills.
func (a *App) ListTerminals() ([]protocol.TerminalInfo, error) {
	var out []protocol.TerminalInfo
	err := a.term.Admin(a.ctx, http.MethodGet, "/api/admin/terminals", nil, &out)
	return out, err
}

// RevokeTerminal disconnects a till from the hub.
func (a *App) RevokeTerminal(id string) error {
	return a.term.Admin(a.ctx, http.MethodPost, "/api/admin/terminals/"+id+"/revoke", nil, nil)
}

// ListUsers lists staff accounts.
func (a *App) ListUsers() ([]terminal.StaffUser, error) { return a.term.Users(a.ctx) }

// SaveUser creates/updates a staff account at the hub.
func (a *App) SaveUser(in protocol.UserInput) (string, error) {
	var r idResp
	err := a.term.Admin(a.ctx, http.MethodPost, "/api/admin/users", in, &r)
	return r.ID, err
}

// SaveStoreSettings updates store settings at the hub.
func (a *App) SaveStoreSettings(values map[string]string) error {
	return a.term.Admin(a.ctx, http.MethodPut, "/api/admin/settings", values, nil)
}

// SavePaymentMethod creates/updates a payment method at the hub.
func (a *App) SavePaymentMethod(m protocol.PaymentMethod) (string, error) {
	var r idResp
	err := a.term.Admin(a.ctx, http.MethodPost, "/api/admin/payment-methods", m, &r)
	return r.ID, err
}

// ---------- approvals ----------

// ApprovalResult identifies the manager who approved.
type ApprovalResult struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// Approve verifies a manager's credentials at this till and returns a
// single-use token, valid for two minutes, that authorises one action.
func (a *App) Approve(username, secret string) (ApprovalResult, error) {
	if _, err := a.currentUser(); err != nil {
		return ApprovalResult{}, err
	}
	u, err := a.term.Authenticate(a.ctx, username, secret)
	if err != nil {
		return ApprovalResult{}, err
	}
	if !terminal.IsManager(u.Role) {
		return ApprovalResult{}, errors.New(u.Name + " is not a manager")
	}
	tok := ids.New()
	a.mu.Lock()
	for k, v := range a.approvals {
		if time.Now().After(v.expires) {
			delete(a.approvals, k)
		}
	}
	a.approvals[tok] = approval{user: u, expires: time.Now().Add(2 * time.Minute)}
	a.mu.Unlock()
	return ApprovalResult{Token: tok, Name: u.Name}, nil
}

// takeApproval resolves a token (nil if absent or expired). It is only
// consumed by consumeApproval once the approved action has succeeded, so a
// sale that fails validation doesn't need a second manager sign-off.
func (a *App) takeApproval(tok string) *terminal.User {
	if tok == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ap, ok := a.approvals[tok]
	if !ok || time.Now().After(ap.expires) {
		delete(a.approvals, tok)
		return nil
	}
	return &ap.user
}

func (a *App) consumeApproval(tok string) {
	a.mu.Lock()
	delete(a.approvals, tok)
	a.mu.Unlock()
}

// ---------- returns ----------

// LookupForReturn finds a sale (from any till) to refund against.
func (a *App) LookupForReturn(receiptNo string) (*terminal.ReturnableSale, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.LookupForReturn(a.ctx, receiptNo)
}

// ReturnResult is returned after completing a refund.
type ReturnResult struct {
	Return     *protocol.Return `json:"return"`
	Printed    bool             `json:"printed"`
	PrintError string           `json:"print_error"`
}

// CompleteReturn records a refund and prints the refund slip.
func (a *App) CompleteReturn(in terminal.ReturnInput) (ReturnResult, error) {
	u, err := a.currentUser()
	if err != nil {
		return ReturnResult{}, err
	}
	r, err := a.term.RecordReturn(a.ctx, u, a.takeApproval(in.ApprovalToken), in)
	if err != nil {
		return ReturnResult{}, err
	}
	a.consumeApproval(in.ApprovalToken)
	res := ReturnResult{Return: r}
	pc := a.GetPrinterConfig()
	cashOut := false
	for _, f := range r.Refunds {
		if f.Kind == protocol.PayCash {
			cashOut = true
		}
	}
	if pc.Target.Kind != printing.TransportNone && pc.Target.Kind != "" {
		if err := a.sendReceipt(printing.FromReturn(r, a.term.Settings(a.ctx), a.term.Currency(a.ctx), a.terminalName(), false), pc, cashOut && pc.OpenDrawer); err != nil {
			res.PrintError = err.Error()
		} else {
			res.Printed = true
		}
	}
	return res, nil
}

// ReturnPreview renders a refund slip.
func (a *App) ReturnPreview(id string, reprint bool) (string, error) {
	r, err := a.term.GetReturn(a.ctx, id)
	if err != nil {
		return "", err
	}
	return a.previewReceipt(printing.FromReturn(r, a.term.Settings(a.ctx), a.term.Currency(a.ctx), a.terminalName(), reprint))
}

// PrintReturn prints a refund slip.
func (a *App) PrintReturn(id string, reprint bool) error {
	r, err := a.term.GetReturn(a.ctx, id)
	if err != nil {
		return err
	}
	return a.sendReceipt(printing.FromReturn(r, a.term.Settings(a.ctx), a.term.Currency(a.ctx), a.terminalName(), reprint), a.GetPrinterConfig(), false)
}

// ---------- shifts ----------

// CurrentShift returns the open shift with its live X report, or nil.
func (a *App) CurrentShift() (*protocol.Shift, error) { return a.term.CurrentShift(a.ctx) }

// OpenShift starts a shift with the given cash float.
func (a *App) OpenShift(floatMinor int64) (*protocol.Shift, error) {
	u, err := a.currentUser()
	if err != nil {
		return nil, err
	}
	return a.term.OpenShift(a.ctx, u, floatMinor)
}

// CashMovement records cash paid into or out of the drawer.
func (a *App) CashMovement(kind string, amountMinor int64, reason, approvalToken string) error {
	u, err := a.currentUser()
	if err != nil {
		return err
	}
	if _, err := a.term.AddCashMovement(a.ctx, u, a.takeApproval(approvalToken), kind, amountMinor, reason); err != nil {
		return err
	}
	a.consumeApproval(approvalToken)
	if pc := a.GetPrinterConfig(); pc.OpenDrawer && pc.Target.Kind != printing.TransportNone && pc.Target.Kind != "" {
		_ = printing.Send(a.ctx, pc.Target, printing.DrawerKick())
	}
	return nil
}

// ShiftMovements lists paid-in/out entries of a shift.
func (a *App) ShiftMovements(shiftID string) ([]protocol.CashMovement, error) {
	return a.term.CashMovements(a.ctx, shiftID)
}

// CloseShiftResult is the outcome of closing a shift.
type CloseShiftResult struct {
	Shift      *protocol.Shift `json:"shift"`
	PrintError string          `json:"print_error"`
}

// CloseShift counts the drawer, ends the shift and prints the Z report.
func (a *App) CloseShift(countedMinor int64, note string) (CloseShiftResult, error) {
	u, err := a.currentUser()
	if err != nil {
		return CloseShiftResult{}, err
	}
	s, err := a.term.CloseShift(a.ctx, u, countedMinor, note)
	if err != nil {
		return CloseShiftResult{}, err
	}
	res := CloseShiftResult{Shift: s}
	if pc := a.GetPrinterConfig(); pc.Target.Kind != printing.TransportNone && pc.Target.Kind != "" {
		if err := a.PrintShift(s.ID); err != nil {
			res.PrintError = err.Error()
		}
	}
	return res, nil
}

// ListShifts lists recent shifts on this till.
func (a *App) ListShifts() ([]protocol.Shift, error) { return a.term.ListShifts(a.ctx, 60) }

// ShiftPreview renders a shift's X/Z report.
func (a *App) ShiftPreview(id string) (string, error) {
	s, err := a.term.GetShift(a.ctx, id)
	if err != nil {
		return "", err
	}
	return a.previewReceipt(printing.ShiftReport(s, a.term.Settings(a.ctx), a.term.Currency(a.ctx), a.terminalName()))
}

// PrintShift prints a shift's X/Z report.
func (a *App) PrintShift(id string) error {
	s, err := a.term.GetShift(a.ctx, id)
	if err != nil {
		return err
	}
	return a.sendReceipt(printing.ShiftReport(s, a.term.Settings(a.ctx), a.term.Currency(a.ctx), a.terminalName()), a.GetPrinterConfig(), false)
}

// ---------- held sales ----------

// HoldSale parks the current cart.
func (a *App) HoldSale(h terminal.HeldSale) (string, error) {
	u, err := a.currentUser()
	if err != nil {
		return "", err
	}
	return a.term.HoldSale(a.ctx, u, h)
}

// HeldSales lists parked carts.
func (a *App) HeldSales() ([]terminal.HeldSale, error) { return a.term.HeldSales(a.ctx) }

// RecallHeldSale removes a parked cart and returns it.
func (a *App) RecallHeldSale(id string) (*terminal.HeldSale, error) {
	return a.term.TakeHeldSale(a.ctx, id)
}

// ---------- customers ----------

// SearchCustomers finds customers.
func (a *App) SearchCustomers(query string) ([]protocol.Customer, error) {
	return a.term.SearchCustomers(a.ctx, query, 100)
}

// SaveCustomer creates or updates a customer (works offline).
func (a *App) SaveCustomer(c protocol.Customer) (*protocol.Customer, error) {
	if _, err := a.currentUser(); err != nil {
		return nil, err
	}
	return a.term.SaveCustomer(a.ctx, c)
}

func (a *App) terminalName() string {
	p, _ := a.term.Pairing(a.ctx)
	return p.TerminalName
}

func (a *App) sendReceipt(r printing.Receipt, c PrinterConfig, openDrawer bool) error {
	job, err := a.render(r, c, openDrawer)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return printing.Send(ctx, c.Target, job)
}
