package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gostarter/internal/project"
	"gostarter/internal/upstream"
	"time"

	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/snapshot"
)

const storageLock int64 = 73410824

func (d *DB) storageTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, storageLock); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

type SnapshotRecord = upstream.SnapshotRecord
type StoredPlan = project.StoredPlan
type UpstreamState = upstream.UpstreamState
type ValidationJob struct {
	ID, SnapshotID, PlanID, SourceKey string
	SourceRevision                    int64
	Status, Diagnostic                string
	ValidationKey                     string
	CandidateEpoch                    int64
	Attempts                          int
	LeaseToken                        string
}

func (d *DB) InsertSnapshot(ctx context.Context, m snapshot.Metadata) error {
	if err := m.Validate(); err != nil {
		return err
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertSnapshot(ctx, tx, m); err != nil {
		return err
	}
	return tx.Commit()
}
func insertSnapshot(ctx context.Context, tx *sql.Tx, m snapshot.Metadata) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO template_snapshots(id,repository,source_ref,commit_sha,archive_sha256,content_sha256,archive_key,manifest,compatibility) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(id) DO NOTHING`, m.ID, m.Repository, m.Ref, m.CommitSHA, m.ArchiveSHA256, m.ContentSHA256, m.ArchiveKey, string(m.Manifest), string(m.Compatibility))
	if err != nil {
		return err
	}
	// Repeat downloads preserve the first capture report. Compatibility with a newer
	// template belongs to a new validation job, not a mutation of source identity.
	var equal bool
	err = tx.QueryRowContext(ctx, `SELECT repository=$2 AND commit_sha=$3 AND archive_sha256=$4 AND content_sha256=$5 AND archive_key=$6 AND manifest=$7::jsonb FROM template_snapshots WHERE id=$1`, m.ID, m.Repository, m.CommitSHA, m.ArchiveSHA256, m.ContentSHA256, m.ArchiveKey, string(m.Manifest)).Scan(&equal)
	if err != nil {
		return err
	}
	if !equal {
		return fmt.Errorf("snapshot identity conflict")
	}
	return nil
}
func (d *DB) GetSnapshot(ctx context.Context, id string) (SnapshotRecord, error) {
	var r SnapshotRecord
	m := &r.Metadata
	err := d.conn.QueryRowContext(ctx, `SELECT id,repository,source_ref,commit_sha,archive_sha256,content_sha256,archive_key,manifest,compatibility,status,created_at FROM template_snapshots WHERE id=$1`, id).Scan(&m.ID, &m.Repository, &m.Ref, &m.CommitSHA, &m.ArchiveSHA256, &m.ContentSHA256, &m.ArchiveKey, &m.Manifest, &m.Compatibility, &r.Status, &r.CreatedAt)
	if err == nil {
		err = m.Validate()
	}
	return r, err
}

// SetUpstream changes the desired source and fences old jobs with a monotonic revision.
// It preserves the last active snapshot and never activates a downloaded candidate.
func (d *DB) SetUpstream(ctx context.Context, key, repository, ref string) (UpstreamState, error) {
	var s UpstreamState
	if key == "" || len(key) > 200 || repository == "" || ref == "" {
		return s, fmt.Errorf("invalid upstream state")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return s, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `INSERT INTO upstream_state(source_key,repository,source_ref) VALUES($1,$2,$3) ON CONFLICT(source_key) DO UPDATE SET repository=EXCLUDED.repository,source_ref=EXCLUDED.source_ref,revision=upstream_state.revision+CASE WHEN upstream_state.repository<>EXCLUDED.repository OR upstream_state.source_ref<>EXCLUDED.source_ref THEN 1 ELSE 0 END,candidate_snapshot_id=CASE WHEN upstream_state.repository<>EXCLUDED.repository OR upstream_state.source_ref<>EXCLUDED.source_ref THEN NULL ELSE upstream_state.candidate_snapshot_id END,updated_at=now() RETURNING source_key,repository,source_ref,revision,COALESCE(active_snapshot_id,''),COALESCE(candidate_snapshot_id,''),candidate_epoch`, key, repository, ref).Scan(&s.Key, &s.Repository, &s.Ref, &s.Revision, &s.ActiveSnapshotID, &s.CandidateSnapshotID, &s.CandidateEpoch)
	if err != nil {
		return s, err
	}
	return s, tx.Commit()
}
func (d *DB) SetCandidate(ctx context.Context, key string, revision int64, id string) error {
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE upstream_state u SET candidate_epoch=u.candidate_epoch+CASE WHEN u.candidate_snapshot_id IS DISTINCT FROM $3 THEN 1 ELSE 0 END,candidate_snapshot_id=$3,updated_at=now() FROM template_snapshots s WHERE u.source_key=$1 AND u.revision=$2 AND s.id=$3 AND s.repository=u.repository AND s.source_ref=u.source_ref`, key, revision, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("stale or mismatched candidate")
	}
	return tx.Commit()
}
func (d *DB) GetUpstream(ctx context.Context, key string) (UpstreamState, error) {
	var s UpstreamState
	err := d.conn.QueryRowContext(ctx, `SELECT source_key,repository,source_ref,revision,COALESCE(active_snapshot_id,''),COALESCE(candidate_snapshot_id,''),candidate_epoch FROM upstream_state WHERE source_key=$1`, key).Scan(&s.Key, &s.Repository, &s.Ref, &s.Revision, &s.ActiveSnapshotID, &s.CandidateSnapshotID, &s.CandidateEpoch)
	return s, err
}

// SavePlan persists a canonical input, never generated source. Existing plans are not renewed.
func (d *DB) SavePlan(ctx context.Context, p *plan.Plan, lifetime time.Duration) (StoredPlan, error) {
	var result StoredPlan
	if p == nil || lifetime <= 0 || lifetime > 30*24*time.Hour {
		return result, fmt.Errorf("invalid plan lifetime")
	}
	if err := p.Verify(); err != nil {
		return result, err
	}
	if p.Snapshot.Kind != "immutable" {
		return result, fmt.Errorf("fixture plans cannot be persisted")
	}
	data, err := json.Marshal(p)
	if err != nil {
		return result, err
	}
	if len(data) > 2<<20 {
		return result, fmt.Errorf("plan exceeds limit")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var sha, content, status string
	err = tx.QueryRowContext(ctx, `SELECT commit_sha,content_sha256,status FROM template_snapshots WHERE id=$1`, p.Snapshot.ID).Scan(&sha, &content, &status)
	if err != nil {
		return result, err
	}
	if sha != p.Snapshot.CommitSHA || content != p.Snapshot.ContentSHA256 || status != "ready" {
		return result, fmt.Errorf("snapshot unavailable or not ready")
	}
	// Reuse a live input, but resolving again after expiry gets a new opaque ID.
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM generation_plans WHERE fingerprint=$1 AND expires_at>now() AND status<>'expired' ORDER BY created_at DESC LIMIT 1`, p.Fingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id, err = randomID()
		if err != nil {
			return result, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO generation_plans(id,snapshot_id,fingerprint,plan,template_version,engine_version,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+$7*interval '1 second')`, id, p.Snapshot.ID, p.Fingerprint, string(data), p.TemplateVersion, p.EngineVersion, lifetime.Seconds())
	}
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return d.GetPlan(ctx, id)
}

var ErrPlanExpired = project.ErrPlanExpired

func (d *DB) GetPlan(ctx context.Context, id string) (StoredPlan, error) {
	var r StoredPlan
	var data []byte
	var expired bool
	var fingerprint string
	err := d.conn.QueryRowContext(ctx, `SELECT id,fingerprint,plan,CASE WHEN expires_at<=now() THEN 'expired' ELSE status END,COALESCE(content_sha256,''),COALESCE(validation_identity,''),diagnostic,created_at,expires_at,expires_at<=now() FROM generation_plans WHERE id=$1`, id).Scan(&r.ID, &fingerprint, &data, &r.Status, &r.ContentSHA256, &r.ValidationIdentity, &r.Diagnostic, &r.CreatedAt, &r.ExpiresAt, &expired)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(data, &r.Plan); err != nil {
		return r, err
	}
	if err = r.Plan.Verify(); err != nil {
		return r, err
	}
	if r.Plan.Fingerprint != fingerprint {
		return r, fmt.Errorf("stored plan identity mismatch")
	}
	if expired {
		return r, ErrPlanExpired
	}
	return r, nil
}

// EnqueueValidation bounds pending work across processes using the storage transaction lock.
func (d *DB) EnqueueValidation(ctx context.Context, j ValidationJob, maxPending int, keys ...string) (string, error) {
	key := ""
	if len(keys) > 0 {
		key = keys[0]
	}
	if len(key) > 128 {
		return "", fmt.Errorf("invalid validation key")
	}

	if maxPending < 1 || maxPending > 10000 || j.SnapshotID == "" || (j.PlanID == "" && (j.SourceKey == "" || j.SourceRevision < 1)) {
		return "", fmt.Errorf("invalid validation job")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM validation_jobs WHERE snapshot_id=$1 AND COALESCE(plan_id,'')=$2 AND COALESCE(source_key,'')=$3 AND COALESCE(source_revision,0)=$4 AND validation_key=$5 AND (source_key IS NULL OR candidate_epoch=(SELECT candidate_epoch FROM upstream_state WHERE source_key=$3)) AND status IN ('queued','running') AND (plan_id IS NULL OR EXISTS(SELECT 1 FROM generation_plans p WHERE p.id=plan_id AND p.expires_at>clock_timestamp() AND p.status IN ('pending','validating'))) AND (source_key IS NULL OR EXISTS(SELECT 1 FROM upstream_state u WHERE u.source_key=$3 AND u.revision=$4 AND u.candidate_snapshot_id=$1))`, j.SnapshotID, j.PlanID, j.SourceKey, j.SourceRevision, key).Scan(&existing)
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if j.PlanID != "" {
		var valid bool
		err = tx.QueryRowContext(ctx, `SELECT snapshot_id=$2 AND expires_at>clock_timestamp() AND status IN ('pending','validating') FROM generation_plans WHERE id=$1`, j.PlanID, j.SnapshotID).Scan(&valid)
		if err != nil {
			return "", err
		}
		if !valid {
			return "", fmt.Errorf("plan cannot be validated")
		}
	}
	if j.SourceKey != "" {
		var valid bool
		err = tx.QueryRowContext(ctx, `SELECT revision=$2 AND candidate_snapshot_id=$3 FROM upstream_state WHERE source_key=$1`, j.SourceKey, j.SourceRevision, j.SnapshotID).Scan(&valid)
		if err != nil {
			return "", err
		}
		if !valid {
			return "", fmt.Errorf("stale validation job")
		}
	}
	var epoch int64
	if j.SourceKey != "" {
		if err = tx.QueryRowContext(ctx, `SELECT candidate_epoch FROM upstream_state WHERE source_key=$1`, j.SourceKey).Scan(&epoch); err != nil {
			return "", err
		}
	}
	var count int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM validation_jobs WHERE status IN ('queued','running')`).Scan(&count)
	if err != nil {
		return "", err
	}
	if count >= maxPending {
		return "", fmt.Errorf("validation queue full")
	}
	token, err := randomID()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO validation_jobs(id,snapshot_id,plan_id,source_key,source_revision,validation_key,candidate_epoch) VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,0),$6,$7)`, token, j.SnapshotID, j.PlanID, j.SourceKey, j.SourceRevision, key, epoch)
	if err != nil {
		return "", err
	}
	return token, tx.Commit()
}

// RetainSnapshots serializes against plan/candidate/job insertion. Active/candidate pointers,
// unexpired plans and queued/running jobs are protected even beyond the retention count.
// remove must only delete the supplied upstream archive and must not reenter this repository.
func (d *DB) RetainSnapshots(ctx context.Context, keep int, remove func(string) error) (int, error) {
	if keep < 1 || remove == nil {
		return 0, fmt.Errorf("invalid retention policy")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// Expired plan metadata can be pruned even when its source remains active.
	if _, err = tx.ExecContext(ctx, `DELETE FROM validation_jobs WHERE plan_id IN (SELECT id FROM generation_plans WHERE expires_at<=clock_timestamp() OR status='expired') AND status NOT IN ('queued','running'); DELETE FROM generation_plans p WHERE (expires_at<=clock_timestamp() OR status='expired') AND NOT EXISTS(SELECT 1 FROM validation_jobs j WHERE j.plan_id=p.id)`); err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, `WITH ranked AS (SELECT id,row_number() OVER(PARTITION BY repository ORDER BY created_at DESC,id) AS rank FROM template_snapshots) SELECT s.id,s.archive_key FROM template_snapshots s JOIN ranked r ON r.id=s.id WHERE r.rank>$1 AND NOT EXISTS(SELECT 1 FROM upstream_state u WHERE u.active_snapshot_id=s.id OR u.candidate_snapshot_id=s.id) AND NOT EXISTS(SELECT 1 FROM generation_plans p WHERE p.snapshot_id=s.id AND p.expires_at>now() AND p.status<>'expired') AND NOT EXISTS(SELECT 1 FROM validation_jobs j WHERE j.snapshot_id=s.id AND j.status IN ('queued','running'))`, keep)
	if err != nil {
		return 0, err
	}
	type candidate struct{ id, key string }
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.key); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, c := range candidates {
		// Terminal jobs and expired plan metadata can be pruned; they do not protect source forever.
		if err = remove(c.key); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM validation_jobs WHERE snapshot_id=$1`, c.id); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM generation_plans WHERE snapshot_id=$1`, c.id); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM template_snapshots WHERE id=$1`, c.id); err != nil {
			return 0, err
		}
	}
	return len(candidates), tx.Commit()
}

// StoreCandidate holds the same lock as retention across immutable archive finalization
// and metadata insertion. It prevents retention deleting a repeated download mid-capture.
func (d *DB) StoreCandidate(ctx context.Context, key string, revision int64, m snapshot.Metadata, write func() error, syncTokens ...string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if write == nil {
		return fmt.Errorf("missing archive writer")
	}
	tx, err := d.storageTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT revision=$2 AND repository=$3 AND source_ref=$4 FROM upstream_state WHERE source_key=$1`, key, revision, m.Repository, m.Ref).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("stale candidate")
	}
	if len(syncTokens) > 0 {
		var leased bool
		if err = tx.QueryRowContext(ctx, `SELECT sync_token=$2 AND sync_until>clock_timestamp() FROM upstream_state WHERE source_key=$1`, key, syncTokens[0]).Scan(&leased); err != nil {
			return err
		}
		if !leased {
			return ErrLeaseLost
		}
	}
	if err = write(); err != nil {
		return err
	}
	if err = insertSnapshot(ctx, tx, m); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE upstream_state SET candidate_epoch=candidate_epoch+CASE WHEN candidate_snapshot_id IS DISTINCT FROM $2 THEN 1 ELSE 0 END,candidate_snapshot_id=$2,updated_at=now() WHERE source_key=$1`, key, m.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) GetValidationJob(ctx context.Context, id string) (ValidationJob, error) {
	var j ValidationJob
	err := d.conn.QueryRowContext(ctx, `SELECT id,snapshot_id,COALESCE(plan_id,''),COALESCE(source_key,''),COALESCE(source_revision,0),status,diagnostic,validation_key,candidate_epoch,attempts,COALESCE(lease_token,'') FROM validation_jobs WHERE id=$1`, id).Scan(&j.ID, &j.SnapshotID, &j.PlanID, &j.SourceKey, &j.SourceRevision, &j.Status, &j.Diagnostic, &j.ValidationKey, &j.CandidateEpoch, &j.Attempts, &j.LeaseToken)
	return j, err
}
