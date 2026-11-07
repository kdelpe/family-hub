// Package testdb is for tests only: the shared test Postgres, and a database that is never there.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

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
	pool, err := pgxpool.New(context.Background(), URL(t))
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
	pool, err := pgxpool.New(context.Background(), UnreachableURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
