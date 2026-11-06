// Package database owns the Postgres schema and the connection to it.
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

// MigrateUp applies every pending migration. golang-migrate holds an advisory lock, so concurrent starts are safe.
func MigrateUp(databaseURL string) error {
	return migrateUp(embedded, databaseURL)
}

// MigrateDown rolls back the given number of applied migrations.
func MigrateDown(databaseURL string, steps int) error {
	return migrateDown(embedded, databaseURL, steps)
}

func migrateUp(fsys fs.FS, databaseURL string) error {
	m, err := newMigrate(fsys, databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	// golang-migrate reports an empty migrations directory as fs.ErrNotExist.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func migrateDown(fsys fs.FS, databaseURL string, steps int) error {
	if steps < 1 {
		return fmt.Errorf("migrate down: steps must be at least 1, got %d", steps)
	}
	m, err := newMigrate(fsys, databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if _, _, err := m.Version(); errors.Is(err, migrate.ErrNilVersion) {
		return errors.New("migrate down: no migrations applied")
	}
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func newMigrate(fsys fs.FS, databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5Scheme(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return m, nil
}

func closeMigrate(m *migrate.Migrate) {
	// Close only releases connections; migration errors are already reported.
	_, _ = m.Close()
}

// The pgx v5 driver registers pgx5://; config.Load guarantees one of these prefixes.
func toPgx5Scheme(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(databaseURL, prefix); ok {
			return "pgx5://" + rest
		}
	}
	return databaseURL
}
