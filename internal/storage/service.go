// Package storage coordinates immutable source bytes and PostgreSQL metadata.
// It does not activate snapshots or run validation jobs.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gostarter/internal/config"
	"gostarter/internal/database"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/validation"
	"gostarter/internal/upstream/profile"
	"gostarter/internal/upstream/snapshot"
	"gostarter/internal/upstream/source"
)

type Service struct {
	repository  *database.DB
	archives    *snapshot.Store
	policy      config.StorageConfig
	githubToken string
}

func New(repository *database.DB, policy config.StorageConfig, githubToken ...string) (*Service, error) {
	if repository == nil || policy.PlanLifetime <= 0 || policy.PlanLifetime > 30*24*time.Hour || policy.RetainSnapshots < 1 || policy.RetainSnapshots > 1000 || policy.MaxPendingJobs < 1 || policy.MaxPendingJobs > 10000 || policy.DiagnosticBytes < 256 || policy.DiagnosticBytes > 8192 {
		return nil, fmt.Errorf("invalid storage policy")
	}
	archives, err := snapshot.NewStore(policy.SnapshotDir)
	if err != nil {
		return nil, err
	}
	token := ""
	if len(githubToken) > 0 {
		token = strings.TrimSpace(githubToken[0])
	}
	return &Service{repository: repository, archives: archives, policy: policy, githubToken: token}, nil
}

// Capture downloads one resolved commit and persists a downloaded candidate. It cannot activate it.
func (s *Service) Capture(ctx context.Context, key, owner, repo, ref string) (result snapshot.Metadata, returnErr error) {
	state, err := s.repository.SetUpstream(ctx, key, strings.ToLower(owner+"/"+repo), ref)
	if err != nil {
		return snapshot.Metadata{}, err
	}
	lease, err := s.repository.BeginSourceSync(ctx, state, 2*time.Minute)
	if err != nil {
		return snapshot.Metadata{}, err
	}
	defer func() {
		ack, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		diagnostic := ""
		if returnErr != nil {
			diagnostic = s.Summary(returnErr.Error())
		}
		returnErr = errors.Join(returnErr, s.repository.FinishSync(ack, lease, diagnostic))
	}()
	return s.CaptureLease(ctx, lease)
}
func (s *Service) CaptureLease(ctx context.Context, lease database.SyncLease) (snapshot.Metadata, error) {
	parts := strings.Split(lease.State.Repository, "/")
	if len(parts) != 2 {
		return snapshot.Metadata{}, fmt.Errorf("invalid source repository")
	}
	info, data, err := source.NewFetcher(parts[0], parts[1], lease.State.Ref, s.githubToken).Fetch(ctx)
	if err != nil {
		return snapshot.Metadata{}, err
	}
	m, _, err := snapshot.Prepare(lease.State.Repository, lease.State.Ref, info.SHA, data)
	if err != nil {
		return m, err
	}
	err = s.repository.StoreCandidate(ctx, lease.State.Key, lease.State.Revision, m, func() error { return s.archives.Put(ctx, m, data) }, lease.Token)
	return m, err
}

func (s *Service) PersistCandidate(ctx context.Context, key string, revision int64, repository, ref, sha string, data []byte) (snapshot.Metadata, error) {
	m, _, err := snapshot.Prepare(repository, ref, sha, data)
	if err != nil {
		return m, err
	}
	if err = s.repository.StoreCandidate(ctx, key, revision, m, func() error { return s.archives.Put(ctx, m, data) }); err != nil {
		return m, err
	}
	return m, nil
}
func (s *Service) Resolve(ctx context.Context, snapshotID string, cfg plan.Config) (database.StoredPlan, error) {
	r, err := s.repository.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return database.StoredPlan{}, err
	}
	valid, err := s.repository.HasSnapshotValidation(ctx, snapshotID, validation.Key())
	if err != nil {
		return database.StoredPlan{}, err
	}
	if r.Status != "ready" || !valid {
		return database.StoredPlan{}, fmt.Errorf("snapshot not ready")
	}
	files, err := s.archives.Read(ctx, r.Metadata)
	if err != nil {
		return database.StoredPlan{}, err
	}
	p, err := plan.Build(files, r.Metadata.Identity(), cfg)
	if err != nil {
		return database.StoredPlan{}, err
	}
	stored, err := s.repository.SavePlan(ctx, p, s.policy.PlanLifetime)
	if err != nil {
		return stored, err
	}
	if stored.Status == "pending" || stored.Status == "validating" {
		_, err = s.Enqueue(ctx, database.ValidationJob{SnapshotID: snapshotID, PlanID: stored.ID})
	}
	return stored, err
}
func (s *Service) Enqueue(ctx context.Context, j database.ValidationJob) (string, error) {
	return s.repository.EnqueueValidation(ctx, j, s.policy.MaxPendingJobs, validation.Key())
}
func (s *Service) Retain(ctx context.Context) (int, error) {
	return s.repository.RetainSnapshots(ctx, s.policy.RetainSnapshots, s.archives.Remove)
}

// Summary truncates on a UTF-8 boundary; full command logs never enter the repository.
func (s *Service) Summary(message string) string {
	if s.githubToken != "" {
		message = strings.ReplaceAll(message, s.githubToken, "[redacted]")
	}
	message = strings.ToValidUTF8(message, "\uFFFD")
	if len(message) <= s.policy.DiagnosticBytes {
		return message
	}
	message = message[:s.policy.DiagnosticBytes]
	for !utf8.ValidString(message) && len(message) > 0 {
		message = message[:len(message)-1]
	}
	return message
}

func (s *Service) Read(ctx context.Context, id string) (database.SnapshotRecord, profile.Source, error) {
	r, err := s.repository.GetSnapshot(ctx, id)
	if err != nil {
		return r, nil, err
	}
	files, err := s.archives.Read(ctx, r.Metadata)
	return r, files, err
}
func (s *Service) Rollback(ctx context.Context, key string, revision, epoch int64, id string) error {
	_, _, err := s.Read(ctx, id)
	if err != nil {
		return err
	}
	return s.repository.RollbackSnapshot(ctx, key, revision, epoch, id, validation.Key())
}
