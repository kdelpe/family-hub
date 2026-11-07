package database

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const (
	connectTimeout = 10 * time.Second
	lockWait       = 5 * time.Minute
)

// The whole directory, so this compiles before the first .sql file exists.
//
//go:embed migrations
var embedded embed.FS

func MigrateUp(databaseURL string) error {
	return migrateUp(embedded, databaseURL)
}

func migrateUp(fsys fs.FS, databaseURL string) error {
	m, err := newMigrate(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if m == nil {
		return nil
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", explain(err))
	}
	return nil
}

func newMigrate(fsys fs.FS, databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	if _, err := src.First(); err != nil {
		_ = src.Close()
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("migrations source: %w", err)
	}

	migrateURL, err := toMigrateURL(databaseURL)
	if err != nil {
		_ = src.Close()
		return nil, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		_ = src.Close()
		return nil, fmt.Errorf("init: %w", err)
	}
	// Longer than lock_timeout, so Postgres ends the wait rather than leaving the lock query running.
	m.LockTimeout = lockWait + time.Minute
	return m, nil
}

func explain(err error) error {
	var dirty migrate.ErrDirty
	switch {
	case errors.As(err, &dirty):
		return fmt.Errorf("database is dirty at version %d: a migration failed partway; repair it as in architecture §6.4", dirty.Version)
	// It wraps fs.ErrNotExist, but so does a missing socket; the message is the precise signal.
	case strings.Contains(err.Error(), "no migration found for version"):
		return fmt.Errorf("database is at a version this binary doesn't have (an older release?): %w", err)
	}
	return err
}

// url.Parse errors are dropped: they quote the password.
func toMigrateURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", errors.New("DATABASE_URL is not a valid URL")
	}
	u.Scheme = "pgx5"
	query := u.Query()
	if query.Get("connect_timeout") == "" {
		query.Set("connect_timeout", strconv.Itoa(int(connectTimeout.Seconds())))
	}
	if query.Get("lock_timeout") == "" {
		query.Set("lock_timeout", strconv.FormatInt(lockWait.Milliseconds(), 10))
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}
