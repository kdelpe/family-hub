package admin

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/admin/mocks"
	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const (
	testEmail    = "test@example.com"
	testPassword = "correct horse battery"
)

var prompts = []string{"Password: ", "Password again: "}

func TestRunCreateRefusesBeforeTheDatabase(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		isTerminal *bool
		wantErr    error
	}{
		{name: "no email", args: nil, wantErr: ErrCreateUsage},
		{name: "empty email", args: []string{"--email", ""}, wantErr: ErrCreateUsage},
		{name: "extra argument", args: []string{"--email", testEmail, "extra"}, wantErr: ErrCreateUsage},
		{name: "unknown flag", args: []string{"--email", testEmail, "--role", "member"}, wantErr: ErrCreateUsage},
		{name: "invalid email", args: []string{"--email", "Test <test@example.com>"}, wantErr: ErrInvalidEmail},
		{name: "not a terminal", args: []string{"--email", testEmail}, isTerminal: new(false), wantErr: ErrNoTerminal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := mocks.NewTerminal(t)
			if tt.isTerminal != nil {
				term.EXPECT().IsTerminal().Return(*tt.isTerminal).Once()
			}
			var stdout bytes.Buffer

			err := RunCreate(t.Context(), tt.args, func(string) string { return "" }, term, &stdout)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, stdout.String())
		})
	}
}

func TestRunCreate(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		existing    string
		unreachable bool
		typed       []string
		readErr     error
		wantErr     error
	}{
		{name: "creates the admin", email: testEmail, typed: []string{testPassword, testPassword}},
		{name: "normalizes the email", email: "  Test@Example.COM ", typed: []string{testPassword, testPassword}},
		{name: "same password in another Unicode form", email: testEmail, typed: []string{testPassword + " café", testPassword + " café"}},
		{name: "database unreachable", email: testEmail, unreachable: true},
		{name: "password too short", email: testEmail, typed: []string{"short"}, wantErr: auth.ErrPasswordTooShort},
		{name: "password is the email", email: testEmail, typed: []string{testEmail}, wantErr: auth.ErrPasswordTooCommon},
		{name: "passwords differ", email: testEmail, typed: []string{testPassword, "correct horse battery!"}, wantErr: ErrPasswordMismatch},
		{name: "read canceled", email: testEmail, typed: []string{""}, readErr: context.Canceled, wantErr: context.Canceled},
		{name: "email taken", email: "TEST@example.com", existing: testEmail, wantErr: ErrEmailTaken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			if tt.existing != "" {
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email) VALUES ($1)`, tt.existing)
				require.NoError(t, err)
			}
			url := pool.Config().ConnString()
			if tt.unreachable {
				url = testdb.UnreachableURL
			}
			getenv := func(name string) string {
				if name == "DATABASE_URL" {
					return url
				}
				return ""
			}
			term := mocks.NewTerminal(t)
			term.EXPECT().IsTerminal().Return(true).Once()
			for i, typed := range tt.typed {
				var err error
				if i == 0 {
					err = tt.readErr
				}
				term.EXPECT().ReadPassword(mock.Anything, prompts[i]).Return(typed, err).Once()
			}
			var stdout bytes.Buffer

			err := RunCreate(t.Context(), []string{"--email", tt.email}, getenv, term, &stdout)

			wantUsers := 0
			if tt.existing != "" {
				wantUsers = 1
			}
			switch {
			case tt.unreachable:
				require.ErrorContains(t, err, "database:")
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			default:
				require.NoError(t, err)
				assert.Equal(t, "Created admin "+testEmail+"\n", stdout.String())
				var role, hash string
				require.NoError(t, pool.QueryRow(t.Context(),
					`SELECT role, password_hash FROM users WHERE email = $1 AND password_changed_at IS NOT NULL`, testEmail).Scan(&role, &hash))
				assert.Equal(t, "admin", role)
				ok, _, err := auth.NewHasher().Verify(t.Context(), hash, tt.typed[0])
				require.NoError(t, err)
				assert.True(t, ok)
				wantUsers = 1
			}
			if err != nil {
				assert.Empty(t, stdout.String())
			}
			var users int
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&users))
			assert.Equal(t, wantUsers, users)
		})
	}
}

func TestRunCreateHelpPrintsUsage(t *testing.T) {
	var stdout bytes.Buffer

	err := RunCreate(t.Context(), []string{"-h"}, func(string) string { return "" }, mocks.NewTerminal(t), &stdout)

	require.NoError(t, err)
	assert.Equal(t, ErrCreateUsage.Error()+"\n", stdout.String())
}

func TestCreateAdminRefusesATakenEmailOnInsert(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	q := gen.New(pool)
	require.NoError(t, createAdmin(t.Context(), q, testEmail, "hash"))

	err := createAdmin(t.Context(), q, testEmail, "hash")

	assert.ErrorIs(t, err, ErrEmailTaken)
}
