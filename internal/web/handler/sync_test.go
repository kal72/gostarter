package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"gostarter/internal/project"
	"gostarter/internal/upstream"
)

type blockedSyncRepository struct{ upstream.UpstreamRepository }

func (blockedSyncRepository) RequestSync(context.Context) error {
	return fmt.Errorf("request rejected: %w", upstream.ErrValidationRunning)
}

func TestSyncRejectsRunningValidation(t *testing.T) {
	h := New(Dependencies{Projects: project.New(nil, nil), Upstream: upstream.New(blockedSyncRepository{}, nil)})
	for _, tc := range []struct {
		name string
		html bool
	}{{"API", false}, {"HTML", true}} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/sync", nil)
			if tc.html {
				h.uiSync(w, r)
			} else {
				h.HandleSync(w, r)
			}
			if w.Code != 409 || !strings.Contains(w.Body.String(), "Validation is running") {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			if tc.html && w.Header().Get("HX-Trigger") != "admin-refresh" {
				t.Fatal("missing status refresh")
			}
			if !tc.html && !strings.Contains(w.Body.String(), "validation_running") {
				t.Fatal("missing error code")
			}
		})
	}
}

func TestSourceViewIncludesGlobalValidationStatus(t *testing.T) {
	if !sourceView(upstream.AdminOverview{ValidationRunning: true}).ValidationRunning {
		t.Fatal("global validation status omitted from UI")
	}
}
