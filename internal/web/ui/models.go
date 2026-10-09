package ui

import (
	"time"
)

type AdminPageModel struct {
	User     *User
	Settings Settings
	Source   Source
}

func ShortID(id string) string {
	if id == "" {
		return "none"
	}
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
func ActiveCommit(source Source) string {
	for _, s := range source.Snapshots {
		if s.ID == source.ActiveID {
			return ShortID(s.CommitSHA)
		}
	}
	return "No snapshot"
}
func SyncStatus(source Source) string {
	if source.SyncUntil != nil && source.SyncUntil.After(time.Now()) {
		return "Sync running"
	}
	if source.SyncRequested {
		return "Sync queued"
	}
	if source.ValidationRunning {
		return "Validation running · Sync unavailable"
	}
	return "Manual sync · idle"
}
func CanRollback(source Source, s Snapshot) bool {
	return s.Compatible && s.Status == "ready" && s.ID != source.ActiveID && s.Repository == source.Repository && s.Ref == source.Ref
}

type User struct{ Username string }
type Settings struct{ DefaultGroup, DefaultService, DefaultGoVersion, DefaultProtocol, UpstreamRepo, UpstreamBranch string }
type Source struct {
	Repository, Ref, ActiveID, CandidateID, SyncError string
	Revision, CandidateEpoch                          int64
	SyncUntil                                         *time.Time
	ValidationRunning                                 bool
	SyncRequested                                     bool
	Snapshots                                         []Snapshot
}
type Snapshot struct {
	ID, Repository, Ref, CommitSHA, Status, ValidationStatus, Diagnostic string
	Compatible                                                           bool
}

func SyncBusy(source Source) bool {
	return source.ValidationRunning || source.SyncRequested || source.SyncUntil != nil && source.SyncUntil.After(time.Now())
}
