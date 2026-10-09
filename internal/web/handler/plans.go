package handler

import (
	"context"
	"errors"
	"gostarter/internal/project"
	"gostarter/internal/project/plan"
	"gostarter/internal/upstream"
	"gostarter/internal/upstream/profile"
	"net/http"
	"strings"
	"time"
)

type planResponse struct {
	ID                 string    `json:"plan_id"`
	Status             string    `json:"status"`
	StatusURL          string    `json:"status_url"`
	ExpiresAt          time.Time `json:"expires_at"`
	Plan               plan.Plan `json:"plan"`
	ContentSHA256      string    `json:"content_sha256,omitempty"`
	ValidationIdentity string    `json:"validation_identity,omitempty"`
	Diagnostic         string    `json:"diagnostic,omitempty"`
	Error              *apiError `json:"error,omitempty"`
}

func responseFor(p project.StoredPlan) planResponse {
	return planResponse{ID: p.ID, Status: p.Status, StatusURL: "/api/v1/plans/" + p.ID, ExpiresAt: p.ExpiresAt, Plan: p.Plan, ContentSHA256: p.ContentSHA256, ValidationIdentity: p.ValidationIdentity, Diagnostic: storageSummary(p.Diagnostic)}
}
func (h *APIHandler) respondPlan(w http.ResponseWriter, r *http.Request, p project.StoredPlan) {
	status, code, err := h.state(r.Context(), p)
	if err != nil {
		backendError(w, err)
		return
	}
	out := responseFor(p)
	if code != "" {
		out.Error = &apiError{code, "Resolve a new plan before generating"}
	}
	if status == 202 {
		w.Header().Set("Location", out.StatusURL)
		w.Header().Set("Retry-After", "1")
	}
	writeJSON(w, status, out)
}
func (h *APIHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "method_not_allowed", "Use POST")
		return
	}
	req, err := readProject(w, r)
	if err != nil {
		problem(w, 400, "invalid_request", err.Error())
		return
	}
	if req.ID != "" {
		problem(w, 400, "invalid_request", "Resolve requires project configuration")
		return
	}
	if !h.available(w) {
		return
	}
	p, err := h.resolve(r.Context(), req)
	if err != nil {
		backendError(w, err)
		return
	}
	h.respondPlan(w, r, p)
}
func (h *APIHandler) HandlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "method_not_allowed", "Use GET")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/")
	if !validPlanID(id) {
		problem(w, 404, "not_found", "Plan was not found")
		return
	}
	if !h.available(w) {
		return
	}
	p, err := h.projects().Get(r.Context(), id)
	if err != nil && !errors.Is(err, project.ErrPlanExpired) {
		backendError(w, err)
		return
	}
	h.respondPlan(w, r, p)
}

func (h *APIHandler) projects() *project.Projects { return h.projectService }
func (h *APIHandler) active(ctx context.Context) (upstream.SnapshotRecord, profile.Source, error) {
	return h.projects().Active(ctx)
}
func (h *APIHandler) resolve(ctx context.Context, req projectRequest) (project.StoredPlan, error) {
	return h.projects().Resolve(ctx, req)
}
func (h *APIHandler) state(ctx context.Context, p project.StoredPlan) (int, string, error) {
	return h.projects().State(ctx, p)
}
