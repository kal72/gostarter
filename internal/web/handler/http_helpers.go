package handler

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getSessionToken(r *http.Request) string {
	if cookie, err := r.Cookie("gostarter_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

// WithCORS adds common CORS headers for API endpoints
func WithCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Location, Retry-After, Content-Disposition, X-Plan-ID, X-Content-SHA256")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
		defer cancel()
		next(w, r.WithContext(ctx))
	}
}
