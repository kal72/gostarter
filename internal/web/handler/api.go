package handler

import (
	"gostarter/internal/auth"
	"gostarter/internal/project"
	"gostarter/internal/upstream"
	"time"
)

type Dependencies struct {
	Projects        *project.Projects
	Auth            *auth.Auth
	Admin           *upstream.Admin
	Upstream        *upstream.Upstream
	SessionLifetime time.Duration
}
type APIHandler struct {
	projectService  *project.Projects
	authService     *auth.Auth
	adminService    *upstream.Admin
	upstreamService *upstream.Upstream
	sessionLifetime time.Duration
}

func New(deps Dependencies) *APIHandler {
	lifetime := deps.SessionLifetime
	if lifetime <= 0 {
		lifetime = 24 * time.Hour
	}
	return &APIHandler{projectService: deps.Projects, authService: deps.Auth, adminService: deps.Admin, upstreamService: deps.Upstream, sessionLifetime: lifetime}
}

type contextKey string

const adminUserContextKey contextKey = "admin_user"
