package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"gostarter/internal/auth"
	"gostarter/internal/upstream"
	"log"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	conn *sql.DB
}

// AdminUser represents a simple admin user with username and password hash
type AdminUser = auth.AdminUser

// AdminSettings holds configurable initializr default settings in PostgreSQL
type AdminSettings = upstream.AdminSettings

// ConnectPostgres connects to PostgreSQL and runs simple schema migrations
func ConnectPostgres(dsn string, adminCreds ...string) (*DB, error) {
	return ConnectPostgresWithOptions(dsn, PoolOptions{25, 5, 5 * time.Minute}, adminCreds...)
}

type PoolOptions struct {
	MaxOpenConns, MaxIdleConns int
	ConnMaxLifetime            time.Duration
}

func ConnectPostgresWithOptions(dsn string, pool PoolOptions, adminCreds ...string) (*DB, error) {
	if pool.MaxOpenConns < 1 || pool.MaxIdleConns < 0 || pool.MaxIdleConns > pool.MaxOpenConns || pool.ConnMaxLifetime <= 0 {
		return nil, fmt.Errorf("invalid database pool")
	}
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/gostarter?sslmode=disable"
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening postgres: %w", err)
	}

	conn.SetMaxOpenConns(pool.MaxOpenConns)
	conn.SetMaxIdleConns(pool.MaxIdleConns)
	conn.SetConnMaxLifetime(pool.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed connecting to postgres: %w", err)
	}

	defaultUser := "admin"
	defaultPass := "admin123"
	if len(adminCreds) >= 1 && adminCreds[0] != "" {
		defaultUser = adminCreds[0]
	}
	if len(adminCreds) >= 2 && adminCreds[1] != "" {
		defaultPass = adminCreds[1]
	}

	d := &DB{conn: conn}
	if err := d.migrate(defaultUser, defaultPass); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed migrating postgres: %w", err)
	}

	log.Println("[DB] Successfully connected to PostgreSQL database (gostarter)")
	return d, nil
}

func (d *DB) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

func (d *DB) Ping() error {
	if d.conn == nil {
		return fmt.Errorf("db not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return d.conn.PingContext(ctx)
}

func (d *DB) migrate(defaultUser, defaultPass string) error {
	if err := d.migrateSchema(); err != nil {
		return err
	}
	// Runtime credentials are seeded separately from versioned schema changes.
	d.seedDefaultAdmin(defaultUser, defaultPass)
	d.seedDefaultSettings()
	return nil
}

func (d *DB) seedDefaultAdmin(user, pass string) {
	if d.conn == nil {
		return
	}
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count)
	if err == nil && count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("[DB] Failed hashing default admin password: %v", err)
			return
		}
		_, err = d.conn.Exec(`INSERT INTO admin_users (username, password_hash) VALUES ($1, $2) ON CONFLICT (username) DO NOTHING;`, user, string(hash))
		if err != nil {
			log.Printf("[DB] Failed seeding admin user: %v", err)
		} else {
			log.Printf("[DB] Default admin user '%s' initialized with bcrypt password", user)
		}
	}
}

func (d *DB) seedDefaultSettings() {
	defaults := map[string]string{
		"default_group":      "github.com/example",
		"default_service":    "my-service",
		"default_go_version": "1.27.1",
		"default_protocol":   "dual",
		"upstream_repo":      "kal72/go-feature-based-boilerplate",
		"upstream_branch":    "main",
	}

	for k, v := range defaults {
		_, _ = d.conn.Exec(`INSERT INTO admin_settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING;`, k, v)
	}
}

func randomID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
