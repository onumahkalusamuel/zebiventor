package terminal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// HTTPError is a non-2xx response from the hub.
type HTTPError struct {
	Status int
	Msg    string
	Code   string
}

func (e *HTTPError) Error() string { return e.Msg }

// IsOffline reports whether err means the hub could not be reached at all
// (as opposed to the hub answering with an error).
func IsOffline(err error) bool {
	var he *HTTPError
	return err != nil && !errors.As(err, &he)
}

func isTerminalRevoked(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.Status == http.StatusUnauthorized && he.Code == "terminal"
}

type client struct {
	http *http.Client
	tls  *pinner
	// headers adds the till's version to each request; onResponse sees
	// every hub response's headers (set once at start-up).
	headers    func(http.Header)
	onResponse func(http.Header)
}

func newClient() *client {
	pn := &pinner{}
	return &client{tls: pn, http: &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 40 * time.Second, // long-poll waits up to 30s
		TLSHandshakeTimeout:   10 * time.Second,
		// The hub's certificate is self-signed: it is checked against the
		// fingerprint pinned at pairing instead of a certificate authority.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, VerifyConnection: pn.verify},
	}}}
}

var errCertChanged = errors.New("certificate changed")

// pinner checks the hub's certificate against the pinned fingerprint. With
// nothing pinned (pairing), it accepts and remembers what it saw.
type pinner struct {
	mu   sync.Mutex
	pin  string
	seen string
}

func (p *pinner) set(fp string) {
	p.mu.Lock()
	p.pin = fp
	p.mu.Unlock()
}

func (p *pinner) get() (pin, seen string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pin, p.seen
}

func (p *pinner) verify(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return errors.New("hub sent no certificate")
	}
	sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
	fp := hex.EncodeToString(sum[:])
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = fp
	if p.pin != "" && p.pin != fp {
		return errCertChanged
	}
	return nil
}

// NormalizeHubURL turns "192.168.1.5" or "192.168.1.5:7420" into a full URL.
func NormalizeHubURL(s string) string {
	s = strings.TrimSpace(strings.TrimRight(s, "/"))
	if s == "" {
		return s
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	rest := s[strings.Index(s, "://")+3:]
	if !strings.Contains(rest, ":") {
		s += ":7420"
	}
	return s
}

func (c *client) do(ctx context.Context, timeout time.Duration, method, url, token, session string, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if session != "" {
		req.Header.Set("X-Session", session)
	}
	if c.headers != nil {
		c.headers(req.Header)
	}
	resp, err := c.http.Do(req)
	if errors.Is(err, errCertChanged) {
		return &HTTPError{Status: 495, Code: "cert", Msg: "The hub's security certificate has changed, so this till stopped talking to it. " +
			"If the hub was reinstalled or moved to a new PC without restoring a backup, pair this till again (Settings → Connection)."}
	}
	if err != nil {
		return fmt.Errorf("hub unreachable: %w", err)
	}
	defer resp.Body.Close()
	if c.onResponse != nil {
		c.onResponse(resp.Header)
	}
	if resp.StatusCode >= 300 {
		var ae protocol.APIError
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&ae)
		if ae.Error == "" {
			ae.Error = resp.Status
		}
		return &HTTPError{Status: resp.StatusCode, Msg: ae.Error, Code: ae.Code}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
