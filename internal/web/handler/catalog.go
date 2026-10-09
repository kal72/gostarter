package handler

import "net/http"

func (h *APIHandler) HandleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "method_not_allowed", "Use GET")
		return
	}
	if !h.available(w) {
		return
	}
	body, etag, err := h.projects().CachedCatalog(r.Context())
	if err != nil {
		backendError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
