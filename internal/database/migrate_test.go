package database

import (
	"errors"
	"io"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/stub"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Exercise migration discovery and version transitions without PostgreSQL.
func TestEmbeddedMigrationLifecycle(t *testing.T) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := (&stub.Stub{}).Open("")
	if err != nil {
		t.Fatal(err)
	}
	runner, err := migrate.NewWithInstance("iofs", source, "stub", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	if err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	version, dirty, err := runner.Version()
	if err != nil || version != 2 || dirty {
		t.Fatalf("version=%d dirty=%v err=%v", version, dirty, err)
	}
	if err := runner.Up(); err != migrate.ErrNoChange {
		t.Fatalf("second Up: %v", err)
	}
	if err := runner.Down(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runner.Version(); err != migrate.ErrNilVersion {
		t.Fatalf("after Down: %v", err)
	}
	if err := runner.Up(); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if err := driver.SetVersion(1, true); err != nil {
		t.Fatal(err)
	}
	var dirtyErr migrate.ErrDirty
	if err := runner.Up(); !errors.As(err, &dirtyErr) {
		t.Fatalf("dirty database must block migrations: %v", err)
	}
}

func TestEmbeddedMigrationDirectionsNotEmpty(t *testing.T) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for version := uint(1); version <= 2; version++ {
		for _, read := range []func(uint) (io.ReadCloser, string, error){source.ReadUp, source.ReadDown} {
			r, _, err := read(version)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(r)
			r.Close()
			if err != nil || len(body) == 0 {
				t.Fatalf("empty or unreadable migration: %v", err)
			}
		}
	}
}
