package database

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Embeds the whole directory so it compiles before the first .sql file exists; iofs skips the README.
//
//go:embed migrations
var embedded embed.FS

func MigrateUp(databaseURL string) error {
	return migrateUp(embedded, databaseURL)
}

func migrateUp(fsys fs.FS, databaseURL string) error {
	m, versions, err := open(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if len(versions) == 0 {
		return nil
	}
	// No pre-checks: golang-migrate checks dirty and unknown versions under its advisory lock.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", explain(err))
	}
	return nil
}

func migrateDown(fsys fs.FS, databaseURL string, steps int) error {
	m, versions, err := open(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	// golang-migrate rolls back all it can before reporting a short limit, so count first.
	// This read happens outside the lock; down is a manual command, not run at startup.
	version, _, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		// Version hides the dirty flag at nil version; Steps reports it.
		if err := m.Steps(-steps); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("migrate down: %w", explain(err))
		}
		return errors.New("migrate down: no migrations applied")
	case err != nil:
		return fmt.Errorf("migrate down: read version: %w", err)
	}
	if count := appliedCount(versions, version); steps > count {
		return fmt.Errorf("migrate down: only %d applied, refusing to roll back %d", count, steps)
	}
	if err := m.Steps(-steps); err != nil {
		return fmt.Errorf("migrate down: %w", explain(err))
	}
	return nil
}

func open(fsys fs.FS, databaseURL string) (*migrate.Migrate, []uint, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, nil, fmt.Errorf("migrations source: %w", err)
	}
	var versions []uint
	v, err := src.First()
	for err == nil {
		versions = append(versions, v)
		v, err = src.Next(v)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("migrations source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5Scheme(databaseURL))
	if err != nil {
		return nil, nil, fmt.Errorf("init: %w", err)
	}
	return m, versions, nil
}

// explain replaces golang-migrate's "force version" advice: we repair by hand instead.
func explain(err error) error {
	var dirty migrate.ErrDirty
	switch {
	case errors.As(err, &dirty):
		return fmt.Errorf("database is dirty at version %d: a migration failed partway; repair it as in architecture §6.4", dirty.Version)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("database is at a version this binary doesn't have (an older release?): %w", err)
	}
	return err
}

func appliedCount(versions []uint, current uint) int {
	count := 0
	for _, v := range versions {
		if v <= current {
			count++
		}
	}
	return count
}

func toPgx5Scheme(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(databaseURL, prefix); ok {
			return "pgx5://" + rest
		}
	}
	return databaseURL
}
