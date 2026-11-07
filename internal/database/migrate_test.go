package database

import (
	"context"
	"crypto/rand"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdata/migrations: 000001 creates widgets, 000002 adds widgets.name.
var testMigrations = os.DirFS("testdata")

func TestMigrateUp(t *testing.T) {
	tests := []struct {
		name string
		runs int
	}{
		{name: "applies every migration", runs: 1},
		{name: "second run is a no-op", runs: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)

			for range tt.runs {
				require.NoError(t, migrateUp(testMigrations, db.url))
			}

			assert.Equal(t, 2, db.version(t))
			assert.True(t, db.columnExists(t, "widgets", "name"))
		})
	}
}

func TestMigrateUpConcurrently(t *testing.T) {
	db := newTestSchema(t)

	errs := make(chan error, 4)
	for range cap(errs) {
		go func() { errs <- migrateUp(testMigrations, db.url) }()
	}
	for range cap(errs) {
		require.NoError(t, <-errs)
	}

	assert.Equal(t, 2, db.version(t))
}

func TestMigrateUpRefusesUnsafeDatabase(t *testing.T) {
	tests := []struct {
		name       string
		setup      string
		wantErrMsg string
	}{
		{name: "dirty", setup: "UPDATE schema_migrations SET dirty = true", wantErrMsg: "migrate up: database is dirty at version 2"},
		{name: "dirty before any version", setup: "UPDATE schema_migrations SET version = -1, dirty = true", wantErrMsg: "migrate up: database is dirty at version -1"},
		{name: "unknown version", setup: "UPDATE schema_migrations SET version = 99", wantErrMsg: "migrate up: database is at a version this binary doesn't have"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)
			require.NoError(t, migrateUp(testMigrations, db.url))
			db.exec(t, tt.setup)

			err := migrateUp(testMigrations, db.url)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestMigrateUpWithNoMigrationsDoesNotConnect(t *testing.T) {
	empty := fstest.MapFS{"migrations/README.md": {Data: []byte("none yet")}}

	require.NoError(t, migrateUp(empty, "postgres://app@127.0.0.1:1/unreachable"))
}

func TestEmbeddedMigrationsRoundTrip(t *testing.T) {
	versions, err := sourceVersions(embedded)
	require.NoError(t, err)
	if len(versions) == 0 {
		t.Skip("no embedded migrations yet; Phase 2 adds the first")
	}
	db := newTestSchema(t)

	require.NoError(t, MigrateUp(db.url))
	db.rollBackAll(t)
	require.NoError(t, MigrateUp(db.url))

	assert.Equal(t, int(slices.Max(versions)), db.version(t))
}

func TestToMigrateURL(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		want       string
		wantErrMsg string
	}{
		{name: "postgres", in: "postgres://app@db/familyhub", want: "pgx5://app@db/familyhub?connect_timeout=10"},
		{name: "postgresql", in: "postgresql://app@db/familyhub?sslmode=disable", want: "pgx5://app@db/familyhub?connect_timeout=10&sslmode=disable"},
		{name: "keeps an explicit connect_timeout", in: "postgres://app@db/familyhub?connect_timeout=3", want: "pgx5://app@db/familyhub?connect_timeout=3"},
		{name: "other schemes", in: "mysql://app:hunter2@db/familyhub", wantErrMsg: "DATABASE_URL must be a postgres:// URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toMigrateURL(tt.in)

			if tt.wantErrMsg != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErrMsg, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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

	name := "test_" + strings.ToLower(rand.Text())
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

func (s testSchema) rollBackAll(t *testing.T) {
	t.Helper()
	migrateURL, err := toMigrateURL(s.url)
	require.NoError(t, err)
	src, err := iofs.New(embedded, "migrations")
	require.NoError(t, err)
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Down())
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
