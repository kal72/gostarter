package handler

import (
	"encoding/json"
	"errors"
	"gostarter/internal/project/protocol"
	"gostarter/internal/upstream"
	"net/http"
)

// =========================================================================
// Simple Admin Dashboard APIs
// =========================================================================

// HandleAdminStats returns database health and application status
func (h *APIHandler) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.available(w) {
		return
	}
	overview, err := h.upstream().Overview(r.Context())
	if err != nil {
		backendError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"db_connected": true, "source": overview, "sync_mode": "manual"})
}

// HandleAdminSettings retrieves or updates admin settings
func (h *APIHandler) HandleAdminSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		if h.upstreamService == nil {
			json.NewEncoder(w).Encode(upstream.AdminSettings{
				DefaultGroup:     "github.com/example",
				DefaultService:   "my-service",
				DefaultGoVersion: "1.27.1",
				DefaultProtocol:  "dual",
			})
			return
		}

		settings, err := h.upstream().Settings(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(settings)
		return
	}

	if r.Method == http.MethodPost {
		var settings upstream.AdminSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		if err := protocol.Validate(settings.DefaultProtocol); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.saveAdminSettings(&settings); err != nil {
			backendError(w, err)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"settings": settings,
		})
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// HandleSync triggers on-demand sync from GitHub
func (h *APIHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.available(w) {
		return
	}
	if err := h.upstream().Sync(r.Context()); err != nil {
		if errors.Is(err, upstream.ErrSyncBusy) {
			problem(w, 409, "sync_busy", "Sync is already queued or running")
			return
		}
		backendError(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"status": "scheduled"})
}

func (h *APIHandler) saveAdminSettings(settings *upstream.AdminSettings) error {
	return h.admin().SaveSettings(settings)
}
func (h *APIHandler) admin() *upstream.Admin {
	if h.adminService != nil {
		return h.adminService
	}
	return upstream.NewSettings(nil)
}
func (h *APIHandler) upstream() *upstream.Upstream {
	if h.upstreamService != nil {
		return h.upstreamService
	}
	return upstream.New(nil, nil)
}
