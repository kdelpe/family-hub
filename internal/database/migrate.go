package database

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
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

func MigrateDown(databaseURL string, steps int) error {
	return migrateDown(embedded, databaseURL, steps)
}

func migrateUp(fsys fs.FS, databaseURL string) error {
	m, versions, err := open(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if _, _, err := checkedVersion(m, versions); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(versions) == 0 {
		return nil
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func migrateDown(fsys fs.FS, databaseURL string, steps int) error {
	if steps < 1 {
		return fmt.Errorf("migrate down: steps must be a positive integer, got %d", steps)
	}
	m, versions, err := open(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	version, applied, err := checkedVersion(m, versions)
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	if !applied {
		return errors.New("migrate down: no migrations applied")
	}
	// golang-migrate would roll back everything it can before reporting a short limit.
	if count := appliedCount(versions, version); steps > count {
		return fmt.Errorf("migrate down: only %d applied, refusing to roll back %d", count, steps)
	}
	if err := m.Steps(-steps); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func open(fsys fs.FS, databaseURL string) (*migrate.Migrate, []uint, error) {
	versions, err := sourceVersions(fsys)
	if err != nil {
		return nil, nil, err
	}
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, nil, fmt.Errorf("migrations source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5Scheme(databaseURL))
	if err != nil {
		return nil, nil, fmt.Errorf("init: %w", err)
	}
	return m, versions, nil
}

// checkedVersion returns the database's version, refusing a dirty database or one at a
// version this binary has no file for (e.g. after rolling back to an older release).
func checkedVersion(m *migrate.Migrate, versions []uint) (version uint, applied bool, err error) {
	version, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("read version: %w", err)
	case dirty:
		return 0, false, fmt.Errorf("database is dirty at version %d: a migration failed partway; repair it as in architecture §6.4", version)
	case !slices.Contains(versions, version):
		return 0, false, fmt.Errorf("database is at version %d, which this binary doesn't have", version)
	}
	return version, true, nil
}

func sourceVersions(fsys fs.FS) ([]uint, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	defer func() { _ = src.Close() }()

	var versions []uint
	v, err := src.First()
	for err == nil {
		versions = append(versions, v)
		v, err = src.Next(v)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	return versions, nil
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
