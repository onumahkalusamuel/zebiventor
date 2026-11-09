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

	h.backOfficeRoutes(mux)
	return recoverer(h, mux)
}

// backOfficeRoutes registers the admin API used by the back-office screens.
// Every handler checks the session's role itself.
func (h *Hub) backOfficeRoutes(mux *http.ServeMux) {
	type sh = func(http.ResponseWriter, *http.Request, *Session)
	handle := func(pattern string, fn sh) { mux.Handle(pattern, h.session(fn)) }
	qi := func(r *http.Request, k string) int64 {
		n, _ := strconv.ParseInt(r.URL.Query().Get(k), 10, 64)
		return n
	}
	qs := func(r *http.Request, k string) string { return r.URL.Query().Get(k) }
	ok := map[string]bool{"ok": true}

	// Suppliers & purchasing.
	handle("GET /api/admin/suppliers", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListSuppliers(r.Context(), s)
		respond(w, v, err)
	})
	handle("POST /api/admin/suppliers", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.Supplier
		if readJSON(w, r, &in) {
			id, err := h.SaveSupplier(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("GET /api/admin/purchase-orders", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListPurchaseOrders(r.Context(), s, qs(r, "status"), qs(r, "supplier"))
		respond(w, v, err)
	})
	handle("GET /api/admin/purchase-orders/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.GetPurchaseOrder(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/purchase-orders", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.PurchaseOrderInput
		if readJSON(w, r, &in) {
			id, err := h.SavePurchaseOrder(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("POST /api/admin/purchase-orders/{id}/status", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in struct {
			Status string `json:"status"`
		}
		if readJSON(w, r, &in) {
			respond(w, ok, h.SetPurchaseOrderStatus(r.Context(), s, r.PathValue("id"), in.Status))
		}
	})
	handle("GET /api/admin/receipts", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListGoodsReceipts(r.Context(), s, qs(r, "supplier"))
		respond(w, v, err)
	})
	handle("GET /api/admin/receipts/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.GetGoodsReceipt(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/receipts", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.GoodsReceiptInput
		if readJSON(w, r, &in) {
			id, err := h.ReceiveGoods(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})

	// Money owed to suppliers.
	handle("GET /api/admin/bills", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListBills(r.Context(), s, qs(r, "supplier"), qs(r, "status"))
		respond(w, v, err)
	})
	handle("GET /api/admin/bills/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		b, pays, err := h.GetBill(r.Context(), s, r.PathValue("id"))
		respond(w, map[string]any{"bill": b, "payments": pays}, err)
	})
	handle("POST /api/admin/bills", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.SupplierBillInput
		if readJSON(w, r, &in) {
			id, err := h.CreateBill(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("POST /api/admin/bills/{id}/void", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in struct {
			Reason string `json:"reason"`
		}
		if readJSON(w, r, &in) {
			respond(w, ok, h.VoidBill(r.Context(), s, r.PathValue("id"), in.Reason))
		}
	})
	handle("GET /api/admin/supplier-payments", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListSupplierPayments(r.Context(), s, qs(r, "supplier"))
		respond(w, v, err)
	})
	handle("GET /api/admin/supplier-payments/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.GetSupplierPayment(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/supplier-payments", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.SupplierPaymentInput
		if readJSON(w, r, &in) {
			id, err := h.PaySupplier(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("GET /api/admin/suppliers/{id}/statement", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.SupplierStatement(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("GET /api/admin/reports/payables", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.PayablesReport(r.Context(), s)
		respond(w, v, err)
	})

	// Locations, transfers, counts, ledger.
	handle("GET /api/admin/locations", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListLocations(r.Context(), s)
		respond(w, v, err)
	})
	handle("POST /api/admin/locations", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.Location
		if readJSON(w, r, &in) {
			id, err := h.SaveLocation(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("GET /api/admin/transfers", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListTransfers(r.Context(), s, qs(r, "status"))
		respond(w, v, err)
	})
	handle("POST /api/admin/transfers/{id}/receive", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.TransferReceipt
		if readJSON(w, r, &in) {
			respond(w, ok, h.ReceiveTransfer(r.Context(), s, r.PathValue("id"), in))
		}
	})
	handle("POST /api/admin/transfers/{id}/cancel", func(w http.ResponseWriter, r *http.Request, s *Session) {
		respond(w, ok, h.CancelTransfer(r.Context(), s, r.PathValue("id")))
	})
	handle("GET /api/admin/transfers/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.GetTransfer(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/transfers", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in TransferInput
		if readJSON(w, r, &in) {
			id, err := h.TransferStock(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("GET /api/admin/counts", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ListStockCounts(r.Context(), s)
		respond(w, v, err)
	})
	handle("POST /api/admin/counts", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in StockCountInput
		if readJSON(w, r, &in) {
			id, err := h.CreateStockCount(r.Context(), s, in)
			respond(w, map[string]string{"id": id}, err)
		}
	})
	handle("GET /api/admin/counts/{id}", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.GetStockCount(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/counts/{id}/entries", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.StockCountEntry
		if readJSON(w, r, &in) {
			v, err := h.CountEntry(r.Context(), s, r.PathValue("id"), in)
			respond(w, v, err)
		}
	})
	handle("POST /api/admin/counts/{id}/post", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.StockCountPost
		if readJSON(w, r, &in) {
			respond(w, ok, h.PostStockCount(r.Context(), s, r.PathValue("id"), in))
		}
	})
	handle("POST /api/admin/counts/{id}/cancel", func(w http.ResponseWriter, r *http.Request, s *Session) {
		respond(w, ok, h.CancelStockCount(r.Context(), s, r.PathValue("id")))
	})
	handle("GET /api/admin/stock/movements", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.StockMovements(r.Context(), s, qs(r, "variant"), qs(r, "location"), int(qi(r, "limit")))
		respond(w, v, err)
	})

	// Customer accounts.
	handle("GET /api/admin/customers/{id}/statement", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.CustomerStatement(r.Context(), s, r.PathValue("id"))
		respond(w, v, err)
	})
	handle("POST /api/admin/customers/{id}/adjust", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in protocol.BalanceAdjustment
		if readJSON(w, r, &in) {
			respond(w, ok, h.AdjustCustomerBalance(r.Context(), s, r.PathValue("id"), in))
		}
	})

	// Reports.
	handle("GET /api/admin/reports/sales", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.SalesReport(r.Context(), s, qi(r, "from"), qi(r, "to"), qs(r, "group"))
		respond(w, v, err)
	})
	handle("GET /api/admin/reports/stock", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.StockReport(r.Context(), s, StockReportFilter{LocationID: qs(r, "location"), CategoryID: qs(r, "category"),
			Query: qs(r, "q"), Only: qs(r, "only"), Limit: int(qi(r, "limit"))})
		respond(w, v, err)
	})
	handle("GET /api/admin/reports/shifts", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ShiftReport(r.Context(), s, qi(r, "from"), qi(r, "to"))
		respond(w, v, err)
	})
	handle("GET /api/admin/audit", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.AuditLog(r.Context(), s, qi(r, "from"), qi(r, "to"), qs(r, "action"))
		respond(w, v, err)
	})

	// Catalogue import / export.
	handle("POST /api/admin/import/products", func(w http.ResponseWriter, r *http.Request, s *Session) {
		var in ImportRequest
		if readJSON(w, r, &in) {
			v, err := h.ImportProducts(r.Context(), s, in)
			respond(w, v, err)
		}
	})
	handle("GET /api/admin/export/products", func(w http.ResponseWriter, r *http.Request, s *Session) {
		v, err := h.ExportProducts(r.Context(), s, qs(r, "location"))
		respond(w, map[string]string{"csv": v}, err)
	})
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
