package handler

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticAssetsConditionalRequest(t *testing.T) {
	handler := cachedAssets(fstest.MapFS{"js/app.js": {Data: []byte("hello")}})
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest("GET", "/js/app.js", nil))
	tag := first.Header().Get("ETag")
	if tag == "" || first.Code != 200 || first.Body.String() != "hello" {
		t.Fatal("missing asset or validator")
	}
	request := httptest.NewRequest("GET", "/js/app.js", nil)
	request.Header.Set("If-None-Match", `"other", W/`+tag)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if second.Code != 304 || second.Body.Len() != 0 {
		t.Fatalf("expected empty 304, got %d", second.Code)
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest("GET", "/js/missing.js", nil))
	if missing.Code != 404 || missing.Header().Get("ETag") != "" {
		t.Fatal("missing asset was cached")
	}
}
