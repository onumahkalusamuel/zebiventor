// Package update describes Zebiventor update packages and checks them.
//
// A release is three files published side by side (the "feed"):
//
//	manifest.json      what the release is (version, platform, file, sha256)
//	manifest.json.sig  ed25519 signature of manifest.json, base64
//	<file>             the installer
//
// The same three files zipped together make an update package that can be
// loaded into the hub by hand. The hub keeps the latest package and tills
// download it from the hub over the LAN, so only the hub needs internet.
package update

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// PublicKey verifies releases (base64 ed25519 public key, set at build time
// with -ldflags "-X .../internal/update.PublicKey=..."). When empty (a
// developer build), unsigned packages are accepted and marked unsigned.
var PublicKey = ""

// DefaultFeed is where the hub looks for new releases unless Settings say
// otherwise (set at build time; empty means "load update files by hand").
var DefaultFeed = ""

const (
	ManifestName  = "manifest.json"
	SignatureName = "manifest.json.sig"
)

// Manifest describes one release for one platform.
type Manifest struct {
	Version    string `json:"version"`
	Platform   string `json:"platform"` // GOOS/GOARCH, e.g. windows/amd64
	File       string `json:"file"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Notes      string `json:"notes,omitempty"`
	ReleasedAt int64  `json:"released_at,omitempty"`
}

// Validate checks the manifest is well formed.
func (m *Manifest) Validate() error {
	switch {
	case m.Version == "":
		return errors.New("update has no version")
	case m.Platform == "" || !strings.Contains(m.Platform, "/"):
		return errors.New("update has no platform")
	case m.File == "" || m.File != path.Base(m.File) || strings.ContainsAny(m.File, `\/:`):
		return errors.New("update has a bad file name")
	case len(m.SHA256) != 64:
		return errors.New("update has no checksum")
	case m.Size <= 0:
		return errors.New("update has no size")
	}
	return nil
}

// ParseManifest decodes and validates a manifest, verifying its signature
// against PublicKey. signed reports whether a signature was checked.
func ParseManifest(raw, sig []byte) (m Manifest, signed bool, err error) {
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, false, fmt.Errorf("update manifest is not valid: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, false, err
	}
	if PublicKey == "" {
		return m, false, nil
	}
	if err := Verify(PublicKey, raw, sig); err != nil {
		return m, false, err
	}
	return m, true, nil
}

// Verify checks sig (base64) is pub's signature of msg.
func Verify(pub string, msg, sig []byte) error {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pub))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("the update key built into this app is not valid")
	}
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(key), msg, s) {
		return errors.New("this update is not signed by Zebiventor; it was not installed")
	}
	return nil
}

// Sign returns the base64 signature of msg.
func Sign(priv ed25519.PrivateKey, msg []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg)))
}

// CheckFile verifies a downloaded installer against the manifest.
func (m *Manifest) CheckFile(r io.Reader) error {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return err
	}
	if n != m.Size {
		return fmt.Errorf("update file is %d bytes, expected %d", n, m.Size)
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(m.SHA256) {
		return errors.New("update file is damaged (checksum does not match)")
	}
	return nil
}

// Package is an unpacked update package (zip).
type Package struct {
	Manifest Manifest
	Raw, Sig []byte
	Signed   bool
	File     *zip.File
}

// OpenPackage reads an update package from a zip in memory or on disk.
func OpenPackage(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, errors.New("this is not a Zebiventor update file")
	}
	p := &Package{}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Base(f.Name)] = f
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, nil
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 1<<20))
	}
	if p.Raw, err = read(ManifestName); err != nil || p.Raw == nil {
		return nil, errors.New("this is not a Zebiventor update file (no manifest)")
	}
	if p.Sig, err = read(SignatureName); err != nil {
		return nil, err
	}
	if p.Manifest, p.Signed, err = ParseManifest(p.Raw, p.Sig); err != nil {
		return nil, err
	}
	if p.File = files[p.Manifest.File]; p.File == nil {
		return nil, fmt.Errorf("update file is missing %s", p.Manifest.File)
	}
	rc, err := p.File.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if err := p.Manifest.CheckFile(rc); err != nil {
		return nil, err
	}
	return p, nil
}

// BuildPackage zips a manifest, signature and installer into an update package.
func BuildPackage(w io.Writer, raw, sig []byte, fileName string, file io.Reader) error {
	zw := zip.NewWriter(w)
	for _, f := range []struct {
		name string
		r    io.Reader
	}{{ManifestName, bytes.NewReader(raw)}, {SignatureName, bytes.NewReader(sig)}, {fileName, file}} {
		if f.name == SignatureName && len(sig) == 0 {
			continue
		}
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Store})
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, f.r); err != nil {
			return err
		}
	}
	return zw.Close()
}

// Compare compares dotted versions ("0.4.0", "v0.4.1-dev"): -1, 0 or 1.
// A pre-release suffix sorts before the release it precedes.
func Compare(a, b string) int {
	na, pa := split(a)
	nb, pb := split(b)
	for i := 0; i < max(len(na), len(nb)); i++ {
		var x, y int
		if i < len(na) {
			x = na[i]
		}
		if i < len(nb) {
			y = nb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	case pa < pb:
		return -1
	}
	return 1
}

func split(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, pre, _ := strings.Cut(v, "-")
	var nums []int
	for p := range strings.SplitSeq(v, ".") {
		n, _ := strconv.Atoi(p)
		nums = append(nums, n)
	}
	return nums, pre
}
