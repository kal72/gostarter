package handler

import (
	"context"
	"net/http"
	"time"
)

// Liveness allows bootstrap before a validated snapshot exists.
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		problem(w, 405, "method_not_allowed", "Use GET or HEAD")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "alive"})
}

// Readiness requires an archive validated under current engine authority.
func (h *APIHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		problem(w, 405, "method_not_allowed", "Use GET or HEAD")
		return
	}
	if !h.available(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if _, _, err := h.active(ctx); err != nil {
		problem(w, 503, "not_ready", "A compatible validated active snapshot is required")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}
