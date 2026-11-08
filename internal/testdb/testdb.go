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

// MigratedPool connects to a throwaway schema, dropped when the test ends, after running migrate on it.
func MigratedPool(t testing.TB, migrate func(ctx context.Context, url string) error) *pgxpool.Pool {
	t.Helper()
	admin := Pool(t)
	ctx := context.Background()
	schema := pgx.Identifier{"test_" + strings.ToLower(rand.Text())}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema.Sanitize()+" CASCADE") })

	u, err := url.Parse(URL(t))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema[0])
	u.RawQuery = q.Encode()
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
