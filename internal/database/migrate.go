package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func (d *DB) migrateSchema() (err error) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	defer source.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := d.conn.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()
	// WithConnection owns only this connection, leaving the application pool open.
	driver, err := postgres.WithConnection(ctx, conn, &postgres.Config{StatementTimeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("initialize migration driver: %w", err)
	}
	runner, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		_ = driver.Close()
		return fmt.Errorf("initialize migrations: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := runner.Close()
		err = errors.Join(err, sourceErr, databaseErr)
	}()
	runner.LockTimeout = 30 * time.Second
	if err = runner.Up(); errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
