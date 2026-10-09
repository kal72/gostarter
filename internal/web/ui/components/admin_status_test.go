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
