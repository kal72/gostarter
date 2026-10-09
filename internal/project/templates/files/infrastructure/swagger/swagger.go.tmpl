// Package swagger serves Swagger UI alongside the generated OpenAPI spec.
//
// Swagger UI assets (index.html) are embedded from infrastructure/swagger/static/.
// The OpenAPI spec is embedded from gen/openapi/ and passed in via Handler(),
// so this package has no dependency on the spec's location.
package swagger

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// Handler returns an http.Handler that serves:
//   - GET /swagger/           → Swagger UI (index.html)
//   - GET /swagger/spec.json  → OpenAPI spec
//
// spec is the raw JSON bytes of the OpenAPI spec — pass openapi.Spec from
// gen/openapi package.
func Handler(spec []byte) http.Handler {
	mux := http.NewServeMux()

	// Serve the spec at a stable URL that index.html points to.
	mux.HandleFunc("/swagger/spec.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(spec)
	})

	// Serve static Swagger UI assets (index.html, etc.)
	staticSub, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(staticSub))))

	return mux
}
