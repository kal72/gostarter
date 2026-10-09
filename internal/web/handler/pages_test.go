package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLRoutesAndLocalAssets(t *testing.T) {
	router := SetupRouter(New(Dependencies{}))
	for _, path := range []string{"/", "/index.html", "/admin", "/admin/", "/admin.html", "/css/tailwind.css", "/vendor/htmx.min.js", "/vendor/alpine.min.js"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		if path == "/" && (strings.Contains(rec.Body.String(), "cdn.tailwindcss.com") || !strings.Contains(rec.Body.String(), `x-data="shell"`)) {
			t.Fatal("page is not using local assets/Alpine")
		}
	}
	for _, path := range []string{"/unknown", "/api/v1/unknown", "/js/missing.js", "/admin.html/other"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 404 {
			t.Fatalf("%s returned HTML fallback: %d", path, rec.Code)
		}
	}
}
func TestUIAdminProtectionAndOrigin(t *testing.T) {
	router := SetupRouter(New(Dependencies{}))
	for _, action := range []string{"settings", "sync", "password", "rollback", "logout"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "http://example.com/ui/admin/"+action, strings.NewReader("default_protocol=dual"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("HX-Request", "true")
		router.ServeHTTP(rec, req)
		if rec.Code != 204 || rec.Header().Get("HX-Redirect") != "/admin" {
			t.Fatalf("unprotected %s: %d", action, rec.Code)
		}
	}
	for _, origin := range []string{"", "null", "https://attacker.test", "http://example.com.attacker.test"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "http://example.com/ui/admin/login", strings.NewReader("username=admin&password=secret"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", origin)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("origin %q accepted: %d", origin, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://example.com/ui/admin/login", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(rec, req)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "temporarily unavailable") {
		t.Fatalf("expected HTML failure: %d", rec.Code)
	}
}
