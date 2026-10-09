package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}
type backgroundWorker interface {
	Run(context.Context) error
	RunSync(context.Context)
}

func serve(parent context.Context, srv httpServer, runner backgroundWorker) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(3)
	go func() { defer wg.Done(); results <- srv.ListenAndServe() }()
	go func() { defer wg.Done(); results <- runner.Run(ctx) }()
	go func() { defer wg.Done(); runner.RunSync(ctx) }()
	var result error
	select {
	case <-ctx.Done():
	case err := <-results:
		result = componentError(err)
	}
	cancel()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		result = errors.Join(result, err)
		_ = srv.Close()
	}
	wg.Wait()
	// Preserve failures racing with cancellation or the other component's exit.
	close(results)
	for err := range results {
		result = errors.Join(result, componentError(err))
	}
	log.Println("GoStarter stopped.")
	return result
}

func componentError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
