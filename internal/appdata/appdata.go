// Package appdata resolves where Zebiventor keeps its files.
package appdata

import (
	"os"
	"path/filepath"
	"runtime"
)

// Version is the application version (overridden at build time with -ldflags).
var Version = "0.1.0-dev"

// Dir returns the data directory. ZEBIVENTOR_DATA overrides it, which also
// makes it easy to run several tills on one machine for testing.
func Dir() string {
	if d := os.Getenv("ZEBIVENTOR_DATA"); d != "" {
		return d
	}
	base, err := os.UserConfigDir() // %AppData% on Windows, ~/Library/Application Support on macOS
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "Zebiventor")
}

// ServiceDir is the data directory for the hub when run as a system service
// (not tied to any user profile).
func ServiceDir() string {
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "Zebiventor")
		}
	}
	return Dir()
}
