// Package app assembles dependencies and owns the application lifecycle.
package app

import (
	"context"
	"fmt"
	"gostarter/internal/auth"
	"gostarter/internal/config"
	"gostarter/internal/database"
	"gostarter/internal/project"
	"gostarter/internal/storage"
	"gostarter/internal/upstream"
	"gostarter/internal/web/handler"
	"gostarter/internal/worker"
	"net/http"
)

type Options struct{ MigrateOnly bool }

// Run owns PostgreSQL for the lifetime of HTTP, validation, and manual sync.
func Run(ctx context.Context, cfg *config.Config, options Options) error {
	// Connect to PostgreSQL (using DSN resolved by Viper)
	repository, err := database.ConnectPostgresWithOptions(cfg.Database.GetDSN(), database.PoolOptions{MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns, ConnMaxLifetime: cfg.Database.ConnMaxLifetime}, cfg.Admin.DefaultUser, cfg.Admin.DefaultPassword)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer repository.Close()
	if options.MigrateOnly {
		return nil
	}
	store, err := storage.New(repository, cfg.Storage)
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	runner, err := worker.New(repository, store, cfg.Validation)
	if err != nil {
		return fmt.Errorf("initialize worker: %w", err)
	}
	apiHandler := handler.New(handler.Dependencies{Projects: project.New(repository, store), Auth: auth.New(repository, cfg.Admin.SessionLifetime), Admin: upstream.NewSettings(repository), Upstream: upstream.New(repository, store), SessionLifetime: cfg.Admin.SessionLifetime})
	router := handler.SetupRouter(apiHandler)

	serverAddr := fmt.Sprintf(":%s", cfg.App.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}

	return serve(ctx, srv, runner)
}
