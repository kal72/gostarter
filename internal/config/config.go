package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config represents the root configuration for GoStarter
type Config struct {
	App        AppConfig        `json:"app"`
	Database   DatabaseConfig   `json:"database"`
	Upstream   UpstreamConfig   `json:"upstream"`
	Admin      AdminConfig      `json:"admin"`
	Storage    StorageConfig    `json:"storage"`
	Validation ValidationConfig `json:"validation"`
}

type ValidationConfig struct {
	Concurrency   int
	PollInterval  time.Duration
	JobTimeout    time.Duration
	LeaseDuration time.Duration
	MaxAttempts   int
	MaxCases      int
	WorkspaceDir  string
	SyncTimeout   time.Duration
}

func (v ValidationConfig) Validate() error {
	if v.Concurrency < 1 || v.Concurrency > 8 || v.PollInterval < 100*time.Millisecond || v.PollInterval > time.Minute || v.JobTimeout < time.Second || v.JobTimeout > time.Hour || v.LeaseDuration < 3*time.Second || v.LeaseDuration > 5*time.Minute || v.MaxAttempts < 1 || v.MaxAttempts > 5 || v.MaxCases < 8 || v.MaxCases > 128 || v.WorkspaceDir == "" || v.SyncTimeout < time.Second || v.SyncTimeout > 10*time.Minute {
		return fmt.Errorf("invalid validation worker limits")
	}
	return nil
}

// StorageConfig bounds metadata lifetime, retained source and validation work.
type StorageConfig struct {
	SnapshotDir     string        `json:"snapshot_dir"`
	PlanLifetime    time.Duration `json:"plan_lifetime"`
	RetainSnapshots int           `json:"retain_snapshots"`
	MaxPendingJobs  int           `json:"max_pending_jobs"`
	DiagnosticBytes int           `json:"diagnostic_bytes"`
}

// AdminConfig holds admin authentication and session settings
type AdminConfig struct {
	DefaultUser     string        `json:"default_user"`
	DefaultPassword string        `json:"default_password"`
	SessionLifetime time.Duration `json:"session_lifetime"`
}

// AppConfig holds HTTP server and application runtime options
type AppConfig struct {
	Name         string        `json:"name"`
	Environment  string        `json:"env"`
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// DatabaseConfig holds PostgreSQL connection and pool parameters
type DatabaseConfig struct {
	Host            string        `json:"host"`
	Port            string        `json:"port"`
	User            string        `json:"user"`
	Password        string        `json:"password"`
	Name            string        `json:"name"`
	SSLMode         string        `json:"sslmode"`
	DSN             string        `json:"dsn"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// UpstreamConfig holds reference architecture source repository settings
type UpstreamConfig struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

// GetDSN returns the constructed connection string or explicit DSN
func (d *DatabaseConfig) GetDSN() string {
	if d.DSN != "" {
		return d.DSN
	}

	escapedUser := url.QueryEscape(d.User)
	escapedPass := url.QueryEscape(d.Password)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		escapedUser,
		escapedPass,
		d.Host,
		d.Port,
		d.Name,
		d.SSLMode,
	)
}

// Load reads configuration from .env file and environment variables using Viper
func Load(envFile string) (*Config, error) {
	v := viper.New()

	// Default configurations
	v.SetDefault("APP_NAME", "gostarter")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_READ_TIMEOUT", 30*time.Second)
	v.SetDefault("APP_WRITE_TIMEOUT", 60*time.Second)
	v.SetDefault("APP_IDLE_TIMEOUT", 120*time.Second)

	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "postgres")
	v.SetDefault("DB_NAME", "gostarter")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("DB_DSN", "")
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 10)
	v.SetDefault("DB_CONN_MAX_LIFETIME", 15*time.Minute)

	v.SetDefault("UPSTREAM_OWNER", "kal72")
	v.SetDefault("UPSTREAM_REPO", "go-feature-based-boilerplate")
	v.SetDefault("UPSTREAM_BRANCH", "main")

	v.SetDefault("ADMIN_DEFAULT_USER", "admin")
	v.SetDefault("ADMIN_DEFAULT_PASSWORD", "admin123")
	v.SetDefault("ADMIN_SESSION_LIFETIME", 24*time.Hour)

	v.SetDefault("SNAPSHOT_DIR", ".cache/snapshots")
	v.SetDefault("PLAN_LIFETIME", 24*time.Hour)
	v.SetDefault("SNAPSHOT_RETAIN_COUNT", 5)
	v.SetDefault("VALIDATION_MAX_PENDING_JOBS", 100)
	v.SetDefault("VALIDATION_DIAGNOSTIC_BYTES", 8192)

	v.SetDefault("VALIDATION_CONCURRENCY", 2)
	v.SetDefault("VALIDATION_POLL_INTERVAL", time.Second)
	v.SetDefault("VALIDATION_JOB_TIMEOUT", 10*time.Minute)
	v.SetDefault("VALIDATION_LEASE_DURATION", 30*time.Second)
	v.SetDefault("VALIDATION_MAX_ATTEMPTS", 2)
	v.SetDefault("VALIDATION_MAX_CASES", 64)
	v.SetDefault("VALIDATION_WORKSPACE_DIR", os.TempDir())
	v.SetDefault("UPSTREAM_SYNC_TIMEOUT", 2*time.Minute)
	// Automatic environment variable lookup
	v.AutomaticEnv()

	// Determine .env file path
	targetEnv := ".env"
	if envFile != "" {
		targetEnv = envFile
	} else if customPath := os.Getenv("ENV_FILE"); customPath != "" {
		targetEnv = customPath
	}

	if _, err := os.Stat(targetEnv); err == nil {
		v.SetConfigFile(targetEnv)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed reading config from %s: %w", targetEnv, err)
		}
	}

	// Resolve App Port (support PORT or APP_PORT)
	port := v.GetString("APP_PORT")
	if p := v.GetString("PORT"); p != "" {
		port = p
	}

	// Resolve Database DSN (support DATABASE_URL or DB_DSN)
	dsn := v.GetString("DB_DSN")
	if d := v.GetString("DATABASE_URL"); d != "" {
		dsn = d
	}

	// Resolve Database User
	dbUser := v.GetString("DB_USER")
	if u := v.GetString("DB_USERNAME"); u != "" {
		dbUser = u
	} else if u := v.GetString("DATABASE_USER"); u != "" {
		dbUser = u
	}

	// Resolve Database Password
	dbPass := v.GetString("DB_PASSWORD")
	if p := v.GetString("DB_PASS"); p != "" {
		dbPass = p
	} else if p := v.GetString("DATABASE_PASSWORD"); p != "" {
		dbPass = p
	}

	// Resolve Database Name
	dbName := v.GetString("DB_NAME")
	if n := v.GetString("DB_DATABASE"); n != "" {
		dbName = n
	} else if n := v.GetString("DATABASE_NAME"); n != "" {
		dbName = n
	}

	cfg := &Config{
		Validation: ValidationConfig{Concurrency: v.GetInt("VALIDATION_CONCURRENCY"), PollInterval: v.GetDuration("VALIDATION_POLL_INTERVAL"), JobTimeout: v.GetDuration("VALIDATION_JOB_TIMEOUT"), LeaseDuration: v.GetDuration("VALIDATION_LEASE_DURATION"), MaxAttempts: v.GetInt("VALIDATION_MAX_ATTEMPTS"), MaxCases: v.GetInt("VALIDATION_MAX_CASES"), WorkspaceDir: v.GetString("VALIDATION_WORKSPACE_DIR"), SyncTimeout: v.GetDuration("UPSTREAM_SYNC_TIMEOUT")},
		Storage: StorageConfig{
			SnapshotDir: v.GetString("SNAPSHOT_DIR"), PlanLifetime: v.GetDuration("PLAN_LIFETIME"), RetainSnapshots: v.GetInt("SNAPSHOT_RETAIN_COUNT"), MaxPendingJobs: v.GetInt("VALIDATION_MAX_PENDING_JOBS"), DiagnosticBytes: v.GetInt("VALIDATION_DIAGNOSTIC_BYTES"),
		},
		App: AppConfig{
			Name:         v.GetString("APP_NAME"),
			Environment:  v.GetString("APP_ENV"),
			Port:         port,
			ReadTimeout:  v.GetDuration("APP_READ_TIMEOUT"),
			WriteTimeout: v.GetDuration("APP_WRITE_TIMEOUT"),
			IdleTimeout:  v.GetDuration("APP_IDLE_TIMEOUT"),
		},
		Database: DatabaseConfig{
			Host:            v.GetString("DB_HOST"),
			Port:            v.GetString("DB_PORT"),
			User:            dbUser,
			Password:        dbPass,
			Name:            dbName,
			SSLMode:         v.GetString("DB_SSLMODE"),
			DSN:             dsn,
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME"),
		},
		Upstream: UpstreamConfig{
			Owner:  v.GetString("UPSTREAM_OWNER"),
			Repo:   v.GetString("UPSTREAM_REPO"),
			Branch: v.GetString("UPSTREAM_BRANCH"),
		},
		Admin: AdminConfig{
			DefaultUser:     v.GetString("ADMIN_DEFAULT_USER"),
			DefaultPassword: v.GetString("ADMIN_DEFAULT_PASSWORD"),
			SessionLifetime: v.GetDuration("ADMIN_SESSION_LIFETIME"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.App.WriteTimeout < 60*time.Second {
		return fmt.Errorf("APP_WRITE_TIMEOUT must be at least 60s for bounded validation wait and materialization")
	}
	if err := c.Validation.Validate(); err != nil {
		return err
	}
	if c.Database.MaxOpenConns < 1 || c.Database.MaxOpenConns > 1000 || c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns || c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("invalid database pool configuration")
	}
	if c.Admin.SessionLifetime <= 0 || c.Admin.SessionLifetime > 30*24*time.Hour {
		return fmt.Errorf("invalid admin session lifetime")
	}

	s := c.Storage
	if s.SnapshotDir == "" || s.PlanLifetime <= 0 || s.PlanLifetime > 30*24*time.Hour || s.RetainSnapshots < 1 || s.RetainSnapshots > 1000 || s.MaxPendingJobs < 1 || s.MaxPendingJobs > 10000 || s.DiagnosticBytes < 256 || s.DiagnosticBytes > 8192 {
		return fmt.Errorf("invalid storage limits")
	}
	return nil
}
