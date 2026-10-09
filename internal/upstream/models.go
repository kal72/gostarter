package upstream

import (
	"gostarter/internal/upstream/snapshot"
	"time"
)

type AdminSettings struct {
	DefaultGroup     string `json:"default_group"`
	DefaultService   string `json:"default_service"`
	DefaultGoVersion string `json:"default_go_version"`
	DefaultProtocol  string `json:"default_protocol"`
	UpstreamRepo     string `json:"upstream_repo"`
	UpstreamBranch   string `json:"upstream_branch"`
}
type AdminSnapshot struct {
	ID               string    `json:"id"`
	Repository       string    `json:"repository"`
	Ref              string    `json:"ref"`
	CommitSHA        string    `json:"commit_sha"`
	Status           string    `json:"status"`
	ValidationStatus string    `json:"validation_status"`
	Diagnostic       string    `json:"diagnostic,omitempty"`
	Compatible       bool      `json:"compatible"`
	CreatedAt        time.Time `json:"created_at"`
}
type AdminOverview struct {
	Repository     string          `json:"repository"`
	Ref            string          `json:"ref"`
	Revision       int64           `json:"revision"`
	CandidateEpoch int64           `json:"candidate_epoch"`
	ActiveID       string          `json:"active_snapshot_id"`
	CandidateID    string          `json:"candidate_snapshot_id"`
	SyncUntil      *time.Time      `json:"sync_until"`
	SyncRequested  bool            `json:"sync_requested"`
	SyncError      string          `json:"sync_error,omitempty"`
	Snapshots      []AdminSnapshot `json:"snapshots"`
}
type SnapshotRecord struct {
	Metadata  snapshot.Metadata
	Status    string
	CreatedAt time.Time
}
type UpstreamState struct {
	Key, Repository, Ref                  string
	Revision                              int64
	CandidateEpoch                        int64
	ActiveSnapshotID, CandidateSnapshotID string
}
