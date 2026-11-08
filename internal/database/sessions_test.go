package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

var seededSessionHash = []byte{1}

func TestCreateSession(t *testing.T) {
	tx := authSchema(t).begin(t)
	userID := seededUserID(t, tx)
	agent := "test"

	require.NoError(t, gen.New(tx).CreateSession(t.Context(), gen.CreateSessionParams{
		IDHash: []byte{2}, UserID: userID, Lifetime: time.Hour, UserAgent: &agent,
	}))

	var gotAgent string
	var gotLifetime time.Duration
	require.NoError(t, tx.QueryRow(t.Context(),
		`SELECT user_agent, expires_at - now() FROM sessions WHERE id_hash = '\x02'`).Scan(&gotAgent, &gotLifetime))
	assert.Equal(t, agent, gotAgent)
	assert.Equal(t, time.Hour, gotLifetime)
}

func TestGetActiveSession(t *testing.T) {
	db := authSchema(t)
	tests := []struct {
		name        string
		idHash      []byte
		setup       string
		idleTimeout time.Duration
		wantFound   bool
	}{
		{name: "an active session", idleTimeout: time.Hour, wantFound: true},
		{name: "an unknown id", idHash: []byte{9}, idleTimeout: time.Hour},
		{name: "idle too long", setup: `UPDATE sessions SET last_seen_at = now() - interval '2 hours'`, idleTimeout: time.Hour},
		{name: "past its absolute expiry", setup: `UPDATE sessions SET expires_at = now() - interval '1 second'`, idleTimeout: time.Hour},
		{name: "a disabled user", setup: `UPDATE users SET disabled_at = now()`, idleTimeout: time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := db.begin(t)
			if tt.setup != "" {
				_, err := tx.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}

			idHash := seededSessionHash
			if tt.idHash != nil {
				idHash = tt.idHash
			}

			row, err := gen.New(tx).GetActiveSession(t.Context(), gen.GetActiveSessionParams{IDHash: idHash, IdleTimeout: tt.idleTimeout})

			if !tt.wantFound {
				require.ErrorIs(t, err, pgx.ErrNoRows)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, seededUserID(t, tx), row.UserID)
			assert.Equal(t, "member", row.Role)
			var lastSeen time.Time
			require.NoError(t, tx.QueryRow(t.Context(), `SELECT last_seen_at FROM sessions`).Scan(&lastSeen))
			assert.Equal(t, lastSeen, row.LastSeenAt)
		})
	}
}

func TestTouchSessionOnlyRewritesAStaleLastSeen(t *testing.T) {
	tx := authSchema(t).begin(t)
	q := gen.New(tx)
	lastSeen := func() time.Time {
		var at time.Time
		require.NoError(t, tx.QueryRow(t.Context(), `SELECT last_seen_at FROM sessions`).Scan(&at))
		return at
	}
	_, err := tx.Exec(t.Context(), `UPDATE sessions SET last_seen_at = now() - interval '2 hours'`)
	require.NoError(t, err)
	before := lastSeen()

	require.NoError(t, q.TouchSession(t.Context(), gen.TouchSessionParams{IDHash: seededSessionHash, TouchAfter: 3 * time.Hour}))
	assert.Equal(t, before, lastSeen(), "fresh enough, left alone")

	require.NoError(t, q.TouchSession(t.Context(), gen.TouchSessionParams{IDHash: seededSessionHash, TouchAfter: time.Hour}))
	var now time.Time
	require.NoError(t, tx.QueryRow(t.Context(), `SELECT now()`).Scan(&now))
	assert.Equal(t, now, lastSeen(), "stale, set to now()")
}

func TestDeleteSession(t *testing.T) {
	tx := authSchema(t).begin(t)

	require.NoError(t, gen.New(tx).DeleteSession(t.Context(), seededSessionHash))

	var n int
	require.NoError(t, tx.QueryRow(t.Context(), `SELECT count(*) FROM sessions`).Scan(&n))
	assert.Zero(t, n)
}

func seededUserID(t *testing.T, tx pgx.Tx) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, tx.QueryRow(t.Context(), `SELECT id FROM users`).Scan(&id))
	return id
}
