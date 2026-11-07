package database

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Embeds the whole directory so it compiles before the first .sql file exists; iofs skips the README.
//
//go:embed migrations
var embedded embed.FS

// Fails fast on an unreachable host instead of waiting for the OS TCP timeout.
const defaultConnectTimeoutSeconds = "10"

func MigrateUp(databaseURL string) error {
	return migrateUp(embedded, databaseURL)
}

func migrateUp(fsys fs.FS, databaseURL string) error {
	versions, err := sourceVersions(fsys)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(versions) == 0 {
		return nil
	}

	migrateURL, err := toMigrateURL(databaseURL)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("migrate up: migrations source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return fmt.Errorf("migrate up: init: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	// golang-migrate checks for a dirty or unknown version under its advisory lock.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", explain(err))
	}
	return nil
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

// Parse errors are not wrapped: they quote the URL, password included.
func toMigrateURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", errors.New("DATABASE_URL must be a postgres:// URL")
	}
	u.Scheme = "pgx5"
	query := u.Query()
	if query.Get("connect_timeout") == "" {
		query.Set("connect_timeout", defaultConnectTimeoutSeconds)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}
