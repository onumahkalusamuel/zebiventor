//go:build !windows

package printing

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// sendSystem submits a raw job through CUPS (macOS/Linux).
func sendSystem(ctx context.Context, name string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lp", "-d", name, "-o", "raw", "-s")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("printing to %q failed: %v %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListPrinters returns installed printer names.
func ListPrinters() ([]string, error) {
	out, err := exec.Command("lpstat", "-e").Output()
	if err != nil {
		return []string{}, nil // CUPS absent or no printers
	}
	names := []string{}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}
