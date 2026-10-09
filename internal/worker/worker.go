// Package worker runs durable validation jobs without a message broker.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gostarter/internal/config"
	"gostarter/internal/database"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/validation"
	"gostarter/internal/storage"
	"gostarter/internal/upstream/profile"
)

type Validator interface {
	Snapshot(context.Context, plan.Snapshot, profile.Source) (validation.Result, error)
	Plan(context.Context, *plan.Plan, profile.Source) (validation.Result, error)
}
type Worker struct {
	DB        *database.DB
	Store     *storage.Service
	Policy    config.ValidationConfig
	Validator Validator
}

func New(repository *database.DB, store *storage.Service, policy config.ValidationConfig) (*Worker, error) {
	if repository == nil || store == nil {
		return nil, fmt.Errorf("worker requires persistent storage")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(policy.WorkspaceDir, 0700); err != nil {
		return nil, err
	}
	return &Worker{repository, store, policy, validation.Engine{WorkspaceDir: policy.WorkspaceDir, MaxCases: policy.MaxCases, CaseCache: repository}}, nil
}

// Run binds this deployment policy once, then starts a bounded set of claim loops.
func (w *Worker) Run(ctx context.Context) error {
	if err := w.DB.ConfigureValidator(ctx, validation.Key()); err != nil {
		return err
	}
	var wg sync.WaitGroup
	for i := 0; i < w.Policy.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				worked, err := w.RunOnce(ctx)
				if err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("[Validation] %s", w.Store.Summary(err.Error()))
				}
				if errors.Is(err, database.ErrValidatorChanged) {
					return
				}
				if !worked || err != nil {
					timer := time.NewTimer(w.Policy.PollInterval)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if err := w.CleanupWorkspaces(ctx); err != nil {
		return false, err
	}
	j, err := w.DB.ClaimValidation(ctx, validation.Key(), w.Policy.LeaseDuration, w.Policy.MaxAttempts, w.Policy.Concurrency)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	jobCtx, cancel := context.WithTimeout(ctx, w.Policy.JobTimeout)
	heartbeat := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(w.Policy.LeaseDuration / 3)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				heartbeat <- nil
				return
			case <-ticker.C:
				renew, c := context.WithTimeout(jobCtx, w.Policy.LeaseDuration/3)
				err := w.DB.RenewValidation(renew, j, w.Policy.LeaseDuration)
				c()
				if err != nil {
					if jobCtx.Err() != nil {
						heartbeat <- nil
						return
					}
					cancel()
					heartbeat <- err
					return
				}
			}
		}
	}()
	root := filepath.Join(w.Policy.WorkspaceDir, "gostarter-job-"+j.ID+"-"+j.LeaseToken)
	var result validation.Result
	workErr := os.Mkdir(root, 0700)
	if workErr == nil {
		result, workErr = w.execute(jobCtx, j, root)
	}
	workErr = errors.Join(workErr, os.RemoveAll(root))
	if jobCtx.Err() != nil && workErr == nil {
		workErr = jobCtx.Err()
	}
	cancel()
	leaseErr := <-heartbeat
	ack, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if leaseErr != nil {
		return true, leaseErr
	}
	if ctx.Err() != nil {
		return true, w.DB.AbandonValidation(ack, j, w.Policy.MaxAttempts)
	}
	diagnostic := ""
	if workErr != nil {
		diagnostic = w.Store.Summary(workErr.Error())
		if diagnostic == "" {
			diagnostic = "validation failed"
		}
	}
	err = w.DB.CompleteValidation(ack, j, validation.Key(), result.ContentSHA256, result.IdentitySHA256, diagnostic)
	if err != nil {
		return true, err
	}
	return true, nil
}
func (w *Worker) execute(ctx context.Context, j database.ValidationJob, root string) (validation.Result, error) {
	snapshot, files, err := w.Store.Read(ctx, j.SnapshotID)
	if err != nil {
		return validation.Result{}, err
	}
	validator := w.Validator
	if engine, ok := validator.(validation.Engine); ok {
		engine.WorkspaceDir = root
		validator = engine
	}
	if j.PlanID == "" {
		return validator.Snapshot(ctx, snapshot.Metadata.Identity(), files)
	}
	stored, err := w.DB.GetPlan(ctx, j.PlanID)
	if err != nil {
		return validation.Result{}, err
	}
	if stored.Plan.Snapshot.ID != j.SnapshotID {
		return validation.Result{}, fmt.Errorf("job snapshot mismatch")
	}
	return validator.Plan(ctx, &stored.Plan, files)
}

// SyncOnce consumes only an explicit admin request or recovers its interrupted lease.
// The lease prevents overlap; source revision fences an in-flight fetch.
func (w *Worker) SyncOnce(ctx context.Context) error {
	lease, err := w.DB.AcquireSync(ctx, "primary", w.Policy.SyncTimeout+5*time.Second)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	syncCtx, cancel := context.WithTimeout(ctx, w.Policy.SyncTimeout)
	defer cancel()
	m, captureErr := w.Store.CaptureLease(syncCtx, lease)
	if captureErr == nil {
		state, e := w.DB.GetUpstream(syncCtx, lease.State.Key)
		if e != nil {
			captureErr = e
		} else {
			valid, e := w.DB.HasSnapshotValidation(syncCtx, m.ID, validation.Key())
			if e != nil {
				captureErr = e
			} else if !valid || state.ActiveSnapshotID != m.ID {
				_, captureErr = w.Store.Enqueue(syncCtx, database.ValidationJob{SnapshotID: m.ID, SourceKey: lease.State.Key, SourceRevision: lease.State.Revision})
			}
		}
	}
	diagnostic := ""
	if captureErr != nil {
		diagnostic = w.Store.Summary(captureErr.Error())
	}
	ack, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	finishErr := w.DB.FinishSync(ack, lease, diagnostic)
	if captureErr == nil && finishErr == nil {
		if err := w.CleanupWorkspaces(ack); err != nil {
			return err
		}
		_, err := w.Store.Retain(ack)
		return err
	}
	return errors.Join(captureErr, finishErr)
}
func (w *Worker) RunSync(ctx context.Context) {
	ticker := time.NewTicker(w.Policy.PollInterval)
	defer ticker.Stop()
	for {
		if err := w.SyncOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[SnapshotSync] %s", w.Store.Summary(err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

var jobDirectory = regexp.MustCompile(`^gostarter-job-([a-f0-9]{64})-([a-f0-9]{64})$`)

// Only known jobs with a dead lease are removed; other applications/databases are not touched.
func (w *Worker) CleanupWorkspaces(ctx context.Context) error {
	entries, err := os.ReadDir(w.Policy.WorkspaceDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "gostarter-job-") {
			continue
		}
		matches := jobDirectory.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		known, live, err := w.DB.WorkspaceLease(ctx, matches[1], matches[2])
		if err != nil {
			return err
		}
		if known && !live {
			if err = os.RemoveAll(filepath.Join(w.Policy.WorkspaceDir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
