package hub

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
)

// EmailSettings are the SMTP details the hub sends documents with. The
// password never leaves the hub; PasswordSet says whether one is stored.
type EmailSettings struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Security    string `json:"security"` // starttls, tls or none
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"` // write-only ("" = unchanged)
	PasswordSet bool   `json:"password_set"`
	From        string `json:"from"`
	FromName    string `json:"from_name"`
}

// Email is a message with an optional HTML document attached.
type Email struct {
	To         []string `json:"to"`
	Cc         []string `json:"cc"`
	Subject    string   `json:"subject"`
	Text       string   `json:"text"`       // the covering note
	HTML       string   `json:"html"`       // shown as the message body under the note
	Attachment string   `json:"attachment"` // file name for HTML as an attachment ("" = none)
	Kind       string   `json:"kind"`       // for the activity log
	Ref        string   `json:"ref"`
}

func (h *Hub) emailSettings(ctx context.Context) EmailSettings {
	get := func(k string) string { v, _ := h.Setting(ctx, k); return v }
	port, _ := strconv.Atoi(get("smtp_port"))
	return EmailSettings{Host: get("smtp_host"), Port: port, Security: get("smtp_security"), Username: get("smtp_user"),
		Password: get("smtp_pass"), From: get("smtp_from"), FromName: get("smtp_from_name")}
}

// GetEmailSettings returns the SMTP settings without the password.
func (h *Hub) GetEmailSettings(ctx context.Context, by *Session) (EmailSettings, error) {
	if !by.Can(adminRoles...) {
		return EmailSettings{}, ErrForbidden
	}
	s := h.emailSettings(ctx)
	s.PasswordSet, s.Password = s.Password != "", ""
	if s.Security == "" {
		s.Security = "starttls"
	}
	if s.FromName == "" {
		s.FromName, _ = h.Setting(ctx, "store_name")
	}
	return s, nil
}

// SaveEmailSettings stores the SMTP settings.
func (h *Hub) SaveEmailSettings(ctx context.Context, by *Session, in EmailSettings) error {
	if !by.Can(adminRoles...) {
		return ErrForbidden
	}
	in.Host = strings.TrimSpace(in.Host)
	in.From = strings.TrimSpace(in.From)
	if in.Host != "" {
		if in.Port <= 0 || in.Port > 65535 {
			return invalid("enter the mail server's port (usually 587)")
		}
		if _, err := mail.ParseAddress(in.From); err != nil {
			return invalid("enter the address emails come from")
		}
	}
	switch in.Security {
	case "starttls", "tls", "none":
	default:
		return invalid("security must be starttls, tls or none")
	}
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		kv := map[string]string{"smtp_host": in.Host, "smtp_port": strconv.Itoa(in.Port), "smtp_security": in.Security,
			"smtp_user": strings.TrimSpace(in.Username), "smtp_from": in.From, "smtp_from_name": strings.TrimSpace(in.FromName)}
		if in.Password != "" {
			kv["smtp_pass"] = in.Password
		}
		for k, v := range kv {
			if err := putSetting(ctx, tx, k, v, now); err != nil {
				return err
			}
		}
		audit(ctx, tx, by.UserID, "", "email.settings", "", "", map[string]any{"host": in.Host, "from": in.From})
		return nil
	})
}

// SendEmail sends a message through the store's mail server.
func (h *Hub) SendEmail(ctx context.Context, by *Session, e Email) error {
	if !by.Can(managerRoles...) {
		return ErrForbidden
	}
	cfg := h.emailSettings(ctx)
	if cfg.Host == "" {
		return invalid("email is not set up yet: an admin can add the mail server in Settings → Email")
	}
	if cfg.FromName == "" {
		cfg.FromName, _ = h.Setting(ctx, "store_name")
	}
	clean := func(list []string) ([]string, error) {
		var out []string
		for _, a := range list {
			for part := range strings.SplitSeq(a, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				addr, err := mail.ParseAddress(part)
				if err != nil {
					return nil, invalid("%q is not an email address", part)
				}
				out = append(out, addr.Address)
			}
		}
		return out, nil
	}
	var err error
	if e.To, err = clean(e.To); err != nil {
		return err
	}
	if e.Cc, err = clean(e.Cc); err != nil {
		return err
	}
	if len(e.To) == 0 {
		return invalid("enter who to send it to")
	}
	if strings.TrimSpace(e.Subject) == "" {
		return invalid("enter a subject")
	}
	msg, err := buildMessage(cfg, e)
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := sendSMTP(sendCtx, cfg, append(append([]string{}, e.To...), e.Cc...), msg); err != nil {
		h.log.Warn("email failed", "to", e.To, "err", err)
		return invalid("the email could not be sent: %v", err)
	}
	h.log.Info("email sent", "to", e.To, "subject", e.Subject)
	return h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		audit(ctx, tx, by.UserID, "", "email.send", e.Kind, e.Ref, map[string]any{"to": e.To, "cc": e.Cc, "subject": e.Subject})
		return nil
	})
}

func buildMessage(cfg EmailSettings, e Email) ([]byte, error) {
	var buf bytes.Buffer
	from := mail.Address{Name: cfg.FromName, Address: cfg.From}
	hdr := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	hdr("From", from.String())
	hdr("To", strings.Join(e.To, ", "))
	if len(e.Cc) > 0 {
		hdr("Cc", strings.Join(e.Cc, ", "))
	}
	hdr("Subject", mime.QEncoding.Encode("utf-8", e.Subject))
	hdr("Date", time.Now().Format(time.RFC1123Z))
	hdr("Message-ID", fmt.Sprintf("<%s@%s>", ids.New(), domainOf(cfg.From)))
	hdr("MIME-Version", "1.0")

	mixed := multipart.NewWriter(&buf)
	hdr("Content-Type", `multipart/mixed; boundary="`+mixed.Boundary()+`"`)
	buf.WriteString("\r\n")

	// Body: the note as text, and the note plus the document as HTML.
	boundary := multipart.NewWriter(io.Discard).Boundary()
	altHdr := textproto.MIMEHeader{}
	altHdr.Set("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	altPart, err := mixed.CreatePart(altHdr)
	if err != nil {
		return nil, err
	}
	alt := multipart.NewWriter(altPart)
	if err := alt.SetBoundary(boundary); err != nil {
		return nil, err
	}
	if err := writeQP(alt, "text/plain; charset=utf-8", e.Text); err != nil {
		return nil, err
	}
	if e.HTML != "" {
		if err := writeQP(alt, "text/html; charset=utf-8", withNote(e.HTML, e.Text)); err != nil {
			return nil, err
		}
	}
	if err := alt.Close(); err != nil {
		return nil, err
	}

	if e.Attachment != "" && e.HTML != "" {
		ah := textproto.MIMEHeader{}
		ah.Set("Content-Type", "text/html; charset=utf-8")
		ah.Set("Content-Transfer-Encoding", "base64")
		ah.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": e.Attachment}))
		w, err := mixed.CreatePart(ah)
		if err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString([]byte(e.HTML))
		for len(enc) > 76 {
			w.Write([]byte(enc[:76] + "\r\n"))
			enc = enc[76:]
		}
		w.Write([]byte(enc + "\r\n"))
	}
	if err := mixed.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeQP(w *multipart.Writer, ctype, body string) error {
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", ctype)
	h.Set("Content-Transfer-Encoding", "quoted-printable")
	p, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	qp := quotedprintable.NewWriter(p)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}

// withNote puts the covering note above the document in the HTML body.
func withNote(doc, note string) string {
	if strings.TrimSpace(note) == "" {
		return doc
	}
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\n", "<br>").Replace(note)
	block := `<div style="max-width:210mm;margin:12px auto;padding:0 14mm;font:11pt/1.5 Segoe UI,Arial,sans-serif;color:#1c1c1a">` + esc + `</div>`
	if i := strings.Index(doc, "<body>"); i >= 0 {
		return doc[:i+6] + block + doc[i+6:]
	}
	return block + doc
}

func domainOf(addr string) string {
	if _, d, ok := strings.Cut(addr, "@"); ok {
		return d
	}
	return "zebiventor.local"
}

// sendSMTP delivers msg. Security: "tls" (port 465), "starttls" (587) or "none".
func sendSMTP(ctx context.Context, cfg EmailSettings, rcpt []string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := net.Dialer{Timeout: 20 * time.Second}
	var conn net.Conn
	var err error
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if cfg.Security == "tls" {
		conn, err = (&tls.Dialer{NetDialer: &d, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if cfg.Security == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("the mail server does not offer STARTTLS; try security \"tls\" on port 465")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if cfg.Username != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return fmt.Errorf("sign-in to the mail server failed: %w", err)
			}
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	for _, r := range rcpt {
		if err := c.Rcpt(r); err != nil {
			return fmt.Errorf("%s: %w", r, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
