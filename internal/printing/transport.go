package printing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Transport kinds.
const (
	TransportNone    = "none"
	TransportSystem  = "system"  // installed printer (Windows spooler RAW / CUPS raw)
	TransportNetwork = "network" // Ethernet/Wi-Fi printer, raw TCP (usually port 9100)
	TransportDevice  = "device"  // direct device path, e.g. /dev/usb/lp0 or a file
)

// Target identifies where to send print jobs.
type Target struct {
	Kind    string `json:"kind"`
	Address string `json:"address"` // printer name, host[:port], or device path
}

// ErrNoPrinter means no printer has been configured.
var ErrNoPrinter = errors.New("no receipt printer is set up on this till (Settings → Printer)")

// Send delivers raw ESC/POS bytes to the target.
func Send(ctx context.Context, t Target, data []byte) error {
	switch t.Kind {
	case "", TransportNone:
		return ErrNoPrinter
	case TransportNetwork:
		return sendNetwork(ctx, t.Address, data)
	case TransportDevice:
		return sendDevice(t.Address, data)
	case TransportSystem:
		if strings.TrimSpace(t.Address) == "" {
			return ErrNoPrinter
		}
		return sendSystem(ctx, t.Address, data)
	}
	return fmt.Errorf("unknown printer connection %q", t.Kind)
}

func sendNetwork(ctx context.Context, addr string, data []byte) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ErrNoPrinter
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "9100")
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("printer at %s is not reachable: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	_, err = conn.Write(data)
	return err
}

func sendDevice(path string, data []byte) error {
	if path == "" {
		return ErrNoPrinter
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
