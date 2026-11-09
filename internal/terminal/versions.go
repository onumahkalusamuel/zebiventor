package terminal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"github.com/onumahkalusamuel/zebiventor/internal/update"
)

// Header names shared with the hub (see hub/http.go).
const (
	hdrVersion    = "X-Zebi-Version"
	hdrProtocol   = "X-Zebi-Protocol"
	hdrPlatform   = "X-Zebi-Platform"
	hdrHubVersion = "X-Zebi-Hub-Version"
	hdrEpoch      = "X-Zebi-Epoch"
	hdrUpdate     = "X-Zebi-Update"
	hdrRev        = "X-Zebi-Rev"
)

// Platform is this build's GOOS/GOARCH, as update packages name it.
func Platform() string { return goruntime.GOOS + "/" + goruntime.GOARCH }

func isVersionMismatch(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.Code == "version"
}

// requestHeaders are sent with every call to the hub.
func (t *Terminal) requestHeaders(h http.Header) {
	h.Set(hdrVersion, t.cfg.Version)
	h.Set(hdrProtocol, strconv.Itoa(protocol.Version))
	h.Set(hdrPlatform, Platform())
}

// onHubHeaders reads the hub's version, the update it holds and its data
// epoch from any response.
func (t *Terminal) onHubHeaders(h http.Header) {
	hv := h.Get(hdrHubVersion)
	if hv == "" {
		return // not a hub (or a very old one)
	}
	upd := h.Get(hdrUpdate)
	if upd != "" && update.Compare(upd, t.cfg.Version) <= 0 {
		upd = ""
	}
	t.updateStatus(func(s *Status) { s.HubVersion, s.UpdateVersion = hv, upd })
	if e := h.Get(hdrEpoch); e != "" {
		t.mu.Lock()
		t.hubEpoch = e
		t.mu.Unlock()
	}
	if r := h.Get(hdrRev); r != "" {
		if n, err := strconv.ParseInt(r, 10, 64); err == nil {
			t.mu.Lock()
			t.hubRev = n
			t.mu.Unlock()
		}
	}
}

// hubRestored notices that the hub's data was restored from a backup (its
// epoch changed). The till then downloads everything again and re-sends
// whatever reached the hub after the backup was taken: the hub ignores
// anything it already has. It reports whether a reset happened.
func (t *Terminal) hubRestored(ctx context.Context) bool {
	t.mu.Lock()
	seen, hubRev := t.hubEpoch, t.hubRev
	t.hubRev = -1
	t.mu.Unlock()
	known := t.kv(ctx, "hub_epoch")
	var takenAt int64
	switch {
	case seen != "" && known == "": // first contact since this feature: just remember it
		_ = t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error { return setKV(ctx, tx, "hub_epoch", seen) })
		return false
	case seen != "" && known != seen: // restored from a backup
		if _, ms, ok := strings.Cut(seen, ":"); ok {
			takenAt, _ = strconv.ParseInt(ms, 10, 64)
		}
	default:
		// Same epoch, but the hub is behind this till: its data was put
		// back by hand (an older copy). Catch up the same way, re-sending
		// the last two weeks.
		since, _ := strconv.ParseInt(t.kv(ctx, "last_rev"), 10, 64)
		if hubRev < 0 || hubRev >= since {
			return false
		}
		takenAt = time.Now().Add(-14 * 24 * time.Hour).UnixMilli()
		if seen == "" {
			seen = known
		}
	}
	var resent int64
	err := t.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "UPDATE outbox SET status='pending' WHERE status='sent' AND sent_at >= ?", takenAt-10*60*1000)
		if err != nil {
			return err
		}
		resent, _ = res.RowsAffected()
		if err := setKV(ctx, tx, "last_rev", "0"); err != nil {
			return err
		}
		return setKV(ctx, tx, "hub_epoch", seen)
	})
	if err != nil {
		t.log.Error("could not catch up with restored hub", "err", err)
		return false
	}
	t.log.Warn("hub was restored from a backup: downloading everything again", "resent", resent, "backup_at", takenAt)
	t.updateStatus(func(s *Status) { s.Resent = int(resent) })
	t.refreshCounts(ctx)
	t.Nudge()
	return true
}

// UpdateOffer asks the hub which release it holds for this till.
func (t *Terminal) UpdateOffer(ctx context.Context) (*protocol.UpdatePackage, error) {
	p, ok := t.Pairing(ctx)
	if !ok {
		return nil, errors.New("this till is not paired")
	}
	var pkg *protocol.UpdatePackage
	err := t.client.do(ctx, 15*time.Second, http.MethodGet, p.HubURL+"/api/update?platform="+url.QueryEscape(Platform()), p.token, "", nil, &pkg)
	if IsOffline(err) {
		return nil, ErrNeedsHub
	}
	return pkg, err
}

// DownloadUpdate fetches the hub's release for this till into dir and
// verifies it (signature and checksum). progress, if set, gets bytes done
// and total. It returns the installer's path.
func (t *Terminal) DownloadUpdate(ctx context.Context, dir string, progress func(done, total int64)) (string, *update.Manifest, error) {
	pkg, err := t.UpdateOffer(ctx)
	if err != nil {
		return "", nil, err
	}
	if pkg == nil {
		return "", nil, errors.New("the hub has no update for this till")
	}
	m, _, err := update.ParseManifest([]byte(pkg.Raw), []byte(pkg.Signature))
	if err != nil {
		return "", nil, err
	}
	if m.Platform != Platform() {
		return "", nil, fmt.Errorf("the hub's update is for %s, not this till (%s)", m.Platform, Platform())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	dst := filepath.Join(dir, m.File)
	if f, err := os.Open(dst); err == nil { // already here?
		err = m.CheckFile(f)
		f.Close()
		if err == nil {
			return dst, &m, nil
		}
	}
	p, _ := t.Pairing(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.HubURL+"/api/update/file?platform="+url.QueryEscape(Platform()), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	t.requestHeaders(req.Header)
	resp, err := (&http.Client{Timeout: time.Hour}).Do(req)
	if err != nil {
		return "", nil, ErrNeedsHub
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("the hub answered %s", resp.Status)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", nil, err
	}
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return "", nil, werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, m.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return "", nil, fmt.Errorf("download interrupted: %w", rerr)
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return "", nil, err
	}
	err = m.CheckFile(f)
	f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", nil, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", nil, err
	}
	t.log.Info("update downloaded", "version", m.Version, "path", dst)
	return dst, &m, nil
}

// AdminUpload streams a file (an update package) to an admin endpoint.
func (t *Terminal) AdminUpload(ctx context.Context, path string, body io.Reader, out any) error {
	p, ok := t.Pairing(ctx)
	if !ok {
		return errors.New("this till is not paired")
	}
	t.mu.Lock()
	sess := t.session
	t.mu.Unlock()
	if sess == "" {
		return ErrNeedsHub
	}
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.HubURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("X-Session", sess)
	t.requestHeaders(req.Header)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return ErrNeedsHub
	}
	defer resp.Body.Close()
	t.onHubHeaders(resp.Header)
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
