package hub

import (
	"net/http"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/templates"
)

// opsRoutes: shop templates, backups, updates and email.
func (h *Hub) opsRoutes(mux *http.ServeMux) {
	admin := func(fn func(http.ResponseWriter, *http.Request, *Session)) http.Handler {
		return h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
			if !s.Can(adminRoles...) {
				respond(w, nil, ErrForbidden)
				return
			}
			fn(w, r, s)
		})
	}
	manager := func(fn func(http.ResponseWriter, *http.Request, *Session)) http.Handler {
		return h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
			if !s.Can(managerRoles...) {
				respond(w, nil, ErrForbidden)
				return
			}
			fn(w, r, s)
		})
	}

	// Shop templates.
	mux.Handle("GET /api/admin/templates", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		respond(w, templates.All(), nil)
	}))
	mux.Handle("POST /api/admin/templates/apply", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req struct {
			Type    string `json:"type"`
			WithTax bool   `json:"with_tax"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		res, err := h.ApplyTemplate(r.Context(), s, req.Type, req.WithTax)
		respond(w, res, err)
	}))

	// Backups.
	mux.Handle("GET /api/admin/backups", manager(func(w http.ResponseWriter, r *http.Request, s *Session) {
		respond(w, h.BackupStatus(r.Context()), nil)
	}))
	mux.Handle("POST /api/admin/backups", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
		_, err := h.Backup(r.Context())
		if err != nil {
			err = invalid("%v", err)
		}
		respond(w, h.BackupStatus(r.Context()), err)
	}))
	mux.Handle("PUT /api/admin/backups/settings", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req struct {
			CopyFolder string `json:"copy_folder"`
			Keep       int    `json:"keep"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if err := h.SaveBackupSettings(r.Context(), s, req.CopyFolder, req.Keep); err != nil {
			respond(w, nil, err)
			return
		}
		respond(w, h.BackupStatus(r.Context()), nil)
	}))

	// Updates: tills (any version) fetch the package the hub holds.
	mux.Handle("GET /api/update", h.terminalAuth(func(w http.ResponseWriter, r *http.Request) {
		pkg, _ := h.AvailableUpdate(r.URL.Query().Get("platform"))
		if pkg == nil {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		respond(w, pkg, nil)
	}, false))
	mux.Handle("GET /api/update/file", h.terminalAuth(func(w http.ResponseWriter, r *http.Request) {
		pkg, path := h.AvailableUpdate(r.URL.Query().Get("platform"))
		if pkg == nil {
			http.NotFound(w, r)
			return
		}
		// Slow links (a branch over the internet) need longer than the default.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Hour))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	}, false))
	mux.Handle("GET /api/admin/update", manager(func(w http.ResponseWriter, r *http.Request, s *Session) {
		respond(w, h.UpdateStatus(r.Context()), nil)
	}))
	mux.Handle("POST /api/admin/update/check", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Hour))
		_, err := h.CheckForUpdates(r.Context())
		if err != nil {
			err = invalid("%v", err)
		}
		respond(w, h.UpdateStatus(r.Context()), err)
	}))
	mux.Handle("POST /api/admin/update/upload", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(time.Hour))
		_, err := h.LoadUpdatePackage(r.Context(), s, r.Body)
		if err != nil {
			respond(w, nil, err)
			return
		}
		respond(w, h.UpdateStatus(r.Context()), nil)
	}))
	mux.Handle("PUT /api/admin/update/settings", admin(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req struct {
			FeedURL string `json:"feed_url"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if err := h.SaveUpdateFeed(r.Context(), s, req.FeedURL); err != nil {
			respond(w, nil, err)
			return
		}
		respond(w, h.UpdateStatus(r.Context()), nil)
	}))

	// Email.
	mux.Handle("GET /api/admin/email", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		resp, err := h.GetEmailSettings(r.Context(), s)
		respond(w, resp, err)
	}))
	mux.Handle("PUT /api/admin/email", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		var req EmailSettings
		if !readJSON(w, r, &req) {
			return
		}
		respond(w, map[string]bool{"ok": true}, h.SaveEmailSettings(r.Context(), s, req))
	}))
	mux.Handle("POST /api/admin/email/send", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
		var req Email
		if !readJSON(w, r, &req) {
			return
		}
		respond(w, map[string]bool{"ok": true}, h.SendEmail(r.Context(), s, req))
	}))
	mux.Handle("GET /api/admin/email/ready", h.session(func(w http.ResponseWriter, r *http.Request, s *Session) {
		host, _ := h.Setting(r.Context(), "smtp_host")
		respond(w, map[string]bool{"ready": host != ""}, nil)
	}))
}
