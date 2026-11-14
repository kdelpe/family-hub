package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangePassword(t *testing.T) {
	tests := []struct {
		name        string
		setup       string
		noToken     bool
		current     string
		newPassword string
		wantErr     error
		// Sessions left when it's refused: this test's two, unless the setup removed one.
		wantSessions int
	}{
		{name: "changed", current: testPassword, newPassword: newPassword},
		{name: "wrong current password", current: "not the password", newPassword: newPassword, wantErr: ErrInvalidCredentials},
		{name: "no password yet (Google only)", setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL, google_sub = 'sub-1'`,
			current: testPassword, newPassword: newPassword, wantErr: ErrInvalidCredentials},
		{name: "new password too short", current: testPassword, newPassword: "short", wantErr: ErrPasswordTooShort},
		{name: "new password is the email", current: testPassword, newPassword: testEmail, wantErr: ErrPasswordTooCommon},
		{name: "disabled meanwhile", setup: `UPDATE users SET disabled_at = now()`, current: testPassword, newPassword: newPassword, wantErr: ErrNoSession},
		{name: "this session signed out meanwhile", setup: `DELETE FROM sessions WHERE user_agent = 'this device'`,
			current: testPassword, newPassword: newPassword, wantErr: ErrNoSession, wantSessions: 1},
		{name: "no session token", noToken: true, current: testPassword, newPassword: newPassword, wantErr: ErrNoSession},
		{name: "this session expired meanwhile", setup: `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE user_agent = 'this device'`,
			current: testPassword, newPassword: newPassword, wantErr: ErrNoSession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			sessions := NewSessions(pool)
			this, err := sessions.Create(t.Context(), userID, "this device")
			require.NoError(t, err)
			_, err = sessions.Create(t.Context(), userID, "another device")
			require.NoError(t, err)
			_, err = insertToken(t, pool, userID, purposePasswordReset)
			require.NoError(t, err)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}
			before := passwordState(t, pool)
			token := this
			if tt.noToken {
				token = ""
			}

			fresh, err := svc.ChangePassword(t.Context(), ChangePasswordInput{UserID: userID, SessionToken: token, CurrentPassword: tt.current, NewPassword: tt.newPassword})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, passwordState(t, pool))
				wantSessions := 2
				if tt.wantSessions != 0 {
					wantSessions = tt.wantSessions
				}
				assert.Equal(t, wantSessions, count(t, pool, `SELECT count(*) FROM sessions`))
				assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM auth_tokens`))
				return
			}
			require.NoError(t, err)
			_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: newPassword, ClientIP: "192.0.2.1"})
			assert.NoError(t, err, "the new password signs in")
			_, err = sessions.Authenticate(t.Context(), this)
			assert.ErrorIs(t, err, ErrNoSession, "the old token is out, even on this device, in case it was copied")
			got, err := sessions.Authenticate(t.Context(), fresh)
			require.NoError(t, err, "this device carries on with the new session")
			assert.Equal(t, userID, got.UserID)
			assert.Equal(t, 2, count(t, pool, `SELECT count(*) FROM sessions`), "the new one and the login's; the other device is out")
			assert.Zero(t, count(t, pool, `SELECT count(*) FROM auth_tokens`), "emailed links stop working")
		})
	}
}

func TestChangePasswordLimitsWrongAttemptsPerSession(t *testing.T) {
	tests := []struct {
		name         string
		wrong        int
		otherSession bool
		wantErr      bool
	}{
		{name: "under the limit", wrong: passwordChangeFailuresPerSession - 1},
		{name: "over the limit, even the right password waits", wrong: passwordChangeFailuresPerSession, wantErr: true},
		{name: "another session of the same user isn't blocked", wrong: passwordChangeFailuresPerSession, otherSession: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			session, err := NewSessions(pool).Create(t.Context(), userID, "test")
			require.NoError(t, err)
			// A refused new password isn't a wrong guess, so it costs nothing.
			_, err = svc.ChangePassword(t.Context(), ChangePasswordInput{UserID: userID, SessionToken: session, CurrentPassword: testPassword, NewPassword: "short"})
			require.ErrorIs(t, err, ErrPasswordTooShort)
			for range tt.wrong {
				_, err := svc.ChangePassword(t.Context(), ChangePasswordInput{UserID: userID, SessionToken: session, CurrentPassword: "wrong", NewPassword: newPassword})
				require.ErrorIs(t, err, ErrInvalidCredentials)
			}

			if tt.otherSession {
				session, err = NewSessions(pool).Create(t.Context(), userID, "the owner's phone")
				require.NoError(t, err)
			}

			_, err = svc.ChangePassword(t.Context(), ChangePasswordInput{UserID: userID, SessionToken: session, CurrentPassword: testPassword, NewPassword: newPassword})

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			var limited *RateLimitedError
			assert.ErrorAs(t, err, &limited)
		})
	}
}

func TestChangePasswordRefusedWhenAChangeWinsTheLock(t *testing.T) {
	tests := []struct {
		name   string
		change string
	}{
		{name: "password reset meanwhile", change: `UPDATE users SET password_changed_at = clock_timestamp()`},
		{name: "email changed", change: `UPDATE users SET email = 'test.new@example.com'`},
		{name: "disabled", change: `UPDATE users SET disabled_at = now()`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			session, err := NewSessions(pool).Create(t.Context(), userID, "test")
			require.NoError(t, err)
			change, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = change.Rollback(context.Background()) })
			_, err = change.Exec(t.Context(), `SELECT 1 FROM users FOR UPDATE; `+tt.change)
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() {
				_, err := svc.ChangePassword(t.Context(), ChangePasswordInput{UserID: userID, SessionToken: session, CurrentPassword: testPassword, NewPassword: newPassword})
				done <- err
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE wait_event_type = 'Lock' AND query LIKE '-- name: LockUser %')`).Scan(&waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "waits on the change's row lock")
			require.NoError(t, change.Commit(t.Context()))

			assert.ErrorIs(t, <-done, ErrNoSession, "sign in again: the change that won revokes sessions")
			ok, _, err := NewHasher().Verify(t.Context(), storedHash(t, pool), newPassword)
			require.NoError(t, err)
			assert.False(t, ok, "the new password wasn't set")
		})
	}
}

func storedHash(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var hash string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT password_hash FROM users`).Scan(&hash))
	return hash
}
