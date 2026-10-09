package components

import (
	"context"
	ui "gostarter/internal/web/ui"
	"strings"
	"testing"
	"time"
)

func TestSyncButtonTracksQueueAndLease(t *testing.T) {
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	for _, tc := range []struct {
		name   string
		source ui.Source
		busy   bool
	}{{"queued", ui.Source{SyncRequested: true}, true}, {"running", ui.Source{SyncUntil: &future}, true}, {"expired", ui.Source{SyncUntil: &past}, false}, {"idle", ui.Source{}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var output strings.Builder
			if err := AdminStats(tc.source).Render(context.Background(), &output); err != nil {
				t.Fatal(err)
			}
			html := output.String()
			a := strings.Index(html, `id="btnAdminSync"`)
			b := strings.Index(html[a:], ">")
			button := html[a : a+b]
			if strings.Contains(button, " disabled ") != tc.busy {
				t.Fatalf("unexpected sync button: %s", button)
			}
		})
	}
}

func TestAdminDisplaysSyncRetryTime(t *testing.T) {
	message := "GitHub API rate limit exceeded. Retry Sync after 2027-01-15 08:02:00 UTC."
	var output strings.Builder
	if err := SnapshotList(ui.Source{SyncError: message}).Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), message) {
		t.Fatal("admin status did not display retry time")
	}
}
