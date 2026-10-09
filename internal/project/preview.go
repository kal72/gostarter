package project

import (
	"context"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/render"
	"gostarter/internal/project/validation"
	"gostarter/internal/upstream/profile"
)

// Preview assembles source without queueing build validation or executing tools.
// Generated files are explicitly marked, never presented as validated output.
func (h *Projects) assemblePreview(ctx context.Context, req ProjectRequest, snapshotID string) (profile.Source, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	var p *plan.Plan
	var source profile.Source
	if req.ID != "" {
		stored, err := h.Get(ctx, req.ID)
		if err != nil {
			return nil, "", err
		}
		if stored.Status == "expired" {
			return nil, "", &Failure{410, "plan_expired", "Resolve a new project before previewing"}
		}
		record, files, err := h.store.Read(ctx, stored.Plan.Snapshot.ID)
		if err != nil {
			return nil, "", ErrSnapshotUnavailable
		}
		valid, err := h.database.HasSnapshotValidation(ctx, record.Metadata.ID, validation.Key())
		if err != nil {
			return nil, "", err
		}
		if !valid || record.Status != "ready" {
			return nil, "", ErrSnapshotUnavailable
		}
		p, source = &stored.Plan, files
	} else {
		record, files, err := h.store.Read(ctx, snapshotID)
		if err != nil {
			return nil, "", err
		}
		valid, err := h.database.HasSnapshotValidation(ctx, snapshotID, validation.Key())
		if err != nil {
			return nil, "", err
		}
		if !valid || record.Status != "ready" {
			return nil, "", ErrSnapshotUnavailable
		}
		p, err = plan.Build(files, record.Metadata.Identity(), req.Config)
		if err != nil {
			return nil, "", err
		}
		if err = checkSelection(req, p); err != nil {
			return nil, "", err
		}
		source = files
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	files, err := render.Assemble(p, source)
	if err != nil {
		return nil, "", err
	}
	for _, f := range p.Files {
		if f.Action == profile.Generate {
			files[f.Path] = []byte("[Generated during download; not included in this source preview]\n")
		}
	}
	return files, p.Config.ServiceName, ctx.Err()
}
