package hub

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

const maxBody = 16 << 20

type ctxKey int

const (
	keyTerminal ctxKey = iota
	keySession
)

// Handler returns the hub's HTTP API.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/hub/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, h.Info(r.Context()))
	})
	mux.HandleFunc("POST /api/pair", func(w http.ResponseWriter, r *http.Request) {
		var req protocol.PairRequest
		if !readJSON(w, r, &req) {
			return
		}
		resp, err := h.Pair(r.Context(), clientIP(r), req)
		respond(w, resp, err)
	})

	// Terminal-authenticated.
	mux.Handle("GET /api/sync/pull", h.terminal(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		wait, _ := strconv.Atoi(r.URL.Query().Get("wait"))
		wait = min(max(wait, 0), 30)
		resp, err := h.Pull(r.Context(), since, time.Duration(wait)*time.Second)
		respond(w, resp, err)
	}))
	mux.Handle("POST /api/sync/push", h.terminal(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.PushRequest
		if !readJSON(w, r, &req) {
			return
		}
		resp, err := h.Push(r.Context(), r.Context().Value(keyTerminal).(*Terminal), req)
		respond(w, resp, err)
	}))
	mux.Handle("GET /api/sales/lookup", h.terminal(func(w http.ResponseWriter, r *http.Request) {
		resp, err := h.LookupSale(r.Context(), r.URL.Query().Get("receipt"))
		respond(w, resp, err)
	}))
	mux.Handle("POST /api/auth/login", h.terminal(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.LoginRequest
		if !readJSON(w, r, &req) {
			return
		}
		resp, err := h.Login(r.Context(), r.Context().Value(keyTerminal).(*Terminal), clientIP(r), req)
		respond(w, resp, err)
	}))
	mux.Handle("POST /api/auth/logout", h.terminal(func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]bool{"ok": true}, h.Logout(r.Context(), r.Header.Get("X-Session")))
	}))

	// Terminal + user session.
	mux.Handle("POST /api/admin/pairing-codes", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		if !s.Can(managerRoles...) {
			respond(w, nil, ErrForbidden)
			return
		}
		var req protocol.PairingCodeRequest
		if !readJSON(w, r, &req) {
			return
		}
		resp, err := h.CreatePairingCode(r.Context(), s.UserID, req)
		respond(w, resp, err)
	}))
	mux.Handle("GET /api/admin/terminals", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		if !s.Can(managerRoles...) {
			respond(w, nil, ErrForbidden)
			return
		}
		resp, err := h.ListTerminals(r.Context())
		respond(w, resp, err)
	}))
	mux.Handle("POST /api/admin/terminals/{id}/revoke", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		if !s.Can(adminRoles...) {
			respond(w, nil, ErrForbidden)
			return
		}
		respond(w, map[string]bool{"ok": true}, h.RevokeTerminal(r.Context(), s, r.PathValue("id")))
	}))
	mux.Handle("POST /api/admin/products", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req protocol.ProductInput
		if !readJSON(w, r, &req) {
			return
		}
		id, err := h.SaveProduct(r.Context(), s, req)
		respond(w, map[string]string{"id": id}, err)
	}))
	mux.Handle("POST /api/admin/categories", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req protocol.Category
		if !readJSON(w, r, &req) {
			return
		}
		id, err := h.SaveCategory(r.Context(), s, req)
		respond(w, map[string]string{"id": id}, err)
	}))
	mux.Handle("POST /api/admin/stock/adjust", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req protocol.StockAdjustRequest
		if !readJSON(w, r, &req) {
			return
		}
		respond(w, map[string]bool{"ok": true}, h.AdjustStock(r.Context(), s, req))
	}))
	mux.Handle("POST /api/admin/users", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req protocol.UserInput
		if !readJSON(w, r, &req) {
			return
		}
		id, err := h.SaveUser(r.Context(), s, req)
		respond(w, map[string]string{"id": id}, err)
	}))
	mux.Handle("POST /api/admin/payment-methods", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req protocol.PaymentMethod
		if !readJSON(w, r, &req) {
			return
		}
		id, err := h.SavePaymentMethod(r.Context(), s, req)
		respond(w, map[string]string{"id": id}, err)
	}))
	mux.Handle("PUT /api/admin/settings", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req map[string]string
		if !readJSON(w, r, &req) {
			return
		}
		respond(w, map[string]bool{"ok": true}, h.SaveSettings(r.Context(), s, req))
	}))

	return recoverer(h, mux)
}

func (h *Hub) terminal(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		t, err := h.AuthTerminal(r.Context(), tok, clientIP(r))
		if err != nil {
			respond(w, nil, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), keyTerminal, t)))
	})
}

func (h *Hub) session(next func(http.ResponseWriter, *http.Request, *Session)) http.Handler {
	return h.terminal(func(w http.ResponseWriter, r *http.Request) {
		t := r.Context().Value(keyTerminal).(*Terminal)
		s, err := h.AuthSession(r.Context(), t, r.Header.Get("X-Session"))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, protocol.APIError{Error: "your session has expired, please sign in again", Code: "session"})
			return
		}
		next(w, r, s)
	})
}

func recoverer(h *Hub, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				h.log.Error("panic in handler", "path", r.URL.Path, "panic", v)
				writeJSON(w, http.StatusInternalServerError, protocol.APIError{Error: "internal error"})
			}
		}()
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.APIError{Error: "invalid request body: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respond(w http.ResponseWriter, v any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, v)
		return
	}
	var ve validationError
	switch {
	case errors.As(err, &ve), errors.Is(err, ErrBadCode):
		writeJSON(w, http.StatusBadRequest, protocol.APIError{Error: err.Error()})
	case errors.Is(err, ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, protocol.APIError{Error: "this till is not paired with the hub", Code: "terminal"})
	case errors.Is(err, ErrBadLogin):
		writeJSON(w, http.StatusUnauthorized, protocol.APIError{Error: err.Error(), Code: "login"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, protocol.APIError{Error: err.Error()})
	case errors.Is(err, ErrRateLimited):
		writeJSON(w, http.StatusTooManyRequests, protocol.APIError{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, protocol.APIError{Error: "hub error: " + err.Error()})
	}
}

// Serve listens on addr until ctx is cancelled, then shuts down gracefully.
// ready, if non-nil, receives the bound address once listening.
func (h *Hub) Serve(ctx context.Context, addr string, ready func(net.Addr)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           h.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second, // > max long-poll wait
		IdleTimeout:       120 * time.Second,
	}
	if ready != nil {
		ready(ln.Addr())
	}
	h.log.Info("hub listening", "addr", ln.Addr().String())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func startOfDayMs() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()
}
