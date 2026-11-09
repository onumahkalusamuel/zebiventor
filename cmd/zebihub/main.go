// Command zebihub runs the Zebiventor store hub without a GUI, for a
// dedicated back-office PC or a till that should keep the hub running in the
// background. It can install itself as a Windows service / systemd unit /
// launchd daemon so the hub starts with the machine.
//
//	zebihub init --store "My Shop" --admin-name "Ada" --admin-user admin
//	zebihub run                       # foreground
//	zebihub pair-code                 # print a code to pair a new till
//	zebihub service install && zebihub service start
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"github.com/onumahkalusamuel/zebiventor/internal/appdata"
	"github.com/onumahkalusamuel/zebiventor/internal/discovery"
	"github.com/onumahkalusamuel/zebiventor/internal/hub"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"golang.org/x/term"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Zebiventor Hub %s

Usage:
  zebihub run        [--data DIR] [--port 7420]   run the hub in the foreground
  zebihub init       --store NAME --admin-name NAME --admin-user USER [--business TYPE] [--demo]
  zebihub pair-code  [--data DIR] [--name "Till 2"]  issue a pairing code for a new till
  zebihub backup     [--data DIR]                  write a backup now
  zebihub restore    [--data DIR] FILE.db          replace the store's data with a backup (hub stopped)
  zebihub status     [--data DIR]
  zebihub service    install|uninstall|start|stop|restart|status [--data DIR] [--port 7420]

Data directory: %s (service: %s)
`, appdata.Version, appdata.Dir(), appdata.ServiceDir())
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "run":
		err = cmdRun(args)
	case "init":
		err = cmdInit(args)
	case "pair-code":
		err = cmdPairCode(args)
	case "backup":
		err = cmdBackup(args)
	case "restore":
		err = cmdRestore(args)
	case "status":
		err = cmdStatus(args)
	case "service":
		err = cmdService(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func openHub(dir string) (*hub.Hub, error) {
	log, _ := appdata.NewLogger(dir, "zebihub.log", os.Stderr)
	slog.SetDefault(log)
	return hub.Open(context.Background(), hub.Config{DataDir: dir, Version: appdata.Version, Logger: log})
}

// ---------- run ----------

type program struct {
	dir    string
	port   int
	cancel context.CancelFunc
	done   chan error
}

func (p *program) Start(service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan error, 1)
	go func() { p.done <- serve(ctx, p.dir, p.port) }()
	return nil
}

func (p *program) Stop(service.Service) error {
	p.cancel()
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
	}
	return nil
}

func serve(ctx context.Context, dir string, port int) error {
	h, err := openHub(dir)
	if err != nil {
		return err
	}
	defer h.Close()
	if !h.IsSetup(ctx) {
		return fmt.Errorf("hub in %s is not set up; run `zebihub init` first", dir)
	}
	info := h.Info(ctx)
	return h.Serve(ctx, fmt.Sprintf(":%d", port), func(a net.Addr) {
		p := a.(*net.TCPAddr).Port
		if err := discovery.Advertise(ctx, info.HubID, info.StoreName, p); err != nil {
			slog.Warn("LAN discovery unavailable; tills must enter the address manually", "err", err)
		}
		for _, ip := range discovery.LANAddresses() {
			fmt.Printf("Hub for %q reachable at http://%s:%d\n", info.StoreName, ip, p)
		}
	})
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	port := fs.Int("port", hub.DefaultPort, "TCP port")
	fs.Parse(args)

	if !service.Interactive() {
		// Started by the service manager.
		s, err := service.New(&program{dir: *dir, port: *port}, svcConfig(*dir, *port))
		if err != nil {
			return err
		}
		return s.Run()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, *dir, *port)
}

// ---------- init ----------

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	in := hub.SetupInput{}
	fs.StringVar(&in.StoreName, "store", "", "store name")
	fs.StringVar(&in.BusinessType, "business", "general", "business type: general, bookshop, grocery, pharmacy, distributor")
	fs.StringVar(&in.LocationName, "location", "Main Store", "first location/branch name")
	fs.StringVar(&in.Address, "address", "", "store address")
	fs.StringVar(&in.Phone, "phone", "", "store phone")
	fs.StringVar(&in.AdminName, "admin-name", "", "administrator's full name")
	fs.StringVar(&in.AdminUsername, "admin-user", "admin", "administrator username")
	fs.StringVar(&in.AdminPassword, "admin-password", "", "administrator password (prompted if omitted)")
	demo := fs.Bool("demo", false, "add a sample catalogue")
	fs.Parse(args)

	if in.AdminPassword == "" {
		pw, err := promptPassword("Admin password: ")
		if err != nil {
			return err
		}
		in.AdminPassword = pw
	}
	h, err := openHub(*dir)
	if err != nil {
		return err
	}
	defer h.Close()
	ctx := context.Background()
	if err := h.Setup(ctx, in); err != nil {
		return err
	}
	if *demo {
		if err := h.SeedDemo(ctx); err != nil {
			return err
		}
	}
	fmt.Printf("Hub for %q created in %s\nNext: `zebihub run` (or `zebihub service install`), then `zebihub pair-code` for each till.\n", in.StoreName, *dir)
	return nil
}

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		return string(b), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}

// ---------- pair-code / backup / status ----------

func cmdPairCode(args []string) error {
	fs := flag.NewFlagSet("pair-code", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	name := fs.String("name", "", "name for the new till (optional)")
	fs.Parse(args)
	h, err := openHub(*dir)
	if err != nil {
		return err
	}
	defer h.Close()
	code, err := h.CreatePairingCode(context.Background(), "", protocol.PairingCodeRequest{TerminalName: *name})
	if err != nil {
		return err
	}
	fmt.Printf("Pairing code: %s  (valid until %s, single use)\n", code.Code, time.UnixMilli(code.ExpiresAt).Format("15:04"))
	fmt.Printf("Security code: %s  (the till shows the same code when it finds this hub)\n", hub.ShortFingerprint(h.TLSFingerprint()))
	return nil
}

func cmdBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	fs.Parse(args)
	h, err := openHub(*dir)
	if err != nil {
		return err
	}
	defer h.Close()
	path, err := h.Backup(context.Background())
	if path != "" {
		fmt.Println("Backup written to", path)
	}
	return err
}

// cmdRestore replaces the store's data with a backup. The hub must be
// stopped; the current data is kept as a backup first.
func cmdRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	port := fs.Int("port", hub.DefaultPort, "TCP port the hub uses")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: zebihub restore [--data DIR] BACKUP.db")
	}
	file := fs.Arg(0)
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", *port), time.Second); err == nil {
		c.Close()
		return fmt.Errorf("the hub is running; stop it first (zebihub service stop)")
	}
	ctx := context.Background()
	h, err := openHub(*dir)
	if err != nil {
		return err
	}
	cur, _ := h.Setting(ctx, "hub_id")
	info, err := hub.InspectBackup(ctx, file, cur)
	if err != nil {
		h.Close()
		return err
	}
	if info.Problem != "" {
		h.Close()
		return fmt.Errorf("%s", info.Problem)
	}
	fmt.Printf("Backup of %q: %d products, %d sales, last activity %s\n", info.StoreName, info.Products, info.Sales,
		time.UnixMilli(info.TakenAt).Format("2 Jan 2006 15:04"))
	if !info.SameStore {
		fmt.Println("WARNING: this backup is from a different store than the one on this PC.")
	}
	if !*yes {
		fmt.Print("Replace the store's data with this backup? Type yes: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			h.Close()
			return fmt.Errorf("cancelled")
		}
	}
	safety, err := h.SafetyBackup(ctx)
	h.Close()
	if err != nil {
		return fmt.Errorf("could not save the current data first: %w", err)
	}
	fmt.Println("Current data saved to", safety)
	if err := hub.RestoreFile(ctx, *dir, file); err != nil {
		return err
	}
	fmt.Println("Restored. Start the hub again; tills will catch up and re-send recent sales.")
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("data", appdata.Dir(), "data directory")
	fs.Parse(args)
	h, err := openHub(*dir)
	if err != nil {
		return err
	}
	defer h.Close()
	ctx := context.Background()
	info := h.Info(ctx)
	sum, _ := h.Summary(ctx)
	fmt.Printf("Store:     %s\nHub id:    %s\nSet up:    %v\nRevision:  %d\nProducts:  %d\nTills:     %d\nSales:     %d (today %d)\n",
		info.StoreName, info.HubID, info.SetupComplete, info.Rev, sum["products"], sum["terminals"], sum["sales"], sum["sales_today"])
	terms, _ := h.ListTerminals(ctx)
	for _, t := range terms {
		seen := "never"
		if t.LastSeenAt > 0 {
			seen = time.Since(time.UnixMilli(t.LastSeenAt)).Round(time.Second).String() + " ago"
		}
		fmt.Printf("  %s  %-20s active=%v last seen %s (%s)\n", t.ReceiptPrefix, t.Name, t.Active, seen, t.LastIP)
	}
	return nil
}

// ---------- service ----------

func svcConfig(dir string, port int) *service.Config {
	return &service.Config{
		Name:        "ZebiventorHub",
		DisplayName: "Zebiventor Hub",
		Description: "Zebiventor store hub: syncs tills, holds the store's records.",
		Arguments:   []string{"run", "--data", dir, "--port", fmt.Sprint(port)},
		Option:      service.KeyValue{"Restart": "always", "OnFailure": "restart"},
	}
}

func cmdService(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("service needs an action: install, uninstall, start, stop, restart, status")
	}
	action := args[0]
	fs := flag.NewFlagSet("service", flag.ExitOnError)
	dir := fs.String("data", appdata.ServiceDir(), "data directory")
	port := fs.Int("port", hub.DefaultPort, "TCP port")
	fs.Parse(args[1:])
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	s, err := service.New(&program{dir: abs, port: *port}, svcConfig(abs, *port))
	if err != nil {
		return err
	}
	if action == "status" {
		st, err := s.Status()
		if err != nil {
			return err
		}
		fmt.Println(map[service.Status]string{service.StatusRunning: "running", service.StatusStopped: "stopped"}[st])
		return nil
	}
	if err := service.Control(s, action); err != nil {
		return err
	}
	fmt.Printf("service %s: ok (data: %s)\n", action, abs)
	return nil
}
