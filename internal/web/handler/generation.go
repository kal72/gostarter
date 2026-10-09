package handler

import (
	"encoding/base64"
	"fmt"
	"gostarter/internal/project"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"
)

func (h *APIHandler) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	h.output(w, r, false)
}
func (h *APIHandler) HandleExplore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "method_not_allowed", "Use POST")
		return
	}
	req, err := readProject(w, r)
	if err != nil {
		problem(w, 400, "invalid_request", err.Error())
		return
	}
	if !h.available(w) {
		return
	}
	files, name, err := h.projects().Preview(r.Context(), req)
	if err != nil {
		backendError(w, err)
		return
	}
	view := project.BuildExploreData(files, name)
	binary := map[string]string{}
	for name, data := range files {
		if !utf8.Valid(data) || view.Files[name] != string(data) {
			binary[name] = base64.StdEncoding.EncodeToString(data)
		}
	}
	writeJSON(w, 200, struct {
		*project.ExploreResponse
		PreviewLevel string            `json:"preview_level"`
		BinaryFiles  map[string]string `json:"binary_files,omitempty"`
	}{view, "source", binary})
}
func (h *APIHandler) HandleStarterZip(w http.ResponseWriter, r *http.Request) {
	h.output(w, r, true)
}
func (h *APIHandler) output(w http.ResponseWriter, r *http.Request, query bool) {
	if !query && r.Method != "POST" || query && r.Method != "GET" && r.Method != "HEAD" {
		problem(w, 405, "method_not_allowed", "Unsupported method")
		return
	}
	var req projectRequest
	var err error
	if query {
		if len(r.URL.RawQuery) > 2<<20 {
			problem(w, 400, "invalid_request", "Query exceeds limit")
			return
		}
		values, parseErr := url.ParseQuery(r.URL.RawQuery)
		if parseErr != nil {
			problem(w, 400, "invalid_request", "Malformed query encoding")
			return
		}
		req, err = queryProject(values)
	} else {
		req, err = readProject(w, r)
	}
	if err != nil {
		problem(w, 400, "invalid_request", err.Error())
		return
	}
	wait := 0
	if query {
		wait = 10
	}
	if raw, ok := r.URL.Query()["wait"]; ok {
		if len(raw) != 1 {
			problem(w, 400, "invalid_wait", "Use wait=0..10 seconds")
			return
		}
		wait, err = strconv.Atoi(raw[0])
		if err != nil || wait < 0 || wait > 10 {
			problem(w, 400, "invalid_wait", "Use wait=0..10 seconds")
			return
		}
	}
	if !h.available(w) {
		return
	}
	p, files, err := h.projects().Generate(r.Context(), req, wait)
	if err != nil {
		if r.Context().Err() == nil {
			backendError(w, err)
		}
		return
	}
	if files == nil {
		h.respondPlan(w, r, p)
		return
	}
	w.Header().Set("X-Plan-ID", p.ID)
	w.Header().Set("X-Content-SHA256", p.ContentSHA256)
	w.Header().Set("Cache-Control", "no-store")
	serviceName := p.Plan.Config.ServiceName
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, serviceName))
	if r.Method == "HEAD" {
		w.WriteHeader(200)
		return
	}
	if err = project.CreateZipArchive(files, serviceName, w); err != nil {
		log.Printf("ZIP stream aborted for plan %s", p.ID)
		panic(http.ErrAbortHandler)
	}
}
