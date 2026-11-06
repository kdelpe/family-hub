package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdata/migrations: 000001 creates widgets, 000002 adds widgets.name.
var testMigrations = os.DirFS("testdata")

func TestMigrateUp(t *testing.T) {
	tests := []struct {
		name        string
		runs        int
		wantVersion int
	}{
		{name: "applies every migration", runs: 1, wantVersion: 2},
		{name: "second run is a no-op", runs: 2, wantVersion: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)

			for range tt.runs {
				require.NoError(t, migrateUp(testMigrations, db.url))
			}

			assert.Equal(t, tt.wantVersion, db.version(t))
			assert.True(t, db.columnExists(t, "widgets", "name"))
		})
	}
}

func TestMigrateDown(t *testing.T) {
	tests := []struct {
		name          string
		steps         int
		wantErrMsg    string
		wantVersion   int
		wantWidgets   bool
		wantNameField bool
	}{
		{name: "one step", steps: 1, wantVersion: 1, wantWidgets: true},
		{name: "every step", steps: 2, wantVersion: 0},
		{name: "zero steps", steps: 0, wantErrMsg: "steps must be a positive integer, got 0", wantVersion: 2, wantWidgets: true, wantNameField: true},
		{name: "more steps than applied changes nothing", steps: 3, wantErrMsg: "only 2 applied, refusing to roll back 3", wantVersion: 2, wantWidgets: true, wantNameField: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)
			require.NoError(t, migrateUp(testMigrations, db.url))

			err := migrateDown(testMigrations, db.url, tt.steps)

			if tt.wantErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
			assert.Equal(t, tt.wantVersion, db.version(t))
			assert.Equal(t, tt.wantWidgets, db.tableExists(t, "widgets"))
			assert.Equal(t, tt.wantNameField, db.columnExists(t, "widgets", "name"))
		})
	}
}

func TestMigrateDownWithNothingApplied(t *testing.T) {
	db := newTestSchema(t)

	err := migrateDown(testMigrations, db.url, 1)

	require.Error(t, err)
	assert.Equal(t, "migrate down: no migrations applied", err.Error())
}

func TestMigrateRefusesUnsafeDatabase(t *testing.T) {
	tests := []struct {
		name       string
		setup      string
		migrate    func(url string) error
		wantErrMsg string
	}{
		{
			name:       "up on a dirty database",
			setup:      "UPDATE schema_migrations SET dirty = true",
			migrate:    func(url string) error { return migrateUp(testMigrations, url) },
			wantErrMsg: "migrate up: database is dirty at version 2",
		},
		{
			name:       "down on a dirty database",
			setup:      "UPDATE schema_migrations SET dirty = true",
			migrate:    func(url string) error { return migrateDown(testMigrations, url, 1) },
			wantErrMsg: "migrate down: database is dirty at version 2",
		},
		{
			name:       "up at a version this binary doesn't have",
			setup:      "UPDATE schema_migrations SET version = 99",
			migrate:    func(url string) error { return migrateUp(testMigrations, url) },
			wantErrMsg: "migrate up: database is at version 99, which this binary doesn't have",
		},
		{
			name:       "down at a version this binary doesn't have",
			setup:      "UPDATE schema_migrations SET version = 99",
			migrate:    func(url string) error { return migrateDown(testMigrations, url, 1) },
			wantErrMsg: "migrate down: database is at version 99, which this binary doesn't have",
		},
		{
			name:       "embedded up at a version this binary doesn't have",
			setup:      "UPDATE schema_migrations SET version = 99",
			migrate:    MigrateUp,
			wantErrMsg: "migrate up: database is at version 99, which this binary doesn't have",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)
			require.NoError(t, migrateUp(testMigrations, db.url))
			db.exec(t, tt.setup)

			err := tt.migrate(db.url)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
			assert.True(t, db.columnExists(t, "widgets", "name"), "schema must be untouched")
		})
	}
}

func TestMigrateUpEmbedded(t *testing.T) {
	db := newTestSchema(t)

	require.NoError(t, MigrateUp(db.url))

	assert.Equal(t, 0, db.version(t))
}

func TestToPgx5Scheme(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "postgres", in: "postgres://app@db/familyhub", want: "pgx5://app@db/familyhub"},
		{name: "postgresql", in: "postgresql://app@db/familyhub", want: "pgx5://app@db/familyhub"},
		{name: "other prefixes untouched", in: "pgx5://app@db/familyhub", want: "pgx5://app@db/familyhub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toPgx5Scheme(tt.in))
		})
	}
}

// testSchema is a throwaway Postgres schema, so tests never share tables or migration state.
type testSchema struct {
	url  string
	conn *pgx.Conn
	name string
}

func newTestSchema(t *testing.T) testSchema {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL must be set in CI")
		}
		t.Skip("TEST_DATABASE_URL not set; run `make up` and `make test`")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })

	name := fmt.Sprintf("test_%d", time.Now().UnixNano())
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{name}.Sanitize()+" CASCADE")
	})

	u, err := url.Parse(base)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()

	return testSchema{url: u.String(), conn: conn, name: name}
}

func (s testSchema) exec(t *testing.T, sql string) {
	t.Helper()
	_, err := s.conn.Exec(context.Background(), "SET search_path TO "+pgx.Identifier{s.name}.Sanitize()+"; "+sql)
	require.NoError(t, err)
}

func (s testSchema) version(t *testing.T) int {
	t.Helper()
	if !s.tableExists(t, "schema_migrations") {
		return 0
	}
	var version int
	err := s.conn.QueryRow(context.Background(),
		"SELECT coalesce(max(version), 0) FROM "+pgx.Identifier{s.name, "schema_migrations"}.Sanitize()).Scan(&version)
	require.NoError(t, err)
	return version
}

func (s testSchema) tableExists(t *testing.T, table string) bool {
	t.Helper()
	var exists bool
	err := s.conn.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)",
		s.name, table).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func (s testSchema) columnExists(t *testing.T, table, column string) bool {
	t.Helper()
	var exists bool
	err := s.conn.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name = $3)",
		s.name, table, column).Scan(&exists)
	require.NoError(t, err)
	return exists
}
