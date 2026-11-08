package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const (
	anaEmail    = "ana@example.com"
	anaPassword = "correct horse battery"
)

func TestLogin(t *testing.T) {
	tests := []struct {
		name     string
		setup    string
		email    string
		password string
		wantOK   bool
	}{
		{name: "right password", email: anaEmail, password: anaPassword, wantOK: true},
		{name: "email in another case with spaces", email: "  Ana@Example.COM ", password: anaPassword, wantOK: true},
		{name: "wrong password", email: anaEmail, password: "wrong horse battery"},
		{name: "unknown email", email: "bo@example.com", password: anaPassword},
		{name: "no password (Google only)", setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL`, email: anaEmail, password: anaPassword},
		{name: "disabled", setup: `UPDATE users SET disabled_at = now()`, email: anaEmail, password: anaPassword},
		{name: "broken stored hash", setup: `UPDATE users SET password_hash = 'nonsense'`, email: anaEmail, password: anaPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withAna(t)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}

			token, user, err := svc.Login(t.Context(), tt.email, tt.password, "test")

			if !tt.wantOK {
				require.ErrorIs(t, err, ErrInvalidCredentials)
				assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, User{ID: userID, Email: anaEmail, Role: "member"}, user)
			session, err := NewSessions(pool).Authenticate(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t, userID, session.UserID)
			assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM users WHERE last_login_at IS NOT NULL`))
		})
	}
}

func TestLoginRehashesOutdatedParameters(t *testing.T) {
	svc, pool, _ := withAna(t)
	old, err := svc.hasher.hash(t.Context(), anaPassword, params{memory: 8 * 1024, time: 1, threads: 1})
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE users SET password_hash = $1`, old)
	require.NoError(t, err)

	_, _, err = svc.Login(t.Context(), anaEmail, anaPassword, "")
	require.NoError(t, err)

	var stored string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT password_hash FROM users`).Scan(&stored))
	assert.NotEqual(t, old, stored)
	ok, rehash, err := svc.hasher.Verify(t.Context(), stored, anaPassword)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, rehash)
}

func TestLoginRefusedWhenACredentialChangesBeforeTheSessionCommits(t *testing.T) {
	tests := []struct {
		name   string
		change string
	}{
		{name: "password reset", change: `UPDATE users SET password_hash = password_hash || 'x'`},
		{name: "email change", change: `UPDATE users SET email = 'ana.new@example.com'`},
		{name: "disabled", change: `UPDATE users SET disabled_at = now()`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, _ := withAna(t)
			change, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = change.Rollback(context.Background()) })
			_, err = change.Exec(t.Context(), tt.change)
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() {
				_, _, err := svc.Login(t.Context(), anaEmail, anaPassword, "")
				done <- err
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE wait_event_type = 'Lock' AND query LIKE '-- name: LockUserForSignIn %')`).Scan(&waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "login waits on the change's row lock")
			require.NoError(t, change.Commit(t.Context()))

			assert.ErrorIs(t, <-done, ErrInvalidCredentials)
			assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
		})
	}
}

func TestUserOfADeletedAccountIsNoSession(t *testing.T) {
	svc, pool, id := withAna(t)
	_, err := pool.Exec(t.Context(), `DELETE FROM users`)
	require.NoError(t, err)

	_, err = svc.User(t.Context(), id)

	assert.ErrorIs(t, err, ErrNoSession)
}

func TestLogout(t *testing.T) {
	svc, pool, _ := withAna(t)
	token, _, err := svc.Login(t.Context(), anaEmail, anaPassword, "")
	require.NoError(t, err)

	require.NoError(t, svc.Logout(t.Context(), token))

	assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
}

func withAna(t *testing.T) (*Service, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	hasher := NewHasher()
	hash, err := hasher.Hash(t.Context(), anaPassword)
	require.NoError(t, err)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO users (email, password_hash, password_changed_at) VALUES ($1, $2, now()) RETURNING id`, anaEmail, hash).Scan(&id))
	return NewService(pool, hasher), pool, id
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(t.Context(), sql).Scan(&n))
	return n
}
