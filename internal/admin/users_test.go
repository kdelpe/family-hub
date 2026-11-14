package admin

import (
	"encoding/base64"
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
	withPassword = `UPDATE users SET password_hash = 'hash', password_changed_at = now() WHERE id = $1`
	withInvite   = `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, 'invite', sha256(gen_random_uuid()::text::bytea), now() + interval '7 days')`
)

func TestListShowsEachAccountsStatus(t *testing.T) {
	tests := []struct {
		name         string
		setup        []string
		wantStatus   string
		wantExpiring bool
	}{
		{name: "a password set up", setup: []string{withPassword}, wantStatus: StatusActive},
		{name: "Google linked", setup: []string{`UPDATE users SET google_sub = 'sub-1' WHERE id = $1`}, wantStatus: StatusActive},
		{name: "invited", setup: []string{withInvite}, wantStatus: StatusInvited, wantExpiring: true},
		{name: "invited, link expired", setup: []string{withInvite, `UPDATE auth_tokens SET expires_at = now() - interval '1 day' WHERE user_id = $1`},
			wantStatus: StatusInvited, wantExpiring: true},
		{name: "invited, no link left", wantStatus: StatusInvited},
		{name: "an active user's leftover invite isn't shown", setup: []string{withPassword, withInvite}, wantStatus: StatusActive},
		{name: "disabled", setup: []string{withPassword, `UPDATE users SET disabled_at = now() WHERE id = $1`}, wantStatus: StatusDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			id := insertUser(t, pool, testEmail)
			for _, sql := range tt.setup {
				_, err := pool.Exec(t.Context(), sql, id)
				require.NoError(t, err)
			}

			page, err := NewUsers(pool, nil, testBaseURL).List(t.Context(), ListInput{Limit: DefaultPageSize})

			require.NoError(t, err)
			require.Len(t, page.Accounts, 1)
			got := page.Accounts[0]
			assert.Equal(t, id, got.ID)
			assert.Equal(t, testEmail, got.Email)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantStatus == StatusDisabled, got.DisabledAt != nil)
			if !tt.wantExpiring {
				assert.Nil(t, got.InviteExpiresAt)
				return
			}
			var want time.Time
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT expires_at FROM auth_tokens WHERE user_id = $1`, id).Scan(&want))
			require.NotNil(t, got.InviteExpiresAt)
			assert.True(t, want.Equal(*got.InviteExpiresAt))
		})
	}
}

func TestListPagesByEmail(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	for _, email := range []string{"c@example.com", "a@example.com", "b@example.com"} {
		insertUser(t, pool, email)
	}
	users := NewUsers(pool, nil, testBaseURL)

	first, err := users.List(t.Context(), ListInput{Limit: 2})
	require.NoError(t, err)
	second, err := users.List(t.Context(), ListInput{Limit: 2, Cursor: first.NextCursor})
	require.NoError(t, err)

	assert.Equal(t, []string{"a@example.com", "b@example.com"}, emails(first))
	assert.NotEmpty(t, first.NextCursor)
	assert.Equal(t, []string{"c@example.com"}, emails(second))
	assert.Empty(t, second.NextCursor, "the last page")
}

func TestListRefusesABadPage(t *testing.T) {
	tests := []struct {
		name    string
		in      ListInput
		wantErr error
	}{
		{name: "limit too big", in: ListInput{Limit: MaxPageSize + 1}, wantErr: ErrInvalidPageSize},
		{name: "negative limit", in: ListInput{Limit: -1}, wantErr: ErrInvalidPageSize},
		{name: "no limit", in: ListInput{}, wantErr: ErrInvalidPageSize},
		{name: "cursor not base64", in: ListInput{Limit: DefaultPageSize, Cursor: "not a cursor!"}, wantErr: ErrInvalidCursor},
		{name: "cursor without an id", in: ListInput{Limit: DefaultPageSize, Cursor: base64.RawURLEncoding.EncodeToString([]byte("a@example.com"))}, wantErr: ErrInvalidCursor},
		{name: "cursor without an email", in: ListInput{Limit: DefaultPageSize, Cursor: base64.RawURLEncoding.EncodeToString([]byte("\x00" + uuid.NewString()))}, wantErr: ErrInvalidCursor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUsers(nil, nil, testBaseURL).List(t.Context(), tt.in)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email) VALUES ($1) RETURNING id`, email).Scan(&id))
	return id
}

func emails(page Page) []string {
	var out []string
	for _, a := range page.Accounts {
		out = append(out, a.Email)
	}
	return out
}
