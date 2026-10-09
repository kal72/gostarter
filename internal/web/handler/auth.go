package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gostarter/internal/auth"
	"net/http"
	"time"
)

// =========================================================================
// Admin Authentication Handlers (Simple Username & Password via Bcrypt)
// =========================================================================

// RequireAdminAuth enforces authentication for protected administrative endpoints
func (h *APIHandler) RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unauthorized: Admin session required",
			})
			return
		}

		if h.authService == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Database unavailable for session verification",
			})
			return
		}

		user, err := h.auth().Session(token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Unauthorized: %v", err),
			})
			return
		}

		ctx := context.WithValue(r.Context(), adminUserContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// HandleAdminLogin authenticates credentials using bcrypt and issues a session token
func (h *APIHandler) HandleAdminLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.authService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Database is currently disconnected",
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request body"})
		return
	}

	user, token, err := h.auth().Login(req.Username, req.Password)
	if err != nil {
		status, message := 401, "Invalid username or password"
		if errors.Is(err, auth.ErrRequiredCredentials) {
			status, message = 400, "Username and password are required"
		}
		if errors.Is(err, auth.ErrSession) {
			status, message = 500, "Failed creating session"
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": message})
		return
	}

	// Set HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "gostarter_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(h.sessionLifetime),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

// HandleAdminLogout terminates active session in database and clears cookie
func (h *APIHandler) HandleAdminLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token != "" && h.authService != nil {
		_ = h.auth().Logout(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "gostarter_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// HandleAdminMe returns current authenticated admin info
func (h *APIHandler) HandleAdminMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user, ok := r.Context().Value(adminUserContextKey).(*auth.AdminUser)
	if !ok || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Unauthorized"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"user":          user,
	})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// HandleAdminChangePassword updates password using bcrypt hashing
func (h *APIHandler) HandleAdminChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := r.Context().Value(adminUserContextKey).(*auth.AdminUser)
	if !ok || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Unauthorized"})
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid payload"})
		return
	}

	if err := h.auth().ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

func (h *APIHandler) auth() *auth.Auth {
	if h.authService != nil {
		return h.authService
	}
	return auth.New(nil, h.sessionLifetime)
}
