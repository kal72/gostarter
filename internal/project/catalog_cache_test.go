package project

import (
	"context"
	"errors"
	"gostarter/internal/upstream"
	"testing"
)

type catalogRepository struct {
	ProjectRepository
	settings upstream.AdminSettings
	valid    bool
}

func (r *catalogRepository) GetUpstream(context.Context, string) (upstream.UpstreamState, error) {
	return upstream.UpstreamState{ActiveSnapshotID: "snapshot"}, nil
}
func (r *catalogRepository) GetSnapshot(context.Context, string) (upstream.SnapshotRecord, error) {
	return upstream.SnapshotRecord{Status: "ready"}, nil
}
func (r *catalogRepository) HasSnapshotValidation(context.Context, string, string) (bool, error) {
	return r.valid, nil
}
func (r *catalogRepository) GetSettingsContext(context.Context) (upstream.AdminSettings, error) {
	return r.settings, nil
}
func TestCatalogCacheChecksAuthorityAndSettingsWithoutArchiveRead(t *testing.T) {
	repository := &catalogRepository{valid: true}
	projects := New(repository, nil)
	ctx := context.Background()
	key, err := projects.catalogKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	projects.catalogCache = catalogEntry{key, []byte(`{"catalog":{}}`), `"catalog"`}
	body, tag, err := projects.CachedCatalog(ctx)
	if err != nil || len(body) == 0 || tag != `"catalog"` {
		t.Fatalf("cache missed: %v", err)
	}
	repository.settings.DefaultService = "changed"
	changed, err := projects.catalogKey(ctx)
	if err != nil || changed == key {
		t.Fatal("settings did not invalidate cache", err)
	}
	repository.valid = false
	_, _, err = projects.CachedCatalog(ctx)
	if !errors.Is(err, ErrSnapshotUnavailable) {
		t.Fatal("revoked validation served cached data", err)
	}
}
