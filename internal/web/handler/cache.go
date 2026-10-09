package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
)

func matchesETag(header, etag string) bool {
	for _, token := range strings.Split(header, ",") {
		token = strings.TrimSpace(token)
		if token == "*" || strings.TrimPrefix(token, "W/") == etag {
			return true
		}
	}
	return false
}
func cachedAssets(files fs.FS) http.Handler {
	tags := map[string]string{}
	_ = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		tags["/"+name] = `"` + hex.EncodeToString(sum[:]) + `"`
		return nil
	})
	server := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tag, ok := tags[r.URL.Path]; ok && (r.Method == "GET" || r.Method == "HEAD") {
			w.Header().Set("Cache-Control", "public, no-cache")
			w.Header().Set("ETag", tag)
			if matchesETag(r.Header.Get("If-None-Match"), tag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		server.ServeHTTP(w, r)
	})
}
