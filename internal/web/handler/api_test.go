package handler_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gostarter/internal/web/handler"
)

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()
	return handler.SetupRouter(handler.New(handler.Dependencies{}))
}

func TestMissingStorageCannotServeGeneration(t *testing.T) {
	router := setupTestServer(t)
	for _, item := range []struct{ method, target, body string }{{"GET", "/api/v1/info", ""}, {"POST", "/api/v1/resolve", `{}`}, {"POST", "/api/v1/generate", `{}`}, {"POST", "/api/v1/explore", `{}`}, {"GET", "/starter.zip?wait=0", ""}} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(item.method, item.target, strings.NewReader(item.body)))
		if rec.Code != 503 || rec.Header().Get("Content-Disposition") != "" || !strings.Contains(rec.Body.String(), "database_unavailable") {
			t.Fatalf("missing storage served %s: %d %s", item.target, rec.Code, rec.Body.String())
		}
	}
}

func TestRejectUnsupportedProtocol(t *testing.T) {
	router := setupTestServer(t)
	check := func(t *testing.T, method, target, body string) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Disposition") != "" {
			t.Fatalf("expected rejection before ZIP headers, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "use dual or grpc") {
			t.Fatalf("expected protocol guidance, got %s", rec.Body.String())
		}
	}
	for _, value := range []string{"rest", " REST ", "unknown"} {
		body, err := json.Marshal(map[string]string{"protocol": value})
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{"/api/v1/generate", "/api/v1/explore"} {
			t.Run(target+"/"+value, func(t *testing.T) { check(t, http.MethodPost, target, string(body)) })
		}
		for _, key := range []string{"protocol", "api"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(key+"/"+method+"/"+value, func(t *testing.T) {
					check(t, method, "/starter.zip?"+url.Values{key: {value}}.Encode(), "")
				})
			}
		}
		encodings := map[string]string{
			"json":   string(body),
			"url":    base64.URLEncoding.EncodeToString(body),
			"rawurl": base64.RawURLEncoding.EncodeToString(body),
			"std":    base64.StdEncoding.EncodeToString(body),
			"rawstd": base64.RawStdEncoding.EncodeToString(body),
		}
		for _, key := range []string{"config", "c", "payload", "data", "b64", "cfg"} {
			for name, encoded := range encodings {
				t.Run(key+"/"+name+"/"+value, func(t *testing.T) {
					check(t, http.MethodGet, "/starter.zip?"+url.Values{key: {encoded}}.Encode(), "")
				})
			}
		}
	}
}

func TestRejectRESTAdminDefault(t *testing.T) {

	api := handler.New(handler.Dependencies{})
	rec := httptest.NewRecorder()
	api.HandleAdminSettings(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"default_protocol":"rest"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 before attempting persistence, got %d", rec.Code)
	}
}

func TestProtocolForms(t *testing.T) {
	router := setupTestServer(t)
	for _, target := range []string{"/", "/admin"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d", target, rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, `value="rest"`) || strings.Contains(body, "REST API Only") {
			t.Errorf("REST-only offered on %s", target)
		}
		if target == "/" && (!strings.Contains(body, `value="dual"`) || !strings.Contains(body, `value="grpc"`)) {
			t.Errorf("supported transports missing on %s", target)
		}
	}
}

func TestAdminEndpoints(t *testing.T) {
	router := setupTestServer(t)

	reqStats := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	recStats := httptest.NewRecorder()
	router.ServeHTTP(recStats, reqStats)
	if recStats.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized from unauthenticated admin stats, got %d", recStats.Code)
	}

	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil)
	recMe := httptest.NewRecorder()
	router.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized from unauthenticated admin me, got %d", recMe.Code)
	}

	reqSettings := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	recSettings := httptest.NewRecorder()
	router.ServeHTTP(recSettings, reqSettings)
	if recSettings.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized from unauthenticated admin settings, got %d", recSettings.Code)
	}

	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(`{}`))
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	if recLogin.Code != http.StatusBadRequest && recLogin.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 400 or 503 from empty login, got %d", recLogin.Code)
	}
}
