// Command zebirelease signs a Windows installer into a Zebiventor release.
//
//	zebirelease keygen -out release.key        # once; keep the key safe, offline
//	zebirelease sign -key release.key -version 0.4.0 -notes "…" -out dist build/bin/zebiventor-amd64-installer.exe
//
// sign writes, into -out:
//
//	manifest.json, manifest.json.sig, the installer   publish these three side by side (the update feed)
//	zebiventor-<version>-update.zip                    the same, for loading into a hub by hand
//
// Build the app with the public key printed by keygen (ZEBI_UPDATE_KEY, see
// scripts/build-windows.sh) so tills and hubs only accept releases you signed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: zebirelease keygen|sign ...")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "release.key", "private key file to write")
	fs.Parse(args)
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists; not overwriting a signing key", *out)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(base64.StdEncoding.EncodeToString(priv)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Println("Private key written to", *out, "(keep it secret and backed up; without it you cannot ship updates)")
	fmt.Println("Public key (build with ZEBI_UPDATE_KEY set to this):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	return nil
}

func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyFile := fs.String("key", "release.key", "private key file")
	version := fs.String("version", "", "release version, e.g. 0.4.0")
	platform := fs.String("platform", "windows/amd64", "GOOS/GOARCH the installer is for")
	notes := fs.String("notes", "", "what changed (shown to the shop)")
	outDir := fs.String("out", "dist", "output folder")
	fs.Parse(args)
	if fs.NArg() != 1 || *version == "" {
		return errors.New("usage: zebirelease sign -key K -version V [-notes N] [-out DIR] INSTALLER")
	}
	kb, err := os.ReadFile(*keyFile)
	if err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(kb)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("not a zebirelease private key")
	}
	priv := ed25519.PrivateKey(key)

	src := fs.Arg(0)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	h := sha256.New()
	size, err := io.Copy(h, in)
	if err != nil {
		return err
	}
	file := fmt.Sprintf("zebiventor-%s-%s-setup.exe", *version, strings.ReplaceAll(*platform, "/", "-"))
	m := update.Manifest{Version: *version, Platform: *platform, File: file, SHA256: hex.EncodeToString(h.Sum(nil)),
		Size: size, Notes: *notes, ReleasedAt: time.Now().UnixMilli()}
	if err := m.Validate(); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	sig := update.Sign(priv, raw)
	if err := update.Verify(base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)), raw, sig); err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	write := func(name string, b []byte) error { return os.WriteFile(filepath.Join(*outDir, name), b, 0o644) }
	if err := write(update.ManifestName, raw); err != nil {
		return err
	}
	if err := write(update.SignatureName, sig); err != nil {
		return err
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dst, err := os.Create(filepath.Join(*outDir, file))
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, in); err != nil {
		dst.Close()
		return err
	}
	dst.Close()

	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return err
	}
	zipName := fmt.Sprintf("zebiventor-%s-update.zip", *version)
	zf, err := os.Create(filepath.Join(*outDir, zipName))
	if err != nil {
		return err
	}
	if err := update.BuildPackage(zf, raw, sig, file, in); err != nil {
		zf.Close()
		return err
	}
	if err := zf.Close(); err != nil {
		return err
	}
	fmt.Printf("Release %s (%s) written to %s:\n  feed: %s, %s, %s\n  update file: %s\n", *version, *platform, *outDir,
		update.ManifestName, update.SignatureName, file, zipName)
	return nil
}
