package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gostarter/internal/upstream"
	"regexp"
	"strings"
	"time"
)

var ErrValidatorChanged = errors.New("validator policy changed")
var ErrLeaseLost = errors.New("validation lease lost")
var ErrStaleJob = errors.New("stale validation job")

// ClaimValidation recovers expired leases before admitting a job. All times use the database clock.
func (d *DB) ClaimValidation(ctx context.Context, key string, lease time.Duration, maxAttempts int, maxRunning ...int) (ValidationJob, error) {
	var j ValidationJob
	if key == "" || lease <= 0 || maxAttempts < 1 {
		return j, fmt.Errorf("invalid claim policy")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return j, err
	}
	defer tx.Rollback()
	if err = checkValidator(ctx, tx, key); err != nil {
		return j, err
	}
	// The final failed/crashed attempt never remains running indefinitely.
	_, err = tx.ExecContext(ctx, `UPDATE validation_jobs SET status=CASE WHEN attempts>=$1 THEN 'failed' ELSE 'queued' END,diagnostic=CASE WHEN attempts>=$1 THEN 'lease expired; retry budget exhausted' ELSE 'lease expired; retry queued' END,lease_token=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE status='running' AND lease_until<=clock_timestamp()`, maxAttempts)
	if err != nil {
		return j, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE validation_jobs j SET status='canceled',diagnostic='input expired, superseded, or validator unavailable',updated_at=clock_timestamp() WHERE j.status='queued' AND (j.validation_key<>$1 OR (j.plan_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM generation_plans p WHERE p.id=j.plan_id AND p.expires_at>clock_timestamp() AND p.status IN ('pending','validating'))) OR (j.source_key IS NOT NULL AND NOT EXISTS(SELECT 1 FROM upstream_state u WHERE u.source_key=j.source_key AND u.revision=j.source_revision AND u.candidate_epoch=j.candidate_epoch AND u.candidate_snapshot_id=j.snapshot_id)))`, key)
	if err != nil {
		return j, err
	}
	// Settle abandoned states, preserving evidence that an older snapshot was valid.
	_, err = tx.ExecContext(ctx, `UPDATE generation_plans p SET status=CASE WHEN expires_at<=clock_timestamp() THEN 'expired' ELSE 'failed' END,diagnostic='validation stopped without a result' WHERE p.status IN ('pending','validating') AND EXISTS(SELECT 1 FROM validation_jobs j WHERE j.plan_id=p.id) AND NOT EXISTS(SELECT 1 FROM validation_jobs j WHERE j.plan_id=p.id AND j.status IN ('queued','running')); UPDATE template_snapshots s SET status=CASE WHEN EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.plan_id IS NULL AND j.status='succeeded') THEN 'ready' ELSE 'failed' END WHERE s.status='validating' AND NOT EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.plan_id IS NULL AND j.status IN ('queued','running'))`)
	if err != nil {
		return j, err
	}
	if len(maxRunning) > 0 {
		var running int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM validation_jobs WHERE status='running'`).Scan(&running); err != nil {
			return j, err
		}
		if running >= maxRunning[0] {
			if err = tx.Commit(); err != nil {
				return j, err
			}
			return j, sql.ErrNoRows
		}
	}
	token, err := randomID()
	if err != nil {
		return j, err
	}
	err = tx.QueryRowContext(ctx, `UPDATE validation_jobs SET status='running',attempts=attempts+1,lease_token=$2,lease_until=clock_timestamp()+$3*interval '1 second',updated_at=clock_timestamp() WHERE id=(SELECT id FROM validation_jobs WHERE status='queued' AND validation_key=$1 ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING id,snapshot_id,COALESCE(plan_id,''),COALESCE(source_key,''),COALESCE(source_revision,0),status,diagnostic,validation_key,candidate_epoch,attempts,lease_token`, key, token, lease.Seconds()).Scan(&j.ID, &j.SnapshotID, &j.PlanID, &j.SourceKey, &j.SourceRevision, &j.Status, &j.Diagnostic, &j.ValidationKey, &j.CandidateEpoch, &j.Attempts, &j.LeaseToken)
	if errors.Is(err, sql.ErrNoRows) {
		if e := tx.Commit(); e != nil {
			return j, e
		}
		return j, sql.ErrNoRows
	}
	if err != nil {
		return j, err
	}
	if j.PlanID != "" {
		_, err = tx.ExecContext(ctx, `UPDATE generation_plans SET status='validating',diagnostic='' WHERE id=$1`, j.PlanID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE template_snapshots SET status='validating' WHERE id=$1 AND status<>'ready'`, j.SnapshotID)
	}
	if err != nil {
		return j, err
	}
	return j, tx.Commit()
}
func (d *DB) RenewValidation(ctx context.Context, j ValidationJob, lease time.Duration) error {
	r, err := d.conn.ExecContext(ctx, `UPDATE validation_jobs SET lease_until=clock_timestamp()+$3*interval '1 second',updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND status='running' AND lease_until>clock_timestamp() AND validation_key=(SELECT value FROM admin_settings WHERE key='validation_key')`, j.ID, j.LeaseToken, lease.Seconds())
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLeaseLost
	}
	return nil
}

// CompleteValidation fences both the lease and source selection. Activation and result commit together.
func (d *DB) CompleteValidation(ctx context.Context, j ValidationJob, key, content, identity, diagnostic string) error {
	if len(diagnostic) > 8192 {
		return fmt.Errorf("diagnostic exceeds limit")
	}
	success := diagnostic == ""
	if success && (!validDigest(content) || !validDigest(identity)) {
		return fmt.Errorf("validation result missing digest")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = checkValidator(ctx, tx, key); err != nil {
		return err
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT lease_token=$2 AND status='running' AND lease_until>clock_timestamp() AND validation_key=$3 AND snapshot_id=$4 AND COALESCE(plan_id,'')=$5 AND COALESCE(source_key,'')=$6 AND COALESCE(source_revision,0)=$7 AND candidate_epoch=$8 FROM validation_jobs WHERE id=$1 FOR UPDATE`, j.ID, j.LeaseToken, key, j.SnapshotID, j.PlanID, j.SourceKey, j.SourceRevision, j.CandidateEpoch).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrLeaseLost
	}
	stale := false
	if j.SourceKey != "" {
		err = tx.QueryRowContext(ctx, `SELECT revision=$2 AND candidate_epoch=$3 AND candidate_snapshot_id=$4 FROM upstream_state WHERE source_key=$1`, j.SourceKey, j.SourceRevision, j.CandidateEpoch, j.SnapshotID).Scan(&valid)
		if err != nil {
			return err
		}
		stale = !valid
	}
	if j.PlanID != "" {
		err = tx.QueryRowContext(ctx, `SELECT expires_at>clock_timestamp() AND status='validating' AND snapshot_id=$2 FROM generation_plans WHERE id=$1`, j.PlanID, j.SnapshotID).Scan(&valid)
		if err != nil {
			return err
		}
		stale = stale || !valid
	}
	status := "failed"
	if success {
		status = "succeeded"
	}
	if stale {
		status = "canceled"
		diagnostic = "input expired or superseded"
	}
	_, err = tx.ExecContext(ctx, `UPDATE validation_jobs SET status=$3,result_digest=NULLIF($4,''),diagnostic=$5,lease_token=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2`, j.ID, j.LeaseToken, status, identity, diagnostic)
	if err != nil {
		return err
	}
	if j.PlanID != "" {
		state := "failed"
		if success && !stale {
			state = "ready"
		}
		_, err = tx.ExecContext(ctx, `UPDATE generation_plans SET status=CASE WHEN expires_at<=clock_timestamp() THEN 'expired' ELSE $2 END,content_sha256=CASE WHEN $2='ready' THEN $3 ELSE NULL END,validation_identity=CASE WHEN $2='ready' THEN $4 ELSE NULL END,diagnostic=$5 WHERE id=$1`, j.PlanID, state, content, identity, diagnostic)
	} else if success && !stale {
		_, err = tx.ExecContext(ctx, `UPDATE template_snapshots SET status='ready' WHERE id=$1`, j.SnapshotID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE upstream_state SET active_snapshot_id=$2,active_validation_key=$3,updated_at=clock_timestamp() WHERE source_key=$1`, j.SourceKey, j.SnapshotID, key)
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE template_snapshots s SET status=CASE WHEN EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.plan_id IS NULL AND j.status='succeeded') THEN 'ready' ELSE 'failed' END WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.plan_id IS NULL AND j.status IN ('queued','running'))`, j.SnapshotID)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if stale {
		return ErrStaleJob
	}
	return nil
}
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func (d *DB) AbandonValidation(ctx context.Context, j ValidationJob, maxAttempts int) error {
	r, err := d.conn.ExecContext(ctx, `UPDATE validation_jobs SET status=CASE WHEN attempts>=$3 THEN 'failed' ELSE 'queued' END,diagnostic='worker stopped',lease_token=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND status='running' AND lease_until>clock_timestamp()`, j.ID, j.LeaseToken, maxAttempts)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLeaseLost
	}
	return nil
}
func (d *DB) HasSnapshotValidation(ctx context.Context, id, key string) (bool, error) {
	var valid bool
	err := d.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM validation_jobs WHERE snapshot_id=$1 AND validation_key=$2 AND plan_id IS NULL AND status='succeeded' AND result_digest IS NOT NULL AND validation_key=(SELECT value FROM admin_settings WHERE key='validation_key'))`, id, key).Scan(&valid)
	return valid, err
}

// Rollback requires existing source bytes (checked by the service) and evidence for the current validator.
// expectedEpoch makes concurrent rollback/activation compare-and-swap operations.
func (d *DB) RollbackSnapshot(ctx context.Context, source string, revision, epoch int64, id, key string) error {
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = checkValidator(ctx, tx, key); err != nil {
		return err
	}
	r, err := tx.ExecContext(ctx, `UPDATE upstream_state u SET active_snapshot_id=s.id,candidate_snapshot_id=s.id,candidate_epoch=u.candidate_epoch+1,active_validation_key=$5,updated_at=clock_timestamp() FROM template_snapshots s WHERE u.source_key=$1 AND u.revision=$2 AND u.candidate_epoch=$3 AND s.id=$4 AND s.repository=u.repository AND s.source_ref=u.source_ref AND s.status='ready' AND EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.plan_id IS NULL AND j.status='succeeded' AND j.validation_key=$5 AND j.result_digest IS NOT NULL)`, source, revision, epoch, id, key)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrStaleJob
	}
	return tx.Commit()
}

type SyncLease struct {
	State UpstreamState
	Token string
}

// AcquireSync consumes a manual request under the settings/activation lock.
// Expired manual leases may resume; legacy schedules and leases never trigger sync.
func (d *DB) AcquireSync(ctx context.Context, key string, duration time.Duration) (SyncLease, error) {
	var lease SyncLease
	tx, err := d.storageTx(ctx)
	if err != nil {
		return lease, err
	}
	defer tx.Rollback()
	token, err := randomID()
	if err != nil {
		return lease, err
	}
	err = tx.QueryRowContext(ctx, `UPDATE upstream_state SET sync_token=$2,sync_until=clock_timestamp()+$3*interval '1 second',sync_requested=false,sync_manual_lease=true WHERE source_key=$1 AND NOT EXISTS(SELECT 1 FROM validation_jobs WHERE status='running') AND (sync_until IS NULL OR sync_until<=clock_timestamp()) AND (sync_requested OR (sync_manual_lease AND sync_token IS NOT NULL AND sync_until<=clock_timestamp())) RETURNING source_key,repository,source_ref,revision,COALESCE(active_snapshot_id,''),COALESCE(candidate_snapshot_id,''),candidate_epoch`, key, token, duration.Seconds()).Scan(&lease.State.Key, &lease.State.Repository, &lease.State.Ref, &lease.State.Revision, &lease.State.ActiveSnapshotID, &lease.State.CandidateSnapshotID, &lease.State.CandidateEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		if e := tx.Commit(); e != nil {
			return lease, e
		}
		return lease, sql.ErrNoRows
	}
	if err != nil {
		return lease, err
	}
	lease.Token = token
	return lease, tx.Commit()
}
func (d *DB) FinishSync(ctx context.Context, lease SyncLease, diagnostic string) error {
	if len(diagnostic) > 8192 {
		return fmt.Errorf("sync diagnostic exceeds limit")
	}
	r, err := d.conn.ExecContext(ctx, `UPDATE upstream_state SET sync_token=NULL,sync_until=NULL,sync_manual_lease=false,last_sync_error=$3 WHERE source_key=$1 AND sync_token=$2`, lease.State.Key, lease.Token, diagnostic)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLeaseLost
	}
	return nil
}

var repositoryPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

func validateSource(repository, ref string) error {
	if !repositoryPattern.MatchString(repository) || strings.Contains(repository, "..") || strings.TrimSpace(ref) != ref || ref == "" || len(ref) > 512 {
		return fmt.Errorf("invalid upstream repository/ref")
	}
	return nil
}

// ConfigureValidator is called once when a worker deployment starts. Later workers fence older policies.
func (d *DB) ConfigureValidator(ctx context.Context, key string) error {
	if !validDigest(key) {
		return fmt.Errorf("invalid validator key")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_settings(key,value) VALUES('validation_key',$1) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, key)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func checkValidator(ctx context.Context, tx *sql.Tx, key string) error {
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM admin_settings WHERE key='validation_key'`).Scan(&current); err != nil {
		return err
	}
	if current != key {
		return ErrValidatorChanged
	}
	return nil
}
func (d *DB) BeginSourceSync(ctx context.Context, state UpstreamState, duration time.Duration) (SyncLease, error) {
	l := SyncLease{State: state}
	token, err := randomID()
	if err != nil {
		return l, err
	}
	r, err := d.conn.ExecContext(ctx, `UPDATE upstream_state SET sync_token=$3,sync_manual_lease=false,sync_until=clock_timestamp()+$4*interval '1 second' WHERE source_key=$1 AND revision=$2 AND (sync_until IS NULL OR sync_until<=clock_timestamp())`, state.Key, state.Revision, token, duration.Seconds())
	if err != nil {
		return l, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return l, err
	}
	if n != 1 {
		return l, ErrLeaseLost
	}
	l.Token = token
	return l, nil
}

func (d *DB) WorkspaceLease(ctx context.Context, id, token string) (known, live bool, err error) {
	err = d.conn.QueryRowContext(ctx, `SELECT true,COALESCE(status='running' AND lease_token=$2 AND lease_until>clock_timestamp(),false) FROM validation_jobs WHERE id=$1`, id, token).Scan(&known, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	return
}

// HasPlanValidation fences publication against the current validator deployment.
func (d *DB) HasPlanValidation(ctx context.Context, id, key, identity string) (bool, error) {
	var valid bool
	err := d.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM validation_jobs WHERE plan_id=$1 AND validation_key=$2 AND result_digest=$3 AND status='succeeded' AND validation_key=(SELECT value FROM admin_settings WHERE key='validation_key'))`, id, key, identity).Scan(&valid)
	return valid, err
}

// RequestSync is the sole scheduler for background source downloads.
func (d *DB) RequestSync(ctx context.Context) error {
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var running bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM validation_jobs WHERE status='running')`).Scan(&running); err != nil {
		return err
	}
	if running {
		return upstream.ErrValidationRunning
	}
	var repository, ref string
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT value FROM admin_settings WHERE key='upstream_repo'),(SELECT value FROM admin_settings WHERE key='upstream_branch')`).Scan(&repository, &ref); err != nil {
		return err
	}
	repository = strings.ToLower(repository)
	if err = validateSource(repository, ref); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO upstream_state(source_key,repository,source_ref,sync_requested) VALUES('primary',$1,$2,true) ON CONFLICT(source_key) DO UPDATE SET sync_requested=true,revision=upstream_state.revision+CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN 1 ELSE 0 END,candidate_snapshot_id=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.candidate_snapshot_id END,sync_token=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.sync_token END,sync_until=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.sync_until END,repository=$1,source_ref=$2 WHERE NOT upstream_state.sync_requested AND (upstream_state.sync_until IS NULL OR upstream_state.sync_until<=clock_timestamp())`, repository, ref)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return upstream.ErrSyncBusy
	}
	return tx.Commit()
}
