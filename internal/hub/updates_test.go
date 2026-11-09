package hub

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/onumahkalusamuel/zebiventor/internal/update"
)

func TestUpdateFeed(t *testing.T) {
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(nil)
	update.PublicKey = base64.StdEncoding.EncodeToString(pub)
	defer func() { update.PublicKey = "" }()

	feed := t.TempDir()
	publish := func(version string, installer []byte, signer ed25519.PrivateKey) {
		sum := sha256.Sum256(installer)
		m := update.Manifest{Version: version, Platform: "windows/amd64", File: "setup-" + version + ".exe", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(installer))}
		raw, _ := json.Marshal(m)
		_ = os.WriteFile(filepath.Join(feed, update.ManifestName), raw, 0o644)
		_ = os.WriteFile(filepath.Join(feed, update.SignatureName), update.Sign(signer, raw), 0o644)
		_ = os.WriteFile(filepath.Join(feed, m.File), installer, 0o644)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(feed)))
	defer srv.Close()

	h, err := Open(ctx, Config{DataDir: t.TempDir(), Version: "0.4.0"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { h.Close() }()
	if err := h.Setup(ctx, SetupInput{StoreName: "S", AdminName: "A", AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	if err := h.SaveUpdateFeed(ctx, SystemSession, "ftp://nope"); err == nil {
		t.Fatal("non-http feed accepted")
	}
	if err := h.SaveUpdateFeed(ctx, SystemSession, srv.URL); err != nil {
		t.Fatal(err)
	}

	// Same version as the hub: nothing to fetch.
	publish("0.4.0", []byte("same"), priv)
	if v, err := h.CheckForUpdates(ctx); err != nil || v != "" {
		t.Fatalf("same version: %q %v", v, err)
	}
	// Signed by someone else: refused, error recorded.
	_, stranger, _ := ed25519.GenerateKey(nil)
	publish("0.4.1", []byte("evil"), stranger)
	if _, err := h.CheckForUpdates(ctx); err == nil {
		t.Fatal("release signed by another key was accepted")
	}
	if st := h.UpdateStatus(ctx); st.LastCheckError == "" || len(st.Packages) != 0 {
		t.Fatalf("after bad release: %+v", st)
	}
	// A proper newer release is downloaded and offered to tills.
	publish("0.4.1", []byte("good installer"), priv)
	if v, err := h.CheckForUpdates(ctx); err != nil || v != "0.4.1" {
		t.Fatalf("good release: %q %v", v, err)
	}
	pkg, path := h.AvailableUpdate("windows/amd64")
	if pkg == nil || !pkg.Signed || pkg.Source != "feed" {
		t.Fatalf("stored: %+v", pkg)
	}
	if b, _ := os.ReadFile(path); string(b) != "good installer" {
		t.Fatalf("stored file %q", b)
	}
	// Survives a restart.
	h.Close()
	h, _ = Open(ctx, Config{DataDir: h.cfg.DataDir, Version: "0.4.0"})
	if pkg, _ := h.AvailableUpdate("windows/amd64"); pkg == nil || pkg.Manifest.Version != "0.4.1" {
		t.Fatal("update lost on restart")
	}
	if v, _ := h.Setting(ctx, "available_updates"); v != `{"windows/amd64":"0.4.1"}` {
		t.Fatalf("available_updates %q", v)
	}
}
