package project

import (
	"context"
	"errors"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"testing"
)

type planRepository struct {
	ProjectRepository
	value     StoredPlan
	validated bool
}

func (r *planRepository) GetPlan(context.Context, string) (StoredPlan, error) {
	return r.value, nil
}
func (r *planRepository) HasPlanValidation(context.Context, string, string, string) (bool, error) {
	return r.validated, nil
}
func compatiblePlan(status string) StoredPlan {
	return StoredPlan{ID: "plan", Status: status, ContentSHA256: "content", ValidationIdentity: "identity", Plan: plan.Plan{EngineVersion: plan.EngineVersion, TemplateVersion: templates.Version, TemplateSHA256: templates.Digest()}}
}
func TestGenerationRejectsUnvalidatedReadyPlanBeforeReadingSource(t *testing.T) {
	repository := &planRepository{value: compatiblePlan("ready")}
	projects := New(repository, nil)
	_, files, err := projects.Generate(context.Background(), ProjectRequest{ID: "plan"}, 0)
	if err != nil || files != nil {
		t.Fatalf("unvalidated plan produced output: %v", err)
	}
	status, code, err := projects.State(context.Background(), repository.value)
	if err != nil || status != 409 || code != "validation_incompatible" {
		t.Fatalf("state=%d code=%s err=%v", status, code, err)
	}
}
func TestGenerationPollingHonorsCancellation(t *testing.T) {
	repository := &planRepository{value: compatiblePlan("pending")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, files, err := New(repository, nil).Generate(ctx, ProjectRequest{ID: "plan"}, 10)
	if !errors.Is(err, context.Canceled) || files != nil {
		t.Fatalf("err=%v files=%v", err, files)
	}
}
func TestPlanExpiryTakesPrecedenceOverEngineCompatibility(t *testing.T) {
	status, code, err := New(nil, nil).State(context.Background(), StoredPlan{Status: "expired"})
	if err != nil || status != 410 || code != "plan_expired" {
		t.Fatalf("state=%d code=%s err=%v", status, code, err)
	}
}

func TestPreviewRejectsExpiredPlanWithoutReadingSource(t *testing.T) {
	repository := &planRepository{value: compatiblePlan("expired")}
	files, _, err := New(repository, nil).Preview(context.Background(), ProjectRequest{ID: "plan"})
	var failure *Failure
	if files != nil || !errors.As(err, &failure) || failure.Code != "plan_expired" {
		t.Fatalf("files=%v err=%v", files, err)
	}
}
func TestPreviewCancellationDoesNotReadOrQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, _, err := New(nil, nil).Preview(ctx, ProjectRequest{})
	if files != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("files=%v err=%v", files, err)
	}
}
