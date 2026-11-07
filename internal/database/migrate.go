package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	connectTimeout = 10 * time.Second
	lockWait       = 5 * time.Minute
)

// The whole directory, so this compiles before the first .sql file exists.
//
//go:embed migrations
var embedded embed.FS

func MigrateUp(ctx context.Context, databaseURL string) error {
	return migrateUp(ctx, embedded, databaseURL)
}

func migrateUp(ctx context.Context, fsys fs.FS, databaseURL string) error {
	m, err := newMigrate(fsys, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if m == nil {
		return nil
	}
	defer func() { _, _ = m.Close() }()
	last, err := lastVersion(fsys)
	if err != nil {
		return fmt.Errorf("migrate up: migrations source: %w", err)
	}

	// One step at a time, so a canceled ctx stops between migrations, never inside one.
	// (golang-migrate's own GracefulStop is a data race.)
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("migrate up: stopped: %w", err)
		}
		err := m.Steps(1)
		// Past the last migration Steps reports "not exist"; so does an unknown version, which isn't at last.
		if errors.Is(err, fs.ErrNotExist) {
			if v, dirty, verr := m.Version(); verr == nil && !dirty && v == last {
				return nil
			}
		}
		if err != nil {
			return fmt.Errorf("migrate up: %w", explain(err))
		}
	}
}

func lastVersion(fsys fs.FS) (uint, error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return 0, err
	}
	defer func() { _ = src.Close() }()
	var last uint
	v, err := src.First()
	for err == nil {
		last = v
		v, err = src.Next(v)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	return last, nil
}

func newMigrate(fsys fs.FS, databaseURL string) (m *migrate.Migrate, err error) {
	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	defer func() {
		if m == nil {
			_ = src.Close()
		}
	}()
	if _, err := src.First(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("migrations source: %w", err)
	}

	connURL, err := withTimeouts(databaseURL)
	if err != nil {
		return nil, err
	}
	// pgx's redaction of the password is best effort, so its error is dropped.
	cfg, err := pgx.ParseConfig(connURL)
	if err != nil {
		return nil, errInvalidConnString
	}
	// Opened here, not from a URL by the driver: it re-encodes the query, turning %20 into a + that pgx reads literally.
	db := stdlib.OpenDB(*cfg)
	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init: %w", err)
	}
	m, err = migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("init: %w", err)
	}
	// Postgres's lock_timeout is the real bound; this only has to outlast it, even when the URL raises it.
	m.LockTimeout = 24 * time.Hour
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
func withTimeouts(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", errors.New("DATABASE_URL is not a valid URL")
	}
	query := u.Query()
	for _, param := range []struct{ key, value string }{
		{"connect_timeout", strconv.Itoa(int(connectTimeout.Seconds()))},
		{"lock_timeout", strconv.FormatInt(lockWait.Milliseconds(), 10)},
	} {
		if !query.Has(param.key) {
			appendRawQuery(u, param.key, param.value)
		}
	}
	return u.String(), nil
}

// Re-encoding the whole query would change how pgx reads it, so value must already be query-safe.
func appendRawQuery(u *url.URL, key, value string) {
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += key + "=" + value
}
