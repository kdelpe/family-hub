// Package testdb is for tests only: the shared test Postgres, and a database that is never there.
package testdb

import (
	"context"
	"crypto/rand"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UnreachableURL refuses every connection at once: nothing listens on port 1.
const UnreachableURL = "postgres://app@127.0.0.1:1/familyhub"

// URL returns TEST_DATABASE_URL, skipping the test without it, except in CI where it must be set.
func URL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL must be set in CI")
		}
		t.Skip("TEST_DATABASE_URL not set; run `make up` and `make test`")
	}
	return url
}

// Pool connects to the test Postgres, skipping or failing as URL does.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Bounded, so an address that drops packets fails the test instead of hanging it.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

// UnreachablePool connects lazily, so creating it succeeds and every use fails.
func UnreachablePool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), UnreachableURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// InstallExtensions puts the migrations' extensions in public, where every test schema finds them
// through SearchPath: one made inside a test schema would vanish when that schema is dropped.
func InstallExtensions(t testing.TB, conn *pgx.Conn) {
	t.Helper()
	ctx := t.Context()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Test packages run in parallel, and two concurrent CREATE EXTENSION IF NOT EXISTS can still collide.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('testdb.extensions'))"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// SearchPath is a test schema's search_path, with public after it for the shared extensions.
func SearchPath(schema string) string {
	return schema + ",public"
}

// MigratedPool connects to a throwaway schema, dropped when the test ends, after running migrate on it.
func MigratedPool(t testing.TB, migrate func(ctx context.Context, url string) error) *pgxpool.Pool {
	t.Helper()
	base := URL(t)
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	admin, err := pgx.Connect(connectCtx, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(ctx) })
	InstallExtensions(t, admin)
	schema := "test_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })

	// Appended raw: re-encoding the query turns %20 into +, which pgx reads literally.
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += "search_path=" + SearchPath(schema)
	if err := migrate(t.Context(), u.String()); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(t.Context(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
