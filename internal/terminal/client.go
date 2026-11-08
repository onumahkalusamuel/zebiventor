package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
}

func newClient() *client {
	return &client{http: &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 40 * time.Second, // long-poll waits up to 30s
	}}}
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
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("hub unreachable: %w", err)
	}
	defer resp.Body.Close()
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
