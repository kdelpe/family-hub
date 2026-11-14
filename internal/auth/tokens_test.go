package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const newPassword = "purple staple orbit"

func TestSetPasswordWithToken(t *testing.T) {
	tests := []struct {
		name     string
		purpose  string
		setup    string
		token    func(issued string) string
		password string
		wantErr  error
	}{
		{name: "invite for a user without a password", purpose: "invite", setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL`},
		{name: "password reset", purpose: "password_reset"},
		{name: "malformed token", purpose: "password_reset", token: func(string) string { return "not-a-token" }, wantErr: ErrInvalidToken},
		{name: "unknown token", purpose: "password_reset", token: func(string) string { _, t := newToken(); return t }, wantErr: ErrInvalidToken},
		{name: "already used", purpose: "password_reset", setup: `UPDATE auth_tokens SET consumed_at = now()`, wantErr: ErrInvalidToken},
		{name: "expired", purpose: "password_reset", setup: `UPDATE auth_tokens SET expires_at = now() - interval '1 second'`, wantErr: ErrInvalidToken},
		{name: "disabled user", purpose: "invite", setup: `UPDATE users SET disabled_at = now()`, wantErr: ErrInvalidToken},
		{name: "password too short", purpose: "password_reset", password: "short", wantErr: ErrPasswordTooShort},
		{name: "password is the email", purpose: "password_reset", password: testEmail, wantErr: ErrPasswordTooCommon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			token, err := insertToken(t, pool, userID, tt.purpose)
			require.NoError(t, err)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}
			if tt.token != nil {
				token = tt.token(token)
			}
			password := newPassword
			if tt.password != "" {
				password = tt.password
			}
			before := passwordState(t, pool)

			session, user, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: password, UserAgent: "test"})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, passwordState(t, pool))
				assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, User{ID: userID, Email: testEmail, Role: "member"}, user)
			got, err := NewSessions(pool).Authenticate(t.Context(), session)
			require.NoError(t, err)
			assert.Equal(t, userID, got.UserID)
			assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM users WHERE last_login_at IS NOT NULL`))
			_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: newPassword})
			assert.NoError(t, err)
		})
	}
}

func TestSetPasswordWithTokenWorksOnce(t *testing.T) {
	svc, pool, userID := withTestUser(t)
	token, err := insertToken(t, pool, userID, "password_reset")
	require.NoError(t, err)
	_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
	require.NoError(t, err)

	_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: "another fine password"})

	assert.ErrorIs(t, err, ErrInvalidToken)
	_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: newPassword})
	assert.NoError(t, err)
}

func TestSetPasswordWithTokenRevokesOtherCredentials(t *testing.T) {
	svc, pool, userID := withTestUser(t)
	oldSession, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	otherReset, err := insertToken(t, pool, userID, "password_reset")
	require.NoError(t, err)
	invite, err := insertToken(t, pool, userID, "invite")
	require.NoError(t, err)
	token, err := insertToken(t, pool, userID, "password_reset")
	require.NoError(t, err)

	newSession, _, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
	require.NoError(t, err)

	sessions := NewSessions(pool)
	_, err = sessions.Authenticate(t.Context(), oldSession)
	assert.ErrorIs(t, err, ErrNoSession)
	_, err = sessions.Authenticate(t.Context(), newSession)
	assert.NoError(t, err)
	for _, other := range []string{otherReset, invite} {
		_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: other, Password: "another fine password"})
		assert.ErrorIs(t, err, ErrInvalidToken)
	}
	_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestSetPasswordWithTokenEndsTheSessionItReplaces(t *testing.T) {
	svc, pool, userID := withTestUser(t)
	var otherID uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email) VALUES ('other@example.com') RETURNING id`).Scan(&otherID))
	othersSession, err := NewSessions(pool).Create(t.Context(), otherID, "")
	require.NoError(t, err)
	token, err := insertToken(t, pool, userID, "invite")
	require.NoError(t, err)

	_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword, ReplacesToken: othersSession})

	require.NoError(t, err)
	_, err = NewSessions(pool).Authenticate(t.Context(), othersSession)
	assert.ErrorIs(t, err, ErrNoSession)
}

func TestSetPasswordWithTokenIsAllOrNothing(t *testing.T) {
	svc, pool, userID := withTestUser(t)
	_, err := insertToken(t, pool, userID, "invite")
	require.NoError(t, err)
	token, err := insertToken(t, pool, userID, "password_reset")
	require.NoError(t, err)
	_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	// The new session is the last write, so every earlier one has to roll back.
	_, err = pool.Exec(t.Context(), `
		CREATE FUNCTION refuse() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'refused'; END $$;
		CREATE TRIGGER refuse_sessions BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION refuse();`)
	require.NoError(t, err)
	before := passwordState(t, pool)

	_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})

	require.Error(t, err)
	assert.Equal(t, before, passwordState(t, pool))
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM sessions`))
	assert.Equal(t, 2, count(t, pool, `SELECT count(*) FROM auth_tokens WHERE consumed_at IS NULL`))
	_, err = pool.Exec(t.Context(), `DROP TRIGGER refuse_sessions ON sessions`)
	require.NoError(t, err)
	_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
	assert.NoError(t, err, "the link is still usable")
}

func TestSetPasswordWithTokenConcurrently(t *testing.T) {
	svc, pool, userID := withTestUser(t)
	token, err := insertToken(t, pool, userID, "password_reset")
	require.NoError(t, err)

	errs := make(chan error, 4)
	for range cap(errs) {
		go func() {
			_, _, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
			errs <- err
		}()
	}
	var succeeded int
	for range cap(errs) {
		if err := <-errs; err == nil {
			succeeded++
		} else {
			assert.ErrorIs(t, err, ErrInvalidToken)
		}
	}

	assert.Equal(t, 1, succeeded)
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM sessions`))
}

func TestSetPasswordWithTokenRefusedWhenAChangeWinsTheLock(t *testing.T) {
	tests := []struct {
		name   string
		change string
	}{
		{name: "token used meanwhile", change: `UPDATE auth_tokens SET consumed_at = now()`},
		{name: "disabled", change: `UPDATE users SET disabled_at = now()`},
		{name: "email change", change: `UPDATE users SET email = 'test.new@example.com'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			token, err := insertToken(t, pool, userID, "password_reset")
			require.NoError(t, err)
			change, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = change.Rollback(context.Background()) })
			_, err = change.Exec(t.Context(), `SELECT 1 FROM users FOR UPDATE; `+tt.change)
			require.NoError(t, err)
			before := passwordState(t, pool)

			done := make(chan error, 1)
			go func() {
				_, _, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
				done <- err
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE wait_event_type = 'Lock' AND query LIKE '-- name: LockUser %')`).Scan(&waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "waits on the change's row lock")
			require.NoError(t, change.Commit(t.Context()))

			assert.ErrorIs(t, <-done, ErrInvalidToken)
			assert.Equal(t, before, passwordState(t, pool))
			assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
		})
	}
}

func TestSetPasswordWithTokenIsBusyWhileARowStaysLocked(t *testing.T) {
	tests := []struct {
		name string
		hold string
	}{
		{name: "the user row", hold: `SELECT 1 FROM users FOR UPDATE`},
		{name: "the token row, past the user lock", hold: `SELECT 1 FROM auth_tokens FOR UPDATE`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			svc.lockTimeout = 100 * time.Millisecond
			token, err := insertToken(t, pool, userID, "password_reset")
			require.NoError(t, err)
			holder, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
			_, err = holder.Exec(t.Context(), tt.hold)
			require.NoError(t, err)

			_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})

			assert.ErrorIs(t, err, ErrBusy)
			require.NoError(t, holder.Rollback(t.Context()))
			_, _, err = svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: token, Password: newPassword})
			assert.NoError(t, err, "a busy refusal leaves the link usable")
		})
	}
}

// The stored hash and its change time, to show a refusal left the password alone.
func passwordState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(t.Context(),
		`SELECT coalesce(password_hash, '') || '|' || coalesce(password_changed_at::text, '') FROM users`).Scan(&state))
	return state
}

func insertToken(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, purpose string) (string, error) {
	t.Helper()
	raw, token := newToken()
	_, err := pool.Exec(t.Context(), `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '30 minutes')`,
		userID, purpose, hashToken(raw))
	return token, err
}
