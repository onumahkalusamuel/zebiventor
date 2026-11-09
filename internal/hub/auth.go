package hub

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"golang.org/x/crypto/bcrypt"
)

const (
	pairingCodeTTL = 10 * time.Minute
	sessionTTL     = 12 * time.Hour
)

// Errors surfaced to API clients.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("you do not have permission to do that")
	ErrBadCode      = errors.New("pairing code is invalid or has expired")
	ErrRateLimited  = errors.New("too many attempts, wait a minute and try again")
	ErrBadLogin     = errors.New("incorrect username or password")
)

// validationError marks client mistakes (HTTP 400) as opposed to server faults.
type validationError struct{ msg string }

func (e validationError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return validationError{fmt.Sprintf(format, a...)} }

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomDigits(n int) string {
	var sb strings.Builder
	for range n {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic(err)
		}
		sb.WriteByte(byte('0' + d.Int64()))
	}
	return sb.String()
}

// Terminal is an authenticated, paired till.
type Terminal struct {
	ID            string
	Name          string
	LocationID    string
	ReceiptPrefix string
}

// Session is an authenticated user acting through a terminal.
type Session struct {
	UserID string
	Name   string
	Role   string
}

// CreatePairingCode issues a single-use 6-digit code that lets a new till
// join this hub. createdBy is the issuing user id (may be "" for CLI use).
func (h *Hub) CreatePairingCode(ctx context.Context, createdBy string, req protocol.PairingCodeRequest) (protocol.PairingCode, error) {
	if !h.IsSetup(ctx) {
		return protocol.PairingCode{}, invalid("hub is not set up yet")
	}
	loc := req.LocationID
	if loc == "" {
		loc, _ = h.Setting(ctx, "default_location_id")
	}
	code := randomDigits(6)
	exp := time.Now().Add(pairingCodeTTL).UnixMilli()
	err := h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM locations WHERE id=? AND active=1", loc).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return invalid("unknown location")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO pairing_codes(code_hash,terminal_name,location_id,created_by,expires_at) VALUES(?,?,?,?,?)`,
			hashToken(code), strings.TrimSpace(req.TerminalName), loc, createdBy, exp); err != nil {
			return err
		}
		audit(ctx, tx, createdBy, "", "pairing_code.create", "location", loc, nil)
		return nil
	})
	if err != nil {
		return protocol.PairingCode{}, err
	}
	return protocol.PairingCode{Code: code, ExpiresAt: exp}, nil
}

// Pair redeems a pairing code and registers a new terminal.
func (h *Hub) Pair(ctx context.Context, clientIP string, req protocol.PairRequest) (protocol.PairResponse, error) {
	if h.pairLimiter.blocked(clientIP) {
		return protocol.PairResponse{}, ErrRateLimited
	}
	code := strings.TrimSpace(req.Code)
	token := randomToken()
	var resp protocol.PairResponse
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		var name, loc string
		var exp, used int64
		err := tx.QueryRowContext(ctx, `SELECT terminal_name, location_id, expires_at, used_at FROM pairing_codes WHERE code_hash=?`,
			hashToken(code)).Scan(&name, &loc, &exp, &used)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (used != 0 || exp < now)) {
			return ErrBadCode
		}
		if err != nil {
			return err
		}
		if n := strings.TrimSpace(req.TerminalName); n != "" {
			name = n
		}
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM terminals").Scan(&count); err != nil {
			return err
		}
		if name == "" {
			name = fmt.Sprintf("Till %d", count+1)
		}
		// Receipt prefixes are unique forever so receipt numbers made offline never collide.
		prefix := ""
		for i := count + 1; ; i++ {
			p := fmt.Sprintf("T%02d", i)
			var exists int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM terminals WHERE receipt_prefix=?", p).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				prefix = p
				break
			}
		}
		id := ids.New()
		if _, err := tx.ExecContext(ctx, `INSERT INTO terminals(id,name,location_id,receipt_prefix,token_hash,active,paired_at,last_seen_at,last_ip,app_version)
			VALUES(?,?,?,?,?,1,?,?,?,?)`, id, name, loc, prefix, hashToken(token), now, now, clientIP, req.AppVersion); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE pairing_codes SET used_at=? WHERE code_hash=?", now, hashToken(code)); err != nil {
			return err
		}
		audit(ctx, tx, "", id, "terminal.pair", "terminal", id, map[string]any{"name": name, "ip": clientIP})
		var hubID, storeName string
		tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='hub_id'").Scan(&hubID)
		tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='store_name'").Scan(&storeName)
		resp = protocol.PairResponse{
			HubID: hubID, StoreName: storeName, TerminalID: id, TerminalName: name,
			Token: token, ReceiptPrefix: prefix, LocationID: loc,
		}
		return nil
	})
	if errors.Is(err, ErrBadCode) {
		h.pairLimiter.fail(clientIP)
	}
	return resp, err
}

// AuthTerminal resolves a bearer token to an active terminal.
func (h *Hub) AuthTerminal(ctx context.Context, token, clientIP, appVersion string, encrypted bool) (*Terminal, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	t := &Terminal{}
	err := h.db.R().QueryRowContext(ctx, `SELECT id,name,location_id,receipt_prefix FROM terminals WHERE token_hash=? AND active=1`,
		hashToken(token)).Scan(&t.ID, &t.Name, &t.LocationID, &t.ReceiptPrefix)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	h.touchTerminal(t.ID, clientIP, appVersion, encrypted)
	return t, nil
}

// touchTerminal records last-seen at most every 30s per terminal so that
// frequent long-polls don't generate a write per request.
func (h *Hub) touchTerminal(id, ip, version string, encrypted bool) {
	h.seenMu.Lock()
	last := h.seen[id]
	if time.Since(last) < 30*time.Second {
		h.seenMu.Unlock()
		return
	}
	h.seen[id] = time.Now()
	h.seenMu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "UPDATE terminals SET last_seen_at=?, last_ip=?, app_version=COALESCE(NULLIF(?,''),app_version), encrypted=? WHERE id=?",
				ids.NowMs(), ip, version, encrypted, id)
			return err
		})
	}()
}

// Login verifies credentials and opens a hub session for admin operations.
func (h *Hub) Login(ctx context.Context, t *Terminal, clientIP string, req protocol.LoginRequest) (protocol.LoginResponse, error) {
	key := clientIP + "|" + strings.ToLower(req.Username)
	if h.loginLimiter.blocked(key) {
		return protocol.LoginResponse{}, ErrRateLimited
	}
	u, err := h.userByUsername(ctx, req.Username)
	if err != nil || !u.Active || !checkSecret(u, req.Password) {
		h.loginLimiter.fail(key)
		return protocol.LoginResponse{}, ErrBadLogin
	}
	token := randomToken()
	exp := time.Now().Add(sessionTTL).UnixMilli()
	err = h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,terminal_id,created_at,expires_at) VALUES(?,?,?,?,?)`,
			hashToken(token), u.ID, t.ID, ids.NowMs(), exp)
		return err
	})
	if err != nil {
		return protocol.LoginResponse{}, err
	}
	u.PasswordHash, u.PinHash = "", ""
	return protocol.LoginResponse{SessionToken: token, ExpiresAt: exp, User: u}, nil
}

// checkSecret accepts either the password or the PIN.
func checkSecret(u protocol.SyncUser, secret string) bool {
	if secret == "" {
		return false
	}
	if u.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(secret)) == nil {
		return true
	}
	return u.PinHash != "" && bcrypt.CompareHashAndPassword([]byte(u.PinHash), []byte(secret)) == nil
}

// Logout ends a session.
func (h *Hub) Logout(ctx context.Context, token string) error {
	return h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", hashToken(token))
		return err
	})
}

// AuthSession resolves a session token issued to terminal t.
func (h *Hub) AuthSession(ctx context.Context, t *Terminal, token string) (*Session, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	s := &Session{}
	err := h.db.R().QueryRowContext(ctx, `SELECT u.id, u.name, u.role FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=? AND s.terminal_id=? AND s.expires_at>? AND u.active=1`,
		hashToken(token), t.ID, ids.NowMs()).Scan(&s.UserID, &s.Name, &s.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	return s, err
}

// Can reports whether the session's role is one of roles.
func (s *Session) Can(roles ...string) bool {
	for _, r := range roles {
		if s.Role == r {
			return true
		}
	}
	return false
}

func (h *Hub) userByUsername(ctx context.Context, username string) (protocol.SyncUser, error) {
	var u protocol.SyncUser
	err := h.db.R().QueryRowContext(ctx, `SELECT id,name,username,role,password_hash,pin_hash,active FROM users WHERE username=?`,
		strings.TrimSpace(username)).Scan(&u.ID, &u.Name, &u.Username, &u.Role, &u.PasswordHash, &u.PinHash, &u.Active)
	return u, err
}

// ListTerminals returns all paired terminals.
func (h *Hub) ListTerminals(ctx context.Context) ([]protocol.TerminalInfo, error) {
	rows, err := h.db.R().QueryContext(ctx, `SELECT t.id,t.name,t.location_id,COALESCE(l.name,''),t.receipt_prefix,t.active,t.paired_at,t.last_seen_at,t.last_ip,t.app_version,t.encrypted
		FROM terminals t LEFT JOIN locations l ON l.id=t.location_id ORDER BY t.receipt_prefix`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.TerminalInfo{}
	for rows.Next() {
		var t protocol.TerminalInfo
		if err := rows.Scan(&t.ID, &t.Name, &t.LocationID, &t.LocationName, &t.ReceiptPrefix, &t.Active, &t.PairedAt, &t.LastSeenAt, &t.LastIP, &t.AppVersion, &t.Encrypted); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeTerminal disables a terminal; its token stops working immediately.
func (h *Hub) RevokeTerminal(ctx context.Context, by *Session, id string) error {
	return h.db.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "UPDATE terminals SET active=0 WHERE id=?", id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return invalid("terminal not found")
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE terminal_id=?", id); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "terminal.revoke", "terminal", id, nil)
		return nil
	})
}
