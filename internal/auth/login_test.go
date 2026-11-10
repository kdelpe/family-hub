package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/ratelimit"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const (
	testEmail    = "test@example.com"
	testPassword = "correct horse battery"
)

func TestLogin(t *testing.T) {
	tests := []struct {
		name     string
		setup    string
		email    string
		password string
		wantOK   bool
	}{
		{name: "right password", email: testEmail, password: testPassword, wantOK: true},
		{name: "email in another case with spaces", email: "  Test@Example.COM ", password: testPassword, wantOK: true},
		{name: "wrong password", email: testEmail, password: "wrong horse battery"},
		{name: "unknown email", email: "other@example.com", password: testPassword},
		{name: "no password (Google only)", setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL`, email: testEmail, password: testPassword},
		{name: "disabled", setup: `UPDATE users SET disabled_at = now()`, email: testEmail, password: testPassword},
		{name: "email longer than any real address", email: strings.Repeat("a", 250) + "@example.com", password: testPassword},
		{name: "email with a NUL byte", email: "test@example.com\x00", password: testPassword},
		{name: "broken stored hash", setup: `UPDATE users SET password_hash = 'nonsense'`, email: testEmail, password: testPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}

			token, user, err := svc.Login(t.Context(), LoginInput{Email: tt.email, Password: tt.password, UserAgent: "test"})

			if !tt.wantOK {
				require.ErrorIs(t, err, ErrInvalidCredentials)
				assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, User{ID: userID, Email: testEmail, Role: "member"}, user)
			session, err := NewSessions(pool).Authenticate(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t, userID, session.UserID)
			assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM users WHERE last_login_at IS NOT NULL`))
		})
	}
}

func TestLoginRehashesOutdatedParameters(t *testing.T) {
	svc, pool, _ := withTestUser(t)
	old, err := svc.hasher.hash(t.Context(), testPassword, params{memory: 8 * 1024, time: 1, threads: 1})
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE users SET password_hash = $1`, old)
	require.NoError(t, err)

	_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	require.NoError(t, err)

	var stored string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT password_hash FROM users`).Scan(&stored))
	assert.NotEqual(t, old, stored)
	ok, rehash, err := svc.hasher.Verify(t.Context(), stored, testPassword)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, rehash)
}

func TestLoginRefusedWhenACredentialChangesBeforeTheSessionCommits(t *testing.T) {
	tests := []struct {
		name   string
		change string
		wantOK bool
	}{
		{name: "password reset", change: `UPDATE users SET password_hash = password_hash || 'x', password_changed_at = now()`},
		{name: "email change", change: `UPDATE users SET email = 'test.new@example.com'`},
		{name: "disabled", change: `UPDATE users SET disabled_at = now()`},
		{name: "another login's rehash is not a change", change: `UPDATE users SET password_hash = password_hash || 'x'`, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, _ := withTestUser(t)
			svc.failuresByEmailAndIP = ratelimit.New(1, time.Hour)
			change, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = change.Rollback(context.Background()) })
			_, err = change.Exec(t.Context(), tt.change)
			require.NoError(t, err)

			done := make(chan error, 1)
			go func() {
				_, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
				done <- err
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE wait_event_type = 'Lock' AND query LIKE '-- name: LockUserForSignIn %')`).Scan(&waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "login waits on the change's row lock")
			require.NoError(t, change.Commit(t.Context()))

			err = <-done
			if tt.wantOK {
				require.NoError(t, err)
				assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM sessions`))
				return
			}
			assert.ErrorIs(t, err, ErrInvalidCredentials)
			assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
			ok, _ := svc.failuresByEmailAndIP.Take(testEmail + "\x00")
			assert.True(t, ok, "the right password isn't counted as a failed guess")
		})
	}
}

func TestParallelGuessesCantOutrunTheLimit(t *testing.T) {
	svc, _, _ := withTestUser(t)
	svc.failuresByEmailAndIP = ratelimit.New(2, time.Hour)
	errs := make(chan error, 6)
	for range 6 {
		go func() {
			_, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: "wrong", ClientIP: "203.0.113.7"})
			errs <- err
		}()
	}

	var guesses, limited int
	for range 6 {
		var rl *RateLimitedError
		switch err := <-errs; {
		case errors.Is(err, ErrInvalidCredentials):
			guesses++
		case errors.As(err, &rl):
			limited++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	assert.Equal(t, 2, guesses)
	assert.Equal(t, 4, limited)
}

func TestLoginFailureLimits(t *testing.T) {
	tests := []struct {
		name        string
		failuresFor func(i int) LoginInput
		thenTry     LoginInput
		wantBlocked bool
	}{
		{
			name:        "same email and IP is blocked, even with the right password",
			failuresFor: func(int) LoginInput { return LoginInput{Email: testEmail, Password: "wrong", ClientIP: "203.0.113.7"} },
			thenTry:     LoginInput{Email: testEmail, Password: testPassword, ClientIP: "203.0.113.7"},
			wantBlocked: true,
		},
		{
			name:        "another IP still signs in below the backstop",
			failuresFor: func(int) LoginInput { return LoginInput{Email: testEmail, Password: "wrong", ClientIP: "203.0.113.7"} },
			thenTry:     LoginInput{Email: testEmail, Password: testPassword, ClientIP: "198.51.100.1"},
		},
		{
			name: "the per-email backstop spans IPs",
			failuresFor: func(i int) LoginInput {
				return LoginInput{Email: testEmail, Password: "wrong", ClientIP: fmt.Sprintf("203.0.113.%d", i)}
			},
			thenTry:     LoginInput{Email: testEmail, Password: testPassword, ClientIP: "198.51.100.1"},
			wantBlocked: true,
		},
		{
			name: "unknown emails count too, so a block says nothing about accounts",
			failuresFor: func(int) LoginInput {
				return LoginInput{Email: "other@example.com", Password: "wrong", ClientIP: "203.0.113.7"}
			},
			thenTry:     LoginInput{Email: "other@example.com", Password: "wrong", ClientIP: "203.0.113.7"},
			wantBlocked: true,
		},
		{
			name: "successful sign-ins don't count",
			failuresFor: func(int) LoginInput {
				return LoginInput{Email: testEmail, Password: testPassword, ClientIP: "203.0.113.7"}
			},
			thenTry: LoginInput{Email: testEmail, Password: testPassword, ClientIP: "203.0.113.7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := withTestUser(t)
			svc.failuresByEmailAndIP = ratelimit.New(2, time.Hour)
			svc.failuresByEmail = ratelimit.New(3, time.Hour)
			for i := range 3 {
				_, _, _ = svc.Login(t.Context(), tt.failuresFor(i))
			}

			_, _, err := svc.Login(t.Context(), tt.thenTry)

			var limited *RateLimitedError
			if !tt.wantBlocked {
				assert.NotErrorAs(t, err, &limited)
				return
			}
			require.ErrorAs(t, err, &limited)
			assert.Positive(t, limited.RetryAfter)
		})
	}
}

func TestLoginIsBusyWhileTheUserRowStaysLocked(t *testing.T) {
	svc, pool, _ := withTestUser(t)
	svc.lockTimeout = 100 * time.Millisecond
	holder, err := pool.Begin(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
	_, err = holder.Exec(t.Context(), `SELECT 1 FROM users FOR UPDATE`)
	require.NoError(t, err)

	_, _, err = svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})

	assert.ErrorIs(t, err, ErrBusy)
	assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
}

func TestUserOfADeletedAccountIsNoSession(t *testing.T) {
	svc, pool, id := withTestUser(t)
	_, err := pool.Exec(t.Context(), `DELETE FROM users`)
	require.NoError(t, err)

	_, err = svc.User(t.Context(), id)

	assert.ErrorIs(t, err, ErrNoSession)
}

func TestLoginEndsTheSessionItReplaces(t *testing.T) {
	svc, pool, _ := withTestUser(t)
	old, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	require.NoError(t, err)

	token, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword, ReplacesToken: old})
	require.NoError(t, err)

	sessions := NewSessions(pool)
	_, err = sessions.Authenticate(t.Context(), old)
	assert.ErrorIs(t, err, ErrNoSession)
	_, err = sessions.Authenticate(t.Context(), token)
	assert.NoError(t, err)
}

func TestLogout(t *testing.T) {
	svc, pool, _ := withTestUser(t)
	token, _, err := svc.Login(t.Context(), LoginInput{Email: testEmail, Password: testPassword})
	require.NoError(t, err)

	require.NoError(t, svc.Logout(t.Context(), token))

	assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
}

func TestValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "plain address", email: testEmail, want: true},
		{name: "subaddress", email: "ana+family@example.com", want: true},
		{name: "longest allowed", email: strings.Repeat("a", 64) + "@" + strings.Repeat("b", 185) + ".com", want: true},
		{name: "local part too long", email: strings.Repeat("a", 65) + "@example.com"},
		{name: "one too long", email: strings.Repeat("a", 64) + "@" + strings.Repeat("b", 186) + ".com"},
		{name: "empty", email: ""},
		{name: "no at sign", email: "test.example.com"},
		{name: "no local part", email: "@example.com"},
		{name: "no domain", email: "test@"},
		{name: "display name", email: "Test <test@example.com>"},
		{name: "two addresses", email: "test@example.com, other@example.com"},
		{name: "inner space", email: "ana @example.com"},
		{name: "NUL byte", email: "test@example.com\x00"},
		{name: "dotless domain", email: "test@localhost"},
		{name: "IP literal", email: "test@[192.0.2.1]"},
		{name: "quoted local part", email: `"test@x"@example.com`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidEmail(tt.email))
		})
	}
}

func withTestUser(t *testing.T) (*Service, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	hasher := NewHasher()
	hash, err := hasher.Hash(t.Context(), testPassword)
	require.NoError(t, err)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(),
		`INSERT INTO users (email, password_hash, password_changed_at) VALUES ($1, $2, now()) RETURNING id`, testEmail, hash).Scan(&id))
	return NewService(pool, hasher), pool, id
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(t.Context(), sql).Scan(&n))
	return n
}
