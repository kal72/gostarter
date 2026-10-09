package upstream

import (
	"context"
	"gostarter/internal/project/validation"
)

type UpstreamRepository interface {
	AdminOverview(context.Context, string) (AdminOverview, error)
	GetSettingsContext(context.Context) (AdminSettings, error)
	RequestSync(context.Context) error
}
type SnapshotActivator interface {
	Rollback(context.Context, string, int64, int64, string) error
}
type Upstream struct {
	database UpstreamRepository
	store    SnapshotActivator
}

func New(database UpstreamRepository, store SnapshotActivator) *Upstream {
	return &Upstream{database, store}
}
func (s *Upstream) Overview(ctx context.Context) (AdminOverview, error) {
	if s.database == nil {
		return AdminOverview{}, ErrUnavailable
	}
	return s.database.AdminOverview(ctx, validation.Key())
}
func (s *Upstream) Settings(ctx context.Context) (AdminSettings, error) {
	if s.database == nil {
		return AdminSettings{}, ErrUnavailable
	}
	return s.database.GetSettingsContext(ctx)
}
func (s *Upstream) Sync(ctx context.Context) error {
	if s.database == nil {
		return ErrUnavailable
	}
	return s.database.RequestSync(ctx)
}
func (s *Upstream) Rollback(ctx context.Context, revision, epoch int64, id string) error {
	if s.store == nil {
		return ErrUnavailable
	}
	return s.store.Rollback(ctx, "primary", revision, epoch, id)
}
