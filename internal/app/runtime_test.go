package app

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

type fakeServer struct {
	failure         error
	shutdownFailure error
	stopped         chan struct{}
	once            sync.Once
	closed          bool
}

func (s *fakeServer) ListenAndServe() error {
	if s.failure != nil {
		return s.failure
	}
	<-s.stopped
	return http.ErrServerClosed
}
func (s *fakeServer) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.shutdownFailure != nil {
		return s.shutdownFailure
	}
	s.once.Do(func() { close(s.stopped) })
	return nil
}
func (s *fakeServer) Close() error {
	s.closed = true
	s.once.Do(func() { close(s.stopped) })
	return nil
}

type fakeWorker struct {
	failure        error
	validationDone chan struct{}
	syncDone       chan struct{}
}

func (w *fakeWorker) Run(ctx context.Context) error {
	defer close(w.validationDone)
	if w.failure != nil {
		return w.failure
	}
	<-ctx.Done()
	return ctx.Err()
}
func (w *fakeWorker) RunSync(ctx context.Context) {
	defer close(w.syncDone)
	<-ctx.Done()
}
func TestApplicationLifecycle(t *testing.T) {
	failure := errors.New("component failed")
	for _, name := range []string{"signal", "http failure", "validation failure", "shutdown failure"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := &fakeServer{stopped: make(chan struct{})}
			worker := &fakeWorker{validationDone: make(chan struct{}), syncDone: make(chan struct{})}
			switch name {
			case "signal":
				cancel()
			case "http failure":
				server.failure = failure
			case "validation failure":
				worker.failure = failure
			case "shutdown failure":
				server.shutdownFailure = failure
				cancel()
			}
			result := make(chan error, 1)
			go func() { result <- serve(ctx, server, worker) }()
			select {
			case err := <-result:
				if name == "signal" && err != nil {
					t.Fatal(err)
				}
				if name != "signal" && !errors.Is(err, failure) {
					t.Fatalf("lost component failure: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("application did not stop")
			}
			for _, done := range []chan struct{}{worker.validationDone, worker.syncDone} {
				select {
				case <-done:
				default:
					t.Fatal("returned before background processing stopped")
				}
			}
			if name == "shutdown failure" && !server.closed {
				t.Fatal("HTTP close fallback was not called")
			}
		})
	}
}
