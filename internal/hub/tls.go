package hub

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
)

// The hub encrypts sync with TLS using its own self-signed certificate.
// There is no certificate authority on a shop's LAN, so tills pin the
// certificate's fingerprint when they pair (trust on first use) and refuse
// any other afterwards. The certificate lives in the hub database (private
// settings), so it moves with backups and a restored hub keeps its identity.

// Fingerprint formats a certificate's SHA-256 as lowercase hex.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// ShortFingerprint is the first 8 hex digits in two groups ("4F2A-91C3"),
// shown on both screens when pairing so people can compare them.
func ShortFingerprint(fp string) string {
	if len(fp) < 8 {
		return fp
	}
	return fmt.Sprintf("%X-%X", fp[:4], fp[4:8])
}

// ensureCert loads the hub's certificate, creating one on first run.
func (h *Hub) ensureCert(ctx context.Context) error {
	certPEM, _ := h.Setting(ctx, "tls_cert")
	keyPEM, _ := h.Setting(ctx, "tls_key")
	if certPEM == "" || keyPEM == "" {
		var err error
		if certPEM, keyPEM, err = newCert(); err != nil {
			return err
		}
		if err := h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			now := ids.NowMs()
			if err := putSetting(ctx, tx, "tls_cert", certPEM, now); err != nil {
				return err
			}
			return putSetting(ctx, tx, "tls_key", keyPEM, now)
		}); err != nil {
			return err
		}
	}
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return fmt.Errorf("hub certificate: %w", err)
	}
	h.cert = cert
	h.fingerprint = Fingerprint(cert.Certificate[0])
	return nil
}

func newCert() (certPEM, keyPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Zebiventor hub", Organization: []string{"Zebiventor"}},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().AddDate(30, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})), nil
}

// TLSFingerprint is the hub certificate's fingerprint.
func (h *Hub) TLSFingerprint() string { return h.fingerprint }

func (h *Hub) tlsConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{h.cert}, MinVersion: tls.VersionTLS12}
}

// ---------- one port, encrypted or not ----------

// sniffListener serves TLS and plain HTTP on the same port: a TLS
// connection starts with a handshake record (0x16). Plain HTTP stays
// available so tills paired before encryption keep working until they
// upgrade themselves (or an admin requires encryption).
type sniffListener struct {
	net.Listener
	cfg *tls.Config
}

func (l *sniffListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &sniffConn{Conn: c, cfg: l.cfg}, nil
}

type sniffConn struct {
	net.Conn // the raw connection (deadlines, addresses, Close)
	cfg      *tls.Config
	once     sync.Once
	inner    net.Conn
	tls      bool
}

func (c *sniffConn) init() {
	c.once.Do(func() {
		br := bufio.NewReader(c.Conn)
		b, err := br.Peek(1)
		pc := &peekedConn{Conn: c.Conn, r: br}
		if err == nil && b[0] == 0x16 {
			c.tls, c.inner = true, tls.Server(pc, c.cfg)
			return
		}
		c.inner = pc
	})
}

func (c *sniffConn) Read(p []byte) (int, error)  { c.init(); return c.inner.Read(p) }
func (c *sniffConn) Write(p []byte) (int, error) { c.init(); return c.inner.Write(p) }

// Encrypted reports whether the connection uses TLS (valid once read from).
func (c *sniffConn) Encrypted() bool { c.init(); return c.tls }

type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

type connKey struct{}

// encryptedRequest reports whether r came over TLS. Requests not served by
// Serve (tests using Handler directly) count as encrypted.
func encryptedRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	c, ok := r.Context().Value(connKey{}).(*sniffConn)
	return !ok || c.Encrypted()
}

// servedPlain reports whether r arrived unencrypted through Serve.
func servedPlain(r *http.Request) bool {
	c, ok := r.Context().Value(connKey{}).(*sniffConn)
	return ok && !c.Encrypted()
}
