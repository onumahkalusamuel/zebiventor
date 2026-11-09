package hub

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"testing"
)

// fakeSMTP accepts one message and returns its data.
func fakeSMTP(t *testing.T) (port int, got chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { io.WriteString(c, s+"\r\n") }
		say("220 fake ESMTP")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "DATA"):
				say("354 go on")
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					data.WriteString(strings.TrimPrefix(l, "."))
				}
				say("250 queued")
				got <- data.String()
			case strings.HasPrefix(cmd, "QUIT"):
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSendEmail(t *testing.T) {
	ctx := context.Background()
	h, err := Open(ctx, Config{DataDir: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Setup(ctx, SetupInput{StoreName: "Zebi Books", AdminName: "Ada", AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	doc := `<!doctype html><html><body><h1>Purchase order PO-000001</h1></body></html>`
	if err := h.SendEmail(ctx, SystemSession, Email{To: []string{"x@example.com"}, Subject: "PO", HTML: doc}); err == nil {
		t.Fatal("sent without a mail server set")
	}
	port, got := fakeSMTP(t)
	if err := h.SaveEmailSettings(ctx, SystemSession, EmailSettings{Host: "127.0.0.1", Port: port, Security: "none",
		From: "orders@zebibooks.ng", FromName: "Zebi Books", Password: "app-password"}); err != nil {
		t.Fatal(err)
	}
	if s, _ := h.GetEmailSettings(ctx, SystemSession); s.Password != "" || !s.PasswordSet {
		t.Fatalf("password leaked or lost: %+v", s)
	}
	if err := h.SendEmail(ctx, SystemSession, Email{To: []string{"bad address"}, Subject: "x"}); err == nil {
		t.Fatal("bad address accepted")
	}
	err = h.SendEmail(ctx, SystemSession, Email{To: []string{"Tunde <tunde@lagosbooks.ng>, ops@lagosbooks.ng"}, Subject: "Purchase order PO-000001 — Zebi Books",
		Text: "Hello Tunde,\nPlease find our order below.", HTML: doc, Attachment: "Purchase order PO-000001.html", Kind: "purchase_order", Ref: "PO-000001"})
	if err != nil {
		t.Fatal(err)
	}
	raw := <-got
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	dec := new(mime.WordDecoder)
	subj, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	if subj != "Purchase order PO-000001 — Zebi Books" || !strings.Contains(msg.Header.Get("To"), "ops@lagosbooks.ng") {
		t.Fatalf("headers: %v", msg.Header)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var parts []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ct := p.Header.Get("Content-Type")
		parts = append(parts, ct+"|"+p.FileName())
		if strings.HasPrefix(ct, "multipart/alternative") {
			_, ap, _ := mime.ParseMediaType(ct)
			ar := multipart.NewReader(p, ap["boundary"])
			for {
				q, err := ar.NextPart()
				if err != nil {
					break
				}
				b, _ := io.ReadAll(q) // quoted-printable decoded by the reader
				parts = append(parts, q.Header.Get("Content-Type")+"|"+strconv.Itoa(len(b)))
				if strings.HasPrefix(q.Header.Get("Content-Type"), "text/html") && !strings.Contains(string(b), "Please find our order below.") {
					t.Fatalf("html body lacks the note: %s", b)
				}
			}
		}
	}
	if len(parts) != 4 || !strings.HasSuffix(parts[3], "|Purchase order PO-000001.html") {
		t.Fatalf("parts: %v", parts)
	}
}
