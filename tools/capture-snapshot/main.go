// capture-snapshot is a development command. Downloaded candidates are never activated here.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"gostarter/internal/config"
	"gostarter/internal/database"
	"gostarter/internal/storage"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	env := flag.String("env", "", "configuration file")
	enqueue := flag.Bool("enqueue", false, "queue candidate validation for the worker")
	key := flag.String("key", "primary", "upstream state key")
	owner := flag.String("owner", "", "GitHub owner (required)")
	repo := flag.String("repo", "", "GitHub repository (required)")
	ref := flag.String("ref", "", "GitHub ref (required)")
	flag.Parse()
	if *owner == "" || *repo == "" || *ref == "" {
		return fmt.Errorf("owner, repo, and ref are required")
	}
	cfg, err := config.Load(*env)
	if err != nil {
		return err
	}
	repository, err := database.ConnectPostgresWithOptions(cfg.Database.GetDSN(), database.PoolOptions{MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns, ConnMaxLifetime: cfg.Database.ConnMaxLifetime}, cfg.Admin.DefaultUser, cfg.Admin.DefaultPassword)
	if err != nil {
		return err
	}
	defer repository.Close()
	service, err := storage.New(repository, cfg.Storage, cfg.Upstream.GitHubToken)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m, err := service.Capture(ctx, *key, *owner, *repo, *ref)
	if err != nil {
		return err
	}
	jobID := ""
	if *enqueue {
		state, err := repository.GetUpstream(ctx, *key)
		if err != nil {
			return err
		}
		jobID, err = service.Enqueue(ctx, database.ValidationJob{SnapshotID: m.ID, SourceKey: *key, SourceRevision: state.Revision})
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"snapshot_id": m.ID, "commit_sha": m.CommitSHA, "content_sha256": m.ContentSHA256, "status": "downloaded", "active": false, "validation_job_id": jobID})
}
