package auth_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestSessionRoundTrip(t *testing.T) {
	pool, userID := withMember(t)
	sessions := auth.NewSessions(pool)

	token, err := sessions.Create(t.Context(), userID, "test agent")
	require.NoError(t, err)

	got, err := sessions.Authenticate(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, auth.Session{UserID: userID, Role: "member"}, got)

	require.NoError(t, sessions.Delete(t.Context(), token))
	_, err = sessions.Authenticate(t.Context(), token)
	assert.ErrorIs(t, err, auth.ErrNoSession)
}

func TestAuthenticateRefusesBadTokens(t *testing.T) {
	pool, userID := withMember(t)
	sessions := auth.NewSessions(pool)
	token, err := sessions.Create(t.Context(), userID, "")
	require.NoError(t, err)

	tests := []struct {
		name  string
		token string
		setup string
	}{
		{name: "empty", token: ""},
		{name: "not base64url", token: "not a token!"},
		{name: "wrong length", token: token[:20]},
		{name: "unknown", token: strings.Repeat("A", len(token))},
		{name: "idle for 30 days", token: token, setup: `UPDATE sessions SET last_seen_at = now() - interval '30 days 1 second'`},
		{name: "past the 180-day cap", token: token, setup: `UPDATE sessions SET expires_at = now()`},
		{name: "disabled user", token: token, setup: `UPDATE users SET disabled_at = now()`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
			if tt.setup != "" {
				_, err := tx.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}

			_, err = auth.NewSessions(tx).Authenticate(t.Context(), tt.token)

			assert.ErrorIs(t, err, auth.ErrNoSession)
		})
	}
}

func TestAuthenticateSlidesTheIdleWindowOnlyWhenStale(t *testing.T) {
	tests := []struct {
		name      string
		lastSeen  time.Duration
		wantTouch bool
	}{
		{name: "seen a minute ago", lastSeen: time.Minute},
		{name: "seen two hours ago", lastSeen: 2 * time.Hour, wantTouch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, userID := withMember(t)
			sessions := auth.NewSessions(pool)
			token, err := sessions.Create(t.Context(), userID, "")
			require.NoError(t, err)
			_, err = pool.Exec(t.Context(), `UPDATE sessions SET last_seen_at = now() - $1::interval`, tt.lastSeen)
			require.NoError(t, err)

			_, err = sessions.Authenticate(t.Context(), token)
			require.NoError(t, err)

			var age time.Duration
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT now() - last_seen_at FROM sessions`).Scan(&age))
			if tt.wantTouch {
				assert.Less(t, age, time.Minute)
			} else {
				assert.GreaterOrEqual(t, age, tt.lastSeen)
			}
		})
	}
}

func TestCreateSetsTheAbsoluteCap(t *testing.T) {
	pool, userID := withMember(t)

	_, err := auth.NewSessions(pool).Create(t.Context(), userID, "")
	require.NoError(t, err)

	var lifetime time.Duration
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT expires_at - created_at FROM sessions`).Scan(&lifetime))
	assert.Equal(t, auth.SessionLifetime, lifetime)
}

func withMember(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email) VALUES ('ana@example.com') RETURNING id`).Scan(&userID))
	return pool, userID
}
