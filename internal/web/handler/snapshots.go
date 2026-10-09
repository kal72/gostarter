package handler

import (
	"encoding/json"
	"io"
	"net/http"
)

// HandleRollback uses revision + candidate epoch CAS; archive availability and
// current validation authority are verified by storage before activation.
func (h *APIHandler) HandleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "method_not_allowed", "Use POST")
		return
	}
	var input struct {
		SnapshotID     string `json:"snapshot_id"`
		Revision       int64  `json:"revision"`
		CandidateEpoch int64  `json:"candidate_epoch"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		problem(w, 400, "invalid_request", "Invalid rollback request")
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || !validPlanID(input.SnapshotID) || input.Revision < 1 || input.CandidateEpoch < 0 {
		problem(w, 400, "invalid_request", "Snapshot and current source revision/epoch are required")
		return
	}
	if !h.available(w) {
		return
	}
	if err := h.upstream().Rollback(r.Context(), input.Revision, input.CandidateEpoch, input.SnapshotID); err != nil {
		problem(w, 409, "rollback_unavailable", "Snapshot unavailable, incompatible, or source changed. Refresh status before retrying.")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "activated", "snapshot_id": input.SnapshotID})
}
