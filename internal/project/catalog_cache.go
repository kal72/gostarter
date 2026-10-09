package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"gostarter/internal/project/validation"
)

type catalogEntry struct {
	key  string
	Body []byte
	ETag string
}

// catalogKey checks current authority cheaply, without reading/parsing the archive.
func (h *Projects) catalogKey(ctx context.Context) (string, error) {
	state, err := h.database.GetUpstream(ctx, "primary")
	if err != nil {
		return "", err
	}
	if state.ActiveSnapshotID == "" {
		return "", ErrSnapshotUnavailable
	}
	record, err := h.database.GetSnapshot(ctx, state.ActiveSnapshotID)
	if err != nil {
		return "", err
	}
	valid, err := h.database.HasSnapshotValidation(ctx, state.ActiveSnapshotID, validation.Key())
	if err != nil {
		return "", err
	}
	if !valid || record.Status != "ready" {
		return "", ErrSnapshotUnavailable
	}
	settings, err := h.database.GetSettingsContext(ctx)
	if err != nil {
		return "", err
	}
	key, err := json.Marshal([]any{state.ActiveSnapshotID, settings, validation.Key()})
	return string(key), err
}

// CachedCatalog retains one immutable serialized catalog per application instance.
func (h *Projects) CachedCatalog(ctx context.Context) ([]byte, string, error) {
	key, err := h.catalogKey(ctx)
	if err != nil {
		return nil, "", err
	}
	h.catalogMu.Lock()
	defer h.catalogMu.Unlock()
	if h.catalogCache.key == key {
		return h.catalogCache.Body, h.catalogCache.ETag, nil
	}
	info, err := h.Catalog(ctx)
	if err != nil {
		return nil, "", err
	}
	current, err := h.catalogKey(ctx)
	if err != nil {
		return nil, "", err
	}
	if current != key {
		return nil, "", ErrSnapshotUnavailable
	}
	body, err := json.Marshal(info)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	h.catalogCache = catalogEntry{key, body, etag}
	return body, etag, nil
}
