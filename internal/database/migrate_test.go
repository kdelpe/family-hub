package database

import (
	"context"
	"crypto/rand"
	"io/fs"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/testdb"
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
				require.NoError(t, migrateUp(t.Context(), testMigrations, db.url))
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
		go func() { errs <- migrateUp(t.Context(), testMigrations, db.url) }()
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
			require.NoError(t, migrateUp(t.Context(), testMigrations, db.url))
			db.exec(t, tt.setup)

			err := migrateUp(t.Context(), testMigrations, db.url)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestMigrateUpWithNoMigrationsDoesNotConnect(t *testing.T) {
	empty := fstest.MapFS{"migrations/README.md": {Data: []byte("none yet")}}

	require.NoError(t, migrateUp(t.Context(), empty, testdb.UnreachableURL))
}

func TestEmbeddedMigrationsRoundTrip(t *testing.T) {
	versions := embeddedVersions(t)
	db := newTestSchema(t)

	require.NoError(t, MigrateUp(t.Context(), db.url))
	db.rollBackAll(t)
	require.NoError(t, MigrateUp(t.Context(), db.url))

	assert.Equal(t, int(slices.Max(versions)), db.version(t))
}

func TestMigrateUpGivesUpOnAHeldLock(t *testing.T) {
	db := newTestSchema(t)
	slow := fstest.MapFS{
		"migrations/000001_slow.up.sql":   {Data: []byte("SELECT pg_sleep(3);")},
		"migrations/000001_slow.down.sql": {Data: []byte("SELECT 1;")},
	}
	holder := make(chan error, 1)
	// Not t.Context(): that is canceled before the cleanup below waits for the holder.
	go func() { holder <- migrateUp(context.Background(), slow, db.url) }()
	t.Cleanup(func() { assert.NoError(t, <-holder) })
	require.Eventually(t, db.sleeping, 5*time.Second, 50*time.Millisecond)

	start := time.Now()
	err := migrateUp(t.Context(), slow, db.url+"&lock_timeout=500")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock timeout")
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestMigrateUpStopsBetweenMigrationsWhenCanceled(t *testing.T) {
	db := newTestSchema(t)
	slowFirst := fstest.MapFS{
		"migrations/000001_slow.up.sql":      {Data: []byte("SELECT pg_sleep(1);")},
		"migrations/000001_slow.down.sql":    {Data: []byte("SELECT 1;")},
		"migrations/000002_widgets.up.sql":   {Data: []byte("CREATE TABLE widgets (id int);")},
		"migrations/000002_widgets.down.sql": {Data: []byte("DROP TABLE widgets;")},
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- migrateUp(ctx, slowFirst, db.url) }()
	require.Eventually(t, db.sleeping, 5*time.Second, 50*time.Millisecond)

	cancel()

	require.ErrorIs(t, <-done, context.Canceled)
	assert.Equal(t, 1, db.version(t))
	assert.False(t, db.tableExists(t, "widgets"))
}

func TestMigrateUpCanceledDuringTheLastMigrationSucceeds(t *testing.T) {
	db := newTestSchema(t)
	slowOnly := fstest.MapFS{
		"migrations/000001_slow.up.sql":   {Data: []byte("SELECT pg_sleep(1);")},
		"migrations/000001_slow.down.sql": {Data: []byte("SELECT 1;")},
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- migrateUp(ctx, slowOnly, db.url) }()
	require.Eventually(t, db.sleeping, 5*time.Second, 50*time.Millisecond)

	cancel()

	require.NoError(t, <-done)
	assert.Equal(t, 1, db.version(t))
}

func TestWithTimeouts(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		want       string
		wantErrMsg string
	}{
		{name: "adds both", in: "postgres://app@db/familyhub", want: "postgres://app@db/familyhub?connect_timeout=10&lock_timeout=300000"},
		{name: "appends to an existing query", in: "postgresql://app@db/familyhub?sslmode=disable", want: "postgresql://app@db/familyhub?sslmode=disable&connect_timeout=10&lock_timeout=300000"},
		{name: "keeps explicit timeouts", in: "postgres://app@db/familyhub?connect_timeout=3&lock_timeout=1000", want: "postgres://app@db/familyhub?connect_timeout=3&lock_timeout=1000"},
		{name: "keeps query encoding", in: "postgres://app@db/familyhub?password=two%20words", want: "postgres://app@db/familyhub?password=two%20words&connect_timeout=10&lock_timeout=300000"},
		{name: "unparseable URL never quotes it", in: "postgres://app:hunter2@local host/familyhub", wantErrMsg: "DATABASE_URL is not a valid URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := withTimeouts(tt.in)

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

func TestMigrateUpWithASpaceInThePassword(t *testing.T) {
	tests := []struct {
		name  string
		build func(u *url.URL, role, password string)
	}{
		{name: "in the user info", build: func(u *url.URL, role, password string) { u.User = url.UserPassword(role, password) }},
		{name: "in the query", build: func(u *url.URL, role, password string) {
			u.User = url.User(role)
			appendRawQuery(u, "password", strings.ReplaceAll(password, " ", "%20"))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestSchema(t)
			role := pgx.Identifier{db.name + "_role"}.Sanitize()
			password := "two words " + rand.Text()
			db.exec(t, "CREATE ROLE "+role+" LOGIN PASSWORD '"+password+"'; "+
				"GRANT USAGE, CREATE ON SCHEMA "+pgx.Identifier{db.name}.Sanitize()+" TO "+role)
			t.Cleanup(func() { db.exec(t, "DROP OWNED BY "+role+"; DROP ROLE "+role) })
			u, err := url.Parse(db.url)
			require.NoError(t, err)
			tt.build(u, db.name+"_role", password)

			require.NoError(t, migrateUp(t.Context(), testMigrations, u.String()))

			assert.Equal(t, 2, db.version(t))
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
	base := testdb.URL(t)

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
	appendRawQuery(u, "search_path", name)

	return testSchema{url: u.String(), conn: conn, name: name}
}

func (s testSchema) exec(t *testing.T, sql string) {
	t.Helper()
	_, err := s.conn.Exec(context.Background(), "SET search_path TO "+pgx.Identifier{s.name}.Sanitize()+"; "+sql)
	require.NoError(t, err)
}

func (s testSchema) rollBackAll(t *testing.T) {
	t.Helper()
	m, _, err := newMigrate(t.Context(), embedded, s.url)
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Down())
}

func embeddedVersions(t *testing.T) []uint {
	t.Helper()
	src, err := iofs.New(embedded, "migrations")
	require.NoError(t, err)
	defer func() { _ = src.Close() }()

	var versions []uint
	v, err := src.First()
	for err == nil {
		versions = append(versions, v)
		v, err = src.Next(v)
	}
	require.ErrorIs(t, err, fs.ErrNotExist)
	return versions
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

// The holder has the advisory lock once its migration is running.
func (s testSchema) sleeping() bool {
	var sleeping bool
	err := s.conn.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT FROM pg_stat_activity WHERE datname = current_database() AND state = 'active' AND query LIKE 'SELECT pg_sleep(%' AND pid <> pg_backend_pid())").Scan(&sleeping)
	return err == nil && sleeping
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
