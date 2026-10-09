package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gostarter/assets"
	"net/http"
)

// SetupRouter separates HTML, fragment, JSON, and static asset routes.
func SetupRouter(api *APIHandler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.Recoverer)
	router.Get("/healthz", api.HandleHealth)
	router.Get("/readyz", api.HandleReady)
	router.Get("/", api.homePage)
	router.Get("/index.html", api.homePage)
	for _, path := range []string{"/admin", "/admin/", "/admin.html"} {
		router.Get(path, api.adminPage)
	}
	for path, handle := range map[string]http.HandlerFunc{
		"/api/v1/resolve":        api.HandleResolve,
		"/api/v1/plans/{planID}": api.HandlePlan,
		"/api/v1/info":           api.HandleInfo,
		"/api/v1/generate":       api.HandleGenerate,
		"/api/v1/explore":        api.HandleExplore,
		"/starter.zip":           api.HandleStarterZip,
		"/api/v1/admin/login":    api.HandleAdminLogin,
	} {
		router.HandleFunc(path, WithCORS(handle))
	}
	router.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(WithCORS(api.RequireAdminAuth(next.ServeHTTP)))
		})
		r.HandleFunc("/api/v1/admin/logout", api.HandleAdminLogout)
		r.HandleFunc("/api/v1/admin/me", api.HandleAdminMe)
		r.HandleFunc("/api/v1/admin/change-password", api.HandleAdminChangePassword)
		r.HandleFunc("/api/v1/admin/stats", api.HandleAdminStats)
		r.HandleFunc("/api/v1/admin/settings", api.HandleAdminSettings)
		r.HandleFunc("/api/v1/admin/sync", api.HandleSync)
		r.HandleFunc("/api/v1/admin/rollback", api.HandleRollback)
	})
	router.Route("/ui/admin", func(r chi.Router) {
		r.Use(sameOriginUI)
		r.Post("/login", api.uiLogin)
		r.Group(func(r chi.Router) {
			r.Use(api.requireUIAdmin)
			r.Post("/logout", api.uiLogout)
			r.Post("/settings", api.uiSettings)
			r.Post("/password", api.uiPassword)
			r.Post("/sync", api.uiSync)
			r.Post("/rollback", api.uiRollback)
			r.Get("/status", api.uiStatus)
			r.Get("/snapshots", api.uiStatus)
		})
	})
	staticHandler := cachedAssets(assets.Assets)
	for _, path := range []string{"/css/*", "/js/*", "/vendor/*"} {
		router.Handle(path, staticHandler)
	}
	return router
}
