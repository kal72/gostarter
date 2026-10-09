package config_test

import (
	"os"
	"testing"

	"gostarter/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected nil error loading default config, got %v", err)
	}

	if cfg.App.Port == "" {
		t.Errorf("expected default port to be set")
	}

	if cfg.Database.Host != "127.0.0.1" {
		t.Errorf("expected default host 127.0.0.1, got %s", cfg.Database.Host)
	}

	if cfg.Database.Name != "gostarter" {
		t.Errorf("expected default db name gostarter, got %s", cfg.Database.Name)
	}

	dsn := cfg.Database.GetDSN()
	expectedPrefix := "postgres://postgres:postgres@127.0.0.1:5432/gostarter"
	if len(dsn) < len(expectedPrefix) || dsn[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected DSN starting with %s, got %s", expectedPrefix, dsn)
	}
}

func TestEnvOverrides(t *testing.T) {
	_ = os.Setenv("APP_PORT", "9090")
	_ = os.Setenv("DB_HOST", "database.internal")
	_ = os.Setenv("DB_NAME", "custom_db")
	defer func() {
		_ = os.Unsetenv("APP_PORT")
		_ = os.Unsetenv("DB_HOST")
		_ = os.Unsetenv("DB_NAME")
	}()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.App.Port != "9090" {
		t.Errorf("expected port 9090 from env, got %s", cfg.App.Port)
	}

	if cfg.Database.Host != "database.internal" {
		t.Errorf("expected host database.internal from env, got %s", cfg.Database.Host)
	}

	if cfg.Database.Name != "custom_db" {
		t.Errorf("expected db name custom_db from env, got %s", cfg.Database.Name)
	}
}

func TestEnvFileLoading(t *testing.T) {
	tmpFile, err := os.CreateTemp("", ".env.*")
	if err != nil {
		t.Fatalf("failed creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
APP_PORT=7777
DB_HOST=10.0.0.5
DB_PORT=5433
DB_USER=root
DB_PASSWORD=secret
DB_NAME=production_db
DB_SSLMODE=require
UPSTREAM_BRANCH=develop
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed writing temp env: %v", err)
	}
	_ = tmpFile.Close()

	cfg, err := config.Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed loading custom env file: %v", err)
	}

	if cfg.App.Port != "7777" {
		t.Errorf("expected port 7777 from env file, got %s", cfg.App.Port)
	}

	if cfg.Database.Host != "10.0.0.5" {
		t.Errorf("expected host 10.0.0.5, got %s", cfg.Database.Host)
	}

	if cfg.Database.Port != "5433" {
		t.Errorf("expected port 5433, got %s", cfg.Database.Port)
	}

	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected sslmode require, got %s", cfg.Database.SSLMode)
	}

	if cfg.Upstream.Branch != "develop" {
		t.Errorf("expected upstream branch develop, got %s", cfg.Upstream.Branch)
	}

	expectedDSN := "postgres://root:secret@10.0.0.5:5433/production_db?sslmode=require"
	if cfg.Database.GetDSN() != expectedDSN {
		t.Errorf("expected DSN %s, got %s", expectedDSN, cfg.Database.GetDSN())
	}
}

func TestStorageConfigLimits(t *testing.T) {
	for key, value := range map[string]string{"PLAN_LIFETIME": "0s", "SNAPSHOT_RETAIN_COUNT": "0", "VALIDATION_MAX_PENDING_JOBS": "10001", "VALIDATION_DIAGNOSTIC_BYTES": "8193", "DB_MAX_IDLE_CONNS": "26", "ADMIN_SESSION_LIFETIME": "-1h", "VALIDATION_CONCURRENCY": "0", "VALIDATION_LEASE_DURATION": "1s", "VALIDATION_MAX_CASES": "7", "VALIDATION_JOB_TIMEOUT": "0s"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := config.Load("/nonexistent-gostarter-config"); err == nil {
				t.Fatalf("accepted %s=%s", key, value)
			}
		})
	}
}
