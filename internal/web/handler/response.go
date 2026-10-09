package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"gostarter/internal/project"
	"gostarter/internal/upstream/manifest"
	"net/http"
	"strings"
	"unicode/utf8"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": apiError{code, storageSummary(message)}})
}
func (h *APIHandler) available(w http.ResponseWriter) bool {
	if h.projectService == nil {
		problem(w, 503, "database_unavailable", "Persistent generation service is unavailable")
		return false
	}
	return true
}

var errSnapshotUnavailable = project.ErrSnapshotUnavailable

func backendError(w http.ResponseWriter, err error) {
	var failure *project.Failure
	if errors.As(err, &failure) {
		problem(w, failure.Status, failure.Code, failure.Message)
		return
	}

	if errors.Is(err, errSnapshotUnavailable) {
		problem(w, 503, "snapshot_unavailable", "No compatible validated active snapshot is available")
		return
	}
	var d *manifest.Diagnostic
	if errors.As(err, &d) {
		diagnostic := *d
		diagnostic.Code = boundedText(d.Code, 128)
		diagnostic.ExtensionID = boundedText(d.ExtensionID, 128)
		diagnostic.File = boundedText(d.File, 512)
		diagnostic.Symbol = boundedText(d.Symbol, 128)
		diagnostic.Cause = boundedText(d.Cause, 384)
		writeJSON(w, 400, map[string]any{"error": apiError{d.Code, storageSummary(d.Cause)}, "diagnostic": diagnostic})
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "not_found", "Requested resource was not found")
		return
	}
	if err.Error() == "validation queue full" {
		problem(w, 503, "validation_queue_full", "Validation queue is full; retry later")
		return
	}
	problem(w, 503, "database_unavailable", "Generation storage is unavailable")
}
func storageSummary(s string) string { return boundedText(s, 8192) }

func boundedText(s string, limit int) string {
	s = strings.ToValidUTF8(s, "�")
	if len(s) > limit {
		s = s[:limit]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}
