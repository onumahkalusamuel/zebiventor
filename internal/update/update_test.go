package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.4.0", "0.4.0", 0}, {"0.4.0", "0.4.1", -1}, {"0.10.0", "0.9.9", 1}, {"v1.0", "1.0.0", 0},
		{"0.4.0-dev", "0.4.0", -1}, {"0.4.0", "0.3.5-dev", 1}, {"0.4.0-beta", "0.4.0-alpha", 1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSignedPackage(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	installer := []byte("pretend installer")
	sum := sha256.Sum256(installer)
	m := Manifest{Version: "0.4.0", Platform: "windows/amd64", File: "zebiventor-setup.exe", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(installer))}
	raw, _ := json.Marshal(m)
	var zip bytes.Buffer
	if err := BuildPackage(&zip, raw, Sign(priv, raw), m.File, bytes.NewReader(installer)); err != nil {
		t.Fatal(err)
	}

	PublicKey = base64.StdEncoding.EncodeToString(pub)
	defer func() { PublicKey = "" }()
	p, err := OpenPackage(bytes.NewReader(zip.Bytes()), int64(zip.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Signed || p.Manifest.Version != "0.4.0" {
		t.Fatalf("package: %+v", p.Manifest)
	}

	// Signed by someone else: refused.
	_, other, _ := ed25519.GenerateKey(nil)
	zip.Reset()
	_ = BuildPackage(&zip, raw, Sign(other, raw), m.File, bytes.NewReader(installer))
	if _, err := OpenPackage(bytes.NewReader(zip.Bytes()), int64(zip.Len())); err == nil {
		t.Fatal("package signed with another key was accepted")
	}

	// Tampered installer: refused.
	zip.Reset()
	_ = BuildPackage(&zip, raw, Sign(priv, raw), m.File, bytes.NewReader([]byte("pretend installer!")))
	if _, err := OpenPackage(bytes.NewReader(zip.Bytes()), int64(zip.Len())); err == nil {
		t.Fatal("tampered installer was accepted")
	}

	// Developer build (no key): unsigned accepted, marked unsigned.
	PublicKey = ""
	zip.Reset()
	_ = BuildPackage(&zip, raw, nil, m.File, bytes.NewReader(installer))
	p, err = OpenPackage(bytes.NewReader(zip.Bytes()), int64(zip.Len()))
	if err != nil || p.Signed {
		t.Fatalf("unsigned dev package: %v signed=%v", err, p != nil && p.Signed)
	}
}
