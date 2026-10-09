package handler

import (
	"gostarter/internal/auth"
	"gostarter/internal/upstream"
	viewmodel "gostarter/internal/web/ui"
)

func userView(user *auth.AdminUser) *viewmodel.User {
	if user == nil {
		return nil
	}
	return &viewmodel.User{Username: user.Username}
}
func settingsView(s upstream.AdminSettings) viewmodel.Settings {
	return viewmodel.Settings{DefaultGroup: s.DefaultGroup, DefaultService: s.DefaultService, DefaultGoVersion: s.DefaultGoVersion, DefaultProtocol: s.DefaultProtocol, UpstreamRepo: s.UpstreamRepo, UpstreamBranch: s.UpstreamBranch}
}
func sourceView(s upstream.AdminOverview) viewmodel.Source {
	out := viewmodel.Source{Repository: s.Repository, Ref: s.Ref, ActiveID: s.ActiveID, CandidateID: s.CandidateID, SyncError: s.SyncError, Revision: s.Revision, CandidateEpoch: s.CandidateEpoch, SyncUntil: s.SyncUntil, SyncRequested: s.SyncRequested, ValidationRunning: s.ValidationRunning}
	for _, v := range s.Snapshots {
		out.Snapshots = append(out.Snapshots, viewmodel.Snapshot{ID: v.ID, Repository: v.Repository, Ref: v.Ref, CommitSHA: v.CommitSHA, Status: v.Status, ValidationStatus: v.ValidationStatus, Diagnostic: v.Diagnostic, Compatible: v.Compatible})
	}
	return out
}
