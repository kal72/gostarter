package project

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/profile"
)

// Done is first inspected after joining the flight, giving deterministic coordination.
type joinedContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *joinedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}

func TestPreviewFlightSharesWorkAndIsolatesOutputs(t *testing.T) {
	h := New(nil, nil)
	var calls atomic.Int32
	release := make(chan struct{})
	work := func(ctx context.Context) (previewResult, error) {
		calls.Add(1)
		if _, ok := ctx.Deadline(); !ok {
			t.Error("shared work has no deadline")
		}
		<-release
		return previewResult{profile.Source{"README.md": []byte("original")}, "service"}, nil
	}
	outputs := make(chan previewResult, 2)
	for i := 0; i < 2; i++ {
		ctx := &joinedContext{Context: context.Background(), joined: make(chan struct{})}
		go func() {
			result, err := h.sharedPreview(ctx, "same", work)
			if err != nil {
				t.Error(err)
			}
			outputs <- result
		}()
		<-ctx.joined
	}
	close(release)
	a, b := <-outputs, <-outputs
	if calls.Load() != 1 {
		t.Fatalf("work ran %d times", calls.Load())
	}
	a.files["README.md"][0] = 'X'
	a.files["new"] = []byte("new")
	if string(b.files["README.md"]) != "original" || b.files["new"] != nil {
		t.Fatal("callers share mutable output")
	}
	if _, err := h.sharedPreview(context.Background(), "same", work); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("completed result retained")
	}
}

func TestPreviewFlightCallerCancellationDoesNotCancelSharedWork(t *testing.T) {
	h := New(nil, nil)
	base, cancel := context.WithCancel(context.Background())
	ctx := &joinedContext{Context: base, joined: make(chan struct{})}
	started, release := make(chan struct{}), make(chan struct{})
	work := func(shared context.Context) (previewResult, error) {
		close(started)
		<-release
		return previewResult{name: "service"}, shared.Err()
	}
	first := make(chan error, 1)
	go func() { _, err := h.sharedPreview(ctx, "same", work); first <- err }()
	<-started
	<-ctx.joined
	follower := &joinedContext{Context: context.Background(), joined: make(chan struct{})}
	second := make(chan error, 1)
	go func() { _, err := h.sharedPreview(follower, "same", work); second <- err }()
	<-follower.joined
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if err := <-second; err != nil {
		t.Fatal("leader cancellation affected follower:", err)
	}
}

func TestPreviewFlightKeySeparatesInputs(t *testing.T) {
	base := ProjectRequest{Config: plan.Config{Module: "example.com/app"}}
	key, _ := previewFlightKey("snapshot-a", base)
	for _, tc := range []struct {
		snapshot string
		req      ProjectRequest
	}{
		{"snapshot-b", base},
		{"snapshot-a", ProjectRequest{Config: plan.Config{Module: "example.com/other"}}},
		{"snapshot-a", ProjectRequest{ID: "stored-plan"}},
		{"snapshot-a", ProjectRequest{Config: base.Config, Explicit: true, Flags: map[string]bool{"auth": false}}},
	} {
		other, err := previewFlightKey(tc.snapshot, tc.req)
		if err != nil || key == other {
			t.Fatal("different inputs share a key")
		}
	}
}

func TestPreviewFlightErrorsAreNotCached(t *testing.T) {
	h := New(nil, nil)
	want := errors.New("temporary failure")
	_, err := h.sharedPreview(context.Background(), "same", func(context.Context) (previewResult, error) { return previewResult{}, want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := h.sharedPreview(ctx, "same", func(context.Context) (previewResult, error) { return previewResult{name: "recovered"}, nil })
	if err != nil || result.name != "recovered" {
		t.Fatal("failure retained", err)
	}
}
