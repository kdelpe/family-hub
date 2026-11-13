package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	checkViolation = "23514"
)

func TestAuthSchemaConstraints(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		wantCode string
	}{
		{name: "a lowercase email is accepted", sql: `INSERT INTO users (email) VALUES ('new@example.com')`},
		{name: "an email must be lowercase", sql: `INSERT INTO users (email) VALUES ('Test@example.com')`, wantCode: checkViolation},
		{name: "emails are unique", sql: `INSERT INTO users (email) VALUES ('test@example.com')`, wantCode: UniqueViolation},
		{name: "a role is admin or member", sql: `INSERT INTO users (email, role) VALUES ('new@example.com', 'owner')`, wantCode: checkViolation},
		{name: "a password hash needs its change time", sql: `INSERT INTO users (email, password_hash) VALUES ('new@example.com', 'x')`, wantCode: checkViolation},
		{name: "a change time needs a password hash", sql: `INSERT INTO users (email, password_changed_at) VALUES ('new@example.com', now())`, wantCode: checkViolation},
		{name: "a Google account links to one login", sql: `INSERT INTO users (email, google_sub) VALUES ('new@example.com', 'sub-1')`, wantCode: UniqueViolation},
		{name: "a token purpose is invite or password_reset", sql: `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
			SELECT id, 'login', '\x02', now() FROM users`, wantCode: checkViolation},
		{name: "a token hash is used once", sql: `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
			SELECT id, 'invite', '\x01', now() FROM users`, wantCode: UniqueViolation},
	}

	db := authSchema(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := db.begin(t)

			_, err := tx.Exec(t.Context(), tt.sql)

			if tt.wantCode == "" {
				require.NoError(t, err)
				return
			}
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			assert.Equal(t, tt.wantCode, pgErr.Code)
		})
	}
}

func TestDeletingAUserDeletesTheirTokensAndSessions(t *testing.T) {
	tx := authSchema(t).begin(t)

	_, err := tx.Exec(t.Context(), `DELETE FROM users`)
	require.NoError(t, err)

	for _, table := range []string{"auth_tokens", "sessions"} {
		var n int
		require.NoError(t, tx.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n))
		assert.Zero(t, n, table)
	}
}

// Holds test@example.com (Google sub-1), a token (hash \x01) and a session; the cases above collide with them.
func authSchema(t *testing.T) testSchema {
	t.Helper()
	db := newTestSchema(t)
	require.NoError(t, MigrateUp(t.Context(), db.url))
	db.exec(t, `
		INSERT INTO users (email, google_sub) VALUES ('test@example.com', 'sub-1');
		INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
			SELECT id, 'invite', '\x01', now() + interval '7 days' FROM users;
		INSERT INTO sessions (id_hash, user_id, expires_at)
			SELECT '\x01', id, now() + interval '180 days' FROM users;`)
	return db
}

// Rolled back when the test ends, so each case starts from the same rows.
func (s testSchema) begin(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := s.conn.Begin(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(t.Context(), "SET LOCAL search_path TO "+pgx.Identifier{s.name}.Sanitize())
	require.NoError(t, err)
	return tx
}
