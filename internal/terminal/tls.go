package terminal

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

// Encryption: the hub serves TLS with a self-signed certificate. A till
// pins its fingerprint when pairing (trust on first use) and refuses any
// other certificate afterwards. Tills paired before encryption existed
// upgrade themselves the first time the hub says it offers TLS.

func httpsURL(u string) string {
	if rest, ok := strings.CutPrefix(u, "http://"); ok {
		return "https://" + rest
	}
	return u
}

// SecurityCode is the short form of a fingerprint shown on both screens
// when pairing ("4F2A-91C3"), so people can check they match.
func SecurityCode(fp string) string {
	if len(fp) < 8 {
		return ""
	}
	return strings.ToUpper(fp[:4] + "-" + fp[4:8])
}

func (t *Terminal) refreshTLSStatus(ctx context.Context) {
	pin := t.kv(ctx, "hub_cert")
	url := t.kv(ctx, "hub_url")
	t.updateStatus(func(s *Status) {
		s.Encrypted = pin != "" && strings.HasPrefix(url, "https://")
		s.SecurityCode = SecurityCode(pin)
	})
}

// probe finds how to reach a hub: encrypted first, else plain (an older
// hub). With tofu, an unknown certificate is accepted and returned as fp;
// without it, the pinned one must match.
func (t *Terminal) probe(ctx context.Context, url string, tofu bool) (base, fp string, info protocol.HubInfo, err error) {
	url = NormalizeHubURL(url)
	if url == "" {
		return "", "", info, errors.New("enter the hub address")
	}
	pinned, _ := t.client.tls.get()
	if tofu {
		t.client.tls.set("")
		defer t.client.tls.set(pinned)
	}
	secure := httpsURL(url)
	err = t.client.do(ctx, 5*time.Second, http.MethodGet, secure+"/api/hub/info", "", "", nil, &info)
	if err == nil {
		_, seen := t.client.tls.get()
		if info.TLSFingerprint != "" && info.TLSFingerprint != seen {
			return "", "", info, errors.New("the hub's certificate doesn't match what it reports; someone may be intercepting the connection")
		}
		return secure, seen, info, nil
	}
	var he *HTTPError
	if errors.As(err, &he) && he.Code == "cert" {
		return "", "", info, err
	}
	if pinned != "" && !tofu {
		return "", "", info, err // a pinned hub must stay encrypted
	}
	// Not encrypted: an older hub (or a test server).
	plain := strings.Replace(url, "https://", "http://", 1)
	err = t.client.do(ctx, 5*time.Second, http.MethodGet, plain+"/api/hub/info", "", "", nil, &info)
	return plain, "", info, err
}

// maybeUpgradeTLS moves a till paired over plain HTTP to an encrypted
// connection once the hub offers one, pinning its certificate.
func (t *Terminal) maybeUpgradeTLS(ctx context.Context) {
	t.mu.Lock()
	offered := t.hubTLS
	t.mu.Unlock()
	p, ok := t.Pairing(ctx)
	if offered == "" || !ok || !strings.HasPrefix(p.HubURL, "http://") || t.kv(ctx, "hub_cert") != "" {
		return
	}
	t.client.tls.set(offered)
	secure := httpsURL(p.HubURL)
	var info protocol.HubInfo
	err := t.client.do(ctx, 5*time.Second, http.MethodGet, secure+"/api/hub/info", "", "", nil, &info)
	if err != nil || info.HubID != p.HubID {
		t.client.tls.set("")
		t.log.Warn("could not switch to an encrypted connection", "err", err)
		return
	}
	err = t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if err := setKV(ctx, tx, "hub_cert", offered); err != nil {
			return err
		}
		return setKV(ctx, tx, "hub_url", secure)
	})
	if err != nil {
		t.client.tls.set("")
		return
	}
	t.log.Info("connection to the hub is now encrypted", "url", secure, "code", SecurityCode(offered))
	t.updateStatus(func(s *Status) { s.HubURL = secure })
	t.refreshTLSStatus(ctx)
}

// sameScheme keeps a rediscovered address encrypted when the hub is pinned.
func (t *Terminal) sameScheme(ctx context.Context, url string) string {
	if t.kv(ctx, "hub_cert") != "" {
		return httpsURL(url)
	}
	return url
}
