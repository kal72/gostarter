package project

import (
	"context"
	"errors"
	"gostarter/internal/project/render"
	"time"
)

func (h *Projects) Generate(ctx context.Context, req ProjectRequest, wait int) (StoredPlan, map[string][]byte, error) {
	p, err := h.Resolve(ctx, req)
	if err != nil && !errors.Is(err, ErrPlanExpired) {
		return p, nil, err
	}
	deadline := time.NewTimer(time.Duration(wait) * time.Second)
	defer deadline.Stop()
polling:
	for wait > 0 && (p.Status == "pending" || p.Status == "validating") {
		select {
		case <-ctx.Done():
			return p, nil, ctx.Err()
		case <-deadline.C:
			break polling
		case <-time.After(200 * time.Millisecond):
			p, err = h.database.GetPlan(ctx, p.ID)
			if err != nil && !errors.Is(err, ErrPlanExpired) {
				return p, nil, err
			}
		}
	}
	status, _, err := h.State(ctx, p)
	if err != nil {
		return p, nil, err
	}
	if status != 200 {
		return p, nil, nil
	}
	// Admission and materialization share a bounded deadline, before ZIP headers.
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	select {
	case h.materializeSlots <- struct{}{}:
		defer func() { <-h.materializeSlots }()
	case <-ctx.Done():
		return p, nil, &Failure{503, "generation_timeout", "Retry this plan later"}
	}
	_, files, err := h.store.Read(ctx, p.Plan.Snapshot.ID)
	if err != nil {
		return p, nil, &Failure{503, "snapshot_unavailable", "Pinned source could not be read"}
	}
	artifact, err := render.Materialize(ctx, &p.Plan, files)
	if err != nil {
		if ctx.Err() != nil {
			return p, nil, &Failure{503, "generation_timeout", "Retry this plan later"}
		}
		return p, nil, &Failure{503, "materialization_failed", "Pinned project could not be regenerated"}
	}
	if err = artifact.VerifyFor(&p.Plan); err != nil || artifact.Receipt.ContentSHA256 != p.ContentSHA256 || artifact.Receipt.IdentitySHA256 != p.ValidationIdentity {
		return p, nil, &Failure{409, "validation_digest_mismatch", "Regenerated output differs from the validated plan"}
	}
	// Recheck expiry and validator fencing after expensive work.
	p, err = h.database.GetPlan(ctx, p.ID)
	if err != nil && !errors.Is(err, ErrPlanExpired) {
		return p, nil, err
	}
	status, _, err = h.State(ctx, p)
	if err != nil {
		return p, nil, err
	}
	if status != 200 {
		return p, nil, nil
	}
	return p, artifact.Files, nil
}
