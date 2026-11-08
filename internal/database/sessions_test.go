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

var sessionHash = []byte{1}

func TestGetActiveSession(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		setup     string
		idleSince time.Time
		wantFound bool
	}{
		{name: "an active session", idleSince: now.Add(-time.Hour), wantFound: true},
		{name: "idle too long", idleSince: now.Add(time.Hour)},
		{name: "past its absolute expiry", setup: `UPDATE sessions SET expires_at = now() - interval '1 second'`, idleSince: now.Add(-time.Hour)},
		{name: "a disabled user", setup: `UPDATE users SET disabled_at = now()`, idleSince: now.Add(-time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, userID := withOneSession(t)
			if tt.setup != "" {
				_, err := tx.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}

			row, err := gen.New(tx).GetActiveSession(t.Context(), gen.GetActiveSessionParams{IDHash: sessionHash, IdleSince: tt.idleSince})

			if !tt.wantFound {
				require.ErrorIs(t, err, pgx.ErrNoRows)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, userID, row.UserID)
			assert.Equal(t, "member", row.Role)
		})
	}
}

func TestTouchSessionOnlyRewritesAStaleLastSeen(t *testing.T) {
	tx, _ := withOneSession(t)
	q := gen.New(tx)
	lastSeen := func() time.Time {
		var at time.Time
		require.NoError(t, tx.QueryRow(t.Context(), `SELECT last_seen_at FROM sessions`).Scan(&at))
		return at
	}
	_, err := tx.Exec(t.Context(), `UPDATE sessions SET last_seen_at = now() - interval '2 hours'`)
	require.NoError(t, err)
	before := lastSeen()

	require.NoError(t, q.TouchSession(t.Context(), gen.TouchSessionParams{IDHash: sessionHash, StaleBefore: before.Add(-time.Minute)}))
	assert.Equal(t, before, lastSeen(), "fresh enough, left alone")

	require.NoError(t, q.TouchSession(t.Context(), gen.TouchSessionParams{IDHash: sessionHash, StaleBefore: before.Add(time.Minute)}))
	assert.True(t, lastSeen().After(before), "stale, rewritten")
}

func TestDeleteSession(t *testing.T) {
	tx, _ := withOneSession(t)

	require.NoError(t, gen.New(tx).DeleteSession(t.Context(), sessionHash))

	var n int
	require.NoError(t, tx.QueryRow(t.Context(), `SELECT count(*) FROM sessions`).Scan(&n))
	assert.Zero(t, n)
}

func withOneSession(t *testing.T) (pgx.Tx, uuid.UUID) {
	t.Helper()
	db := newTestSchema(t)
	require.NoError(t, MigrateUp(t.Context(), db.url))
	tx := db.begin(t)
	var userID uuid.UUID
	require.NoError(t, tx.QueryRow(t.Context(), `INSERT INTO users (email) VALUES ('ana@example.com') RETURNING id`).Scan(&userID))
	agent := "test"
	require.NoError(t, gen.New(tx).CreateSession(t.Context(), gen.CreateSessionParams{
		IDHash: sessionHash, UserID: userID, ExpiresAt: time.Now().Add(180 * 24 * time.Hour), UserAgent: &agent,
	}))
	return tx, userID
}
