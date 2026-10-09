package handler

import (
	"bytes"
	"context"
	"errors"
	"gostarter/internal/auth"
	"gostarter/internal/upstream"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"gostarter/internal/web/ui/components"
	ui "gostarter/internal/web/ui/pages"
)

func renderHTML(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	var body bytes.Buffer
	if err := component.Render(r.Context(), &body); err != nil {
		log.Printf("render page: %v", err)
		http.Error(w, "Unable to display this page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
func (h *APIHandler) homePage(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, r, 200, ui.HomePage())
}
func (h *APIHandler) sessionUser(r *http.Request) *auth.AdminUser {
	if h.authService == nil {
		return nil
	}
	user, err := h.auth().Session(getSessionToken(r))
	if err != nil {
		return nil
	}
	return user
}
func redirectUI(w http.ResponseWriter, r *http.Request, location string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(204)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}
func (h *APIHandler) adminPage(w http.ResponseWriter, r *http.Request) {
	model := ui.AdminPageModel{User: userView(h.sessionUser(r))}
	if model.User != nil {
		settings, err := h.upstream().Settings(r.Context())
		model.Settings = settingsView(settings)
		if err != nil {
			renderHTML(w, r, 503, components.Notice("Unable to load settings. Please refresh.", true))
			return
		}
		source, err := h.upstream().Overview(r.Context())
		model.Source = sourceView(source)
		if err != nil {
			renderHTML(w, r, 503, components.Notice("Unable to load upstream status. Please refresh.", true))
			return
		}
	}
	renderHTML(w, r, 200, ui.AdminPage(model))
}
func (h *APIHandler) requireUIAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.sessionUser(r)
		if user == nil {
			redirectUI(w, r, "/admin")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminUserContextKey, user)))
	})
}

// HTML forms need same-origin validation independently of JSON API CORS.
func sameOriginUI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Host, r.Host) || (parsed.Scheme != "http" && parsed.Scheme != "https") || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			renderHTML(w, r, 403, components.Notice("Please refresh this page and try again.", true))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			renderHTML(w, r, 400, components.Notice("Invalid form submission.", true))
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (h *APIHandler) uiLogin(w http.ResponseWriter, r *http.Request) {
	if h.authService == nil {
		renderHTML(w, r, 503, components.Notice("Sign in is temporarily unavailable.", true))
		return
	}
	_, token, err := h.auth().Login(r.PostForm.Get("username"), r.PostForm.Get("password"))
	if err != nil {
		status, message := 401, "Invalid username or password."
		if errors.Is(err, auth.ErrRequiredCredentials) {
			status, message = 400, "Username and password are required."
		}
		if errors.Is(err, auth.ErrSession) {
			status, message = 503, "Unable to create a session. Please try again."
		}
		renderHTML(w, r, status, components.Notice(message, true))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "gostarter_session", Value: token, Path: "/", Expires: time.Now().Add(h.sessionLifetime), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	redirectUI(w, r, "/admin")
}
func (h *APIHandler) uiLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth().Logout(getSessionToken(r)); err != nil {
		renderHTML(w, r, 503, components.Notice("Unable to sign out. Please try again.", true))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "gostarter_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	redirectUI(w, r, "/admin")
}
func (h *APIHandler) uiSettings(w http.ResponseWriter, r *http.Request) {
	settings := upstream.AdminSettings{DefaultGroup: strings.TrimSpace(r.PostForm.Get("default_group")), DefaultService: strings.TrimSpace(r.PostForm.Get("default_service")), DefaultGoVersion: r.PostForm.Get("default_go_version"), DefaultProtocol: r.PostForm.Get("default_protocol"), UpstreamRepo: strings.TrimSpace(r.PostForm.Get("upstream_repo")), UpstreamBranch: strings.TrimSpace(r.PostForm.Get("upstream_branch"))}
	if err := h.saveAdminSettings(&settings); err != nil {
		renderHTML(w, r, 400, components.Notice(storageSummary(err.Error()), true))
		return
	}
	w.Header().Set("HX-Trigger", "admin-refresh")
	renderHTML(w, r, 200, components.Notice("Settings saved.", false))
}
func (h *APIHandler) uiStatus(w http.ResponseWriter, r *http.Request) {
	source, err := h.upstream().Overview(r.Context())
	if err != nil {
		renderHTML(w, r, 503, components.Notice("Unable to refresh status.", true))
		return
	}
	if strings.HasSuffix(r.URL.Path, "/snapshots") {
		renderHTML(w, r, 200, components.SnapshotList(sourceView(source)))
		return
	}
	renderHTML(w, r, 200, components.AdminStats(sourceView(source)))
}
func (h *APIHandler) uiSync(w http.ResponseWriter, r *http.Request) {
	if err := h.upstream().Sync(r.Context()); err != nil {
		if errors.Is(err, upstream.ErrSyncBusy) {
			w.Header().Set("HX-Trigger", "admin-refresh")
			renderHTML(w, r, 409, components.Notice("Sync is already queued or running.", true))
			return
		}
		renderHTML(w, r, 503, components.Notice("Unable to request sync. Please try again.", true))
		return
	}
	w.Header().Set("HX-Trigger", "admin-refresh")
	renderHTML(w, r, 202, components.Notice("Sync requested.", false))
}
func (h *APIHandler) uiPassword(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(adminUserContextKey).(*auth.AdminUser)
	password := r.PostForm.Get("new_password")
	if password != r.PostForm.Get("confirm_password") {
		renderHTML(w, r, 400, components.Notice("New passwords do not match.", true))
		return
	}
	if err := h.auth().ChangePassword(user.ID, r.PostForm.Get("current_password"), password); err != nil {
		renderHTML(w, r, 400, components.Notice("Unable to change password. Check your current password and try again.", true))
		return
	}
	w.Header().Set("HX-Trigger", "password-changed")
	renderHTML(w, r, 200, components.Notice("Password updated.", false))
}
func (h *APIHandler) uiRollback(w http.ResponseWriter, r *http.Request) {
	id := r.PostForm.Get("snapshot_id")
	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	epoch, epochErr := strconv.ParseInt(r.PostForm.Get("candidate_epoch"), 10, 64)
	if err != nil || epochErr != nil || revision < 1 || epoch < 0 || !validPlanID(id) {
		renderHTML(w, r, 400, components.Notice("Refresh snapshot status before retrying.", true))
		return
	}
	if h.upstreamService == nil {
		renderHTML(w, r, 503, components.Notice("Snapshot storage is unavailable.", true))
		return
	}
	if err := h.upstream().Rollback(r.Context(), revision, epoch, id); err != nil {
		renderHTML(w, r, 409, components.Notice("Snapshot unavailable or source changed. Refresh and try again.", true))
		return
	}
	w.Header().Set("HX-Trigger", "admin-refresh")
	renderHTML(w, r, 200, components.Notice("Validated snapshot activated.", false))
}
