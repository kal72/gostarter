package database

import (
	"context"
	"database/sql"
	"errors"
	"gostarter/internal/upstream"
)

type AdminSnapshot = upstream.AdminSnapshot
type AdminOverview = upstream.AdminOverview

func (d *DB) AdminOverview(ctx context.Context, key string) (AdminOverview, error) {
	out := AdminOverview{Snapshots: []AdminSnapshot{}}
	state, err := d.GetUpstream(ctx, "primary")
	if errors.Is(err, sql.ErrNoRows) {
		settings, err := d.GetSettingsContext(ctx)
		out.Repository = settings.UpstreamRepo
		out.Ref = settings.UpstreamBranch
		return out, err
	}
	if err != nil {
		return out, err
	}
	out.Repository = state.Repository
	out.Ref = state.Ref
	out.Revision = state.Revision
	out.CandidateEpoch = state.CandidateEpoch
	out.ActiveID = state.ActiveSnapshotID
	out.CandidateID = state.CandidateSnapshotID
	if err = d.conn.QueryRowContext(ctx, `SELECT sync_until,sync_requested,last_sync_error FROM upstream_state WHERE source_key='primary'`).Scan(&out.SyncUntil, &out.SyncRequested, &out.SyncError); err != nil {
		return out, err
	}
	rows, err := d.conn.QueryContext(ctx, `SELECT s.id,s.repository,s.source_ref,s.commit_sha,s.status,s.created_at,COALESCE(j.status,'not_queued'),COALESCE(j.diagnostic,''),EXISTS(SELECT 1 FROM validation_jobs v WHERE v.snapshot_id=s.id AND v.plan_id IS NULL AND v.status='succeeded' AND v.result_digest IS NOT NULL AND v.validation_key=$3 AND v.validation_key=(SELECT value FROM admin_settings WHERE key='validation_key')) FROM template_snapshots s LEFT JOIN LATERAL (SELECT status,diagnostic FROM validation_jobs WHERE snapshot_id=s.id AND plan_id IS NULL ORDER BY created_at DESC,id DESC LIMIT 1) j ON TRUE WHERE (s.repository=$1 AND s.source_ref=$2) OR s.id=$4 OR s.id=$5 ORDER BY (s.id=$4 OR s.id=$5) DESC,s.created_at DESC LIMIT 20`, state.Repository, state.Ref, key, state.ActiveSnapshotID, state.CandidateSnapshotID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item AdminSnapshot
		if err = rows.Scan(&item.ID, &item.Repository, &item.Ref, &item.CommitSHA, &item.Status, &item.CreatedAt, &item.ValidationStatus, &item.Diagnostic, &item.Compatible); err != nil {
			return out, err
		}
		out.Snapshots = append(out.Snapshots, item)
	}
	return out, rows.Err()
}
