package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/project/validation"
	"gostarter/internal/upstream/profile"
)

type previewResult struct {
	files profile.Source
	name  string
}

// Preview shares only in-flight work. Completed results are not retained.
// Each caller keeps its own cancellation and receives independent output bytes.
func (h *Projects) Preview(ctx context.Context, req ProjectRequest) (profile.Source, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	snapshotID, err := h.previewSnapshot(ctx, req)
	if err != nil {
		return nil, "", err
	}
	key, err := previewFlightKey(snapshotID, req)
	if err != nil {
		return nil, "", err
	}
	value, err := h.sharedPreview(ctx, key, func(workCtx context.Context) (previewResult, error) {
		files, name, err := h.assemblePreview(workCtx, req, snapshotID)
		return previewResult{files, name}, err
	})
	if err != nil {
		return nil, "", err
	}
	// Authority/expiry may have changed while the caller waited.
	current, err := h.previewSnapshot(ctx, req)
	if err != nil {
		return nil, "", err
	}
	if current != snapshotID {
		return nil, "", ErrSnapshotUnavailable
	}
	record, err := h.database.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, "", err
	}
	if record.Status != "ready" {
		return nil, "", ErrSnapshotUnavailable
	}
	valid, err := h.database.HasSnapshotValidation(ctx, snapshotID, validation.Key())
	if err != nil {
		return nil, "", err
	}
	if !valid {
		return nil, "", ErrSnapshotUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return value.files, value.name, nil
}

func (h *Projects) previewSnapshot(ctx context.Context, req ProjectRequest) (string, error) {
	if req.ID != "" {
		stored, err := h.Get(ctx, req.ID)
		if err != nil {
			return "", err
		}
		if stored.Status == "expired" {
			return "", &Failure{410, "plan_expired", "Resolve a new project before previewing"}
		}
		return stored.Plan.Snapshot.ID, nil
	}
	state, err := h.database.GetUpstream(ctx, "primary")
	if err != nil {
		return "", err
	}
	if state.ActiveSnapshotID == "" {
		return "", ErrSnapshotUnavailable
	}
	return state.ActiveSnapshotID, nil
}

func previewFlightKey(snapshotID string, req ProjectRequest) (string, error) {
	body, err := json.Marshal([]any{snapshotID, req, plan.EngineVersion, templates.Digest(), validation.Key()})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

func (h *Projects) sharedPreview(ctx context.Context, key string, work func(context.Context) (previewResult, error)) (previewResult, error) {
	result := h.previewFlights.DoChan(key, func() (any, error) {
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		return work(workCtx)
	})
	select {
	case <-ctx.Done():
		return previewResult{}, ctx.Err()
	case shared := <-result:
		if err := ctx.Err(); err != nil {
			return previewResult{}, err
		}
		if shared.Err != nil {
			return previewResult{}, shared.Err
		}
		value := shared.Val.(previewResult)
		files := make(profile.Source, len(value.files))
		for path, body := range value.files {
			files[path] = append([]byte(nil), body...)
		}
		return previewResult{files, value.name}, nil
	}
}
