package project

import (
	"context"
	"errors"
	"golang.org/x/sync/singleflight"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/project/validation"
	"gostarter/internal/upstream"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
	"sync"
)

var ErrSnapshotUnavailable = errors.New("snapshot unavailable")

type ProjectRequest struct {
	Config   plan.Config
	ID       string
	Explicit bool
	Flags    map[string]bool
}

// ProjectRepository contains metadata and validation authority used by project logic.
type ProjectRepository interface {
	GetSnapshot(context.Context, string) (upstream.SnapshotRecord, error)
	GetUpstream(context.Context, string) (upstream.UpstreamState, error)
	GetPlan(context.Context, string) (StoredPlan, error)
	HasSnapshotValidation(context.Context, string, string) (bool, error)
	HasPlanValidation(context.Context, string, string, string) (bool, error)
	GetSettingsContext(context.Context) (upstream.AdminSettings, error)
}
type ProjectStorage interface {
	Read(context.Context, string) (upstream.SnapshotRecord, profile.Source, error)
	Resolve(context.Context, string, plan.Config) (StoredPlan, error)
}

// Projects orchestrates immutable source selection, dependency resolution and validation.
type Projects struct {
	previewFlights   singleflight.Group
	catalogMu        sync.Mutex
	catalogCache     catalogEntry
	database         ProjectRepository
	store            ProjectStorage
	materializeSlots chan struct{}
}

func New(database ProjectRepository, store ProjectStorage) *Projects {
	return &Projects{database: database, store: store, materializeSlots: make(chan struct{}, 2)}
}
func (h *Projects) Active(ctx context.Context) (upstream.SnapshotRecord, profile.Source, error) {
	state, err := h.database.GetUpstream(ctx, "primary")
	if err != nil {
		return upstream.SnapshotRecord{}, nil, err
	}
	if state.ActiveSnapshotID == "" {
		return upstream.SnapshotRecord{}, nil, ErrSnapshotUnavailable
	}
	record, files, err := h.store.Read(ctx, state.ActiveSnapshotID)
	if err != nil {
		return record, nil, ErrSnapshotUnavailable
	}
	valid, err := h.database.HasSnapshotValidation(ctx, record.Metadata.ID, validation.Key())
	if err != nil {
		return record, nil, err
	}
	if !valid || record.Status != "ready" {
		return record, nil, ErrSnapshotUnavailable
	}
	return record, files, nil
}
func (h *Projects) Resolve(ctx context.Context, req ProjectRequest) (StoredPlan, error) {
	if req.ID != "" {
		return h.database.GetPlan(ctx, req.ID)
	}
	record, files, err := h.Active(ctx)
	if err != nil {
		return StoredPlan{}, err
	}
	p, err := plan.Build(files, record.Metadata.Identity(), req.Config)
	if err != nil {
		return StoredPlan{}, err
	}
	if err := checkSelection(req, p); err != nil {
		return StoredPlan{}, err
	}
	return h.store.Resolve(ctx, record.Metadata.ID, p.Config)
}

func (h *Projects) State(ctx context.Context, p StoredPlan) (int, string, error) {
	if p.Status == "expired" {
		return 410, "plan_expired", nil
	}
	if p.Plan.EngineVersion != plan.EngineVersion || p.Plan.TemplateVersion != templates.Version || p.Plan.TemplateSHA256 != templates.Digest() {
		return 409, "plan_incompatible", nil
	}
	if p.Status == "failed" {
		return 422, "validation_failed", nil
	}
	if p.Status != "ready" {
		return 202, "", nil
	}
	ok, err := h.database.HasPlanValidation(ctx, p.ID, validation.Key(), p.ValidationIdentity)
	if err != nil {
		return 503, "database_unavailable", err
	}
	if !ok || p.ContentSHA256 == "" || p.ValidationIdentity == "" {
		return 409, "validation_incompatible", nil
	}
	return 200, "", nil
}

func (h *Projects) Get(ctx context.Context, id string) (StoredPlan, error) {
	return h.database.GetPlan(ctx, id)
}

func checkSelection(req ProjectRequest, p *plan.Plan) error {
	if req.Explicit {
		active := map[string]bool{}
		for _, e := range p.Resolved.Extensions {
			active[e.ID] = true
		}
		for id, on := range req.Flags {
			if id == "routine" || id == "transaction" {
				continue
			}
			if active[id] != on {
				return manifest.NewDiagnostic("selection_conflict", id, "", "extensions", "legacy boolean conflicts with resolved extensions")
			}
		}
	}
	return nil
}
