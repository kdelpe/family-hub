package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const (
	otherAdminEmail = "admin@example.com"
	// This test's user becomes the only enabled admin.
	onlyAdmin = `UPDATE users SET role = 'admin', disabled_at = CASE WHEN id = $1 THEN NULL ELSE now() END`
	// The other admin never set up, so this test's user is the only admin who can sign in.
	invitedOtherAdmin = `UPDATE users SET role = 'admin',
		password_hash = CASE WHEN id = $1 THEN password_hash END, password_changed_at = CASE WHEN id = $1 THEN password_changed_at END`
)

func TestUpdate(t *testing.T) {
	ptr := func(s string) *string { return &s }
	yes, no := true, false

	tests := []struct {
		name           string
		setup          string
		in             UpdateInput
		wantInviteSent bool
		sendErr        error
		wantErr        error
		wantEmail      string
		wantRole       string
		wantStatus     string
		wantRevoked    bool
	}{
		{name: "made an admin", in: UpdateInput{Role: ptr(auth.RoleAdmin)}, wantEmail: testEmail, wantRole: auth.RoleAdmin, wantStatus: StatusActive},
		{name: "disabled", in: UpdateInput{Disabled: &yes}, wantEmail: testEmail, wantRole: auth.RoleMember, wantStatus: StatusDisabled, wantRevoked: true},
		{name: "enabled again", setup: `UPDATE users SET disabled_at = now() WHERE id = $1`, in: UpdateInput{Disabled: &no},
			wantEmail: testEmail, wantRole: auth.RoleMember, wantStatus: StatusActive},
		{name: "a new email starts over with an invite", in: UpdateInput{Email: ptr(" New@Example.COM ")}, wantInviteSent: true,
			wantEmail: "new@example.com", wantRole: auth.RoleMember, wantStatus: StatusInvited, wantRevoked: true},
		{name: "a new email while enabling", setup: `UPDATE users SET disabled_at = now() WHERE id = $1`, in: UpdateInput{Email: ptr("new@example.com"), Disabled: &no}, wantInviteSent: true,
			wantEmail: "new@example.com", wantRole: auth.RoleMember, wantStatus: StatusInvited, wantRevoked: true},
		{name: "the same email isn't a change", in: UpdateInput{Email: ptr("TEST@example.com"), Disabled: &yes},
			wantEmail: testEmail, wantRole: auth.RoleMember, wantStatus: StatusDisabled, wantRevoked: true},
		{name: "a disabled user's email", setup: `UPDATE users SET disabled_at = now() WHERE id = $1`, in: UpdateInput{Email: ptr("new@example.com")}, wantErr: ErrDisabledEmailChange},
		{name: "a new email while disabling", in: UpdateInput{Email: ptr("new@example.com"), Disabled: &yes}, wantErr: ErrDisabledEmailChange},
		{name: "email taken", in: UpdateInput{Email: ptr(otherAdminEmail)}, wantErr: ErrEmailTaken},
		{name: "invite not sent changes nothing", in: UpdateInput{Email: ptr("new@example.com")}, wantInviteSent: true, sendErr: errors.New("smtp: 550"), wantErr: ErrEmailNotSent},
		{name: "invalid email", in: UpdateInput{Email: ptr("Test <new@example.com>")}, wantErr: ErrInvalidEmail},
		{name: "unknown role", in: UpdateInput{Role: ptr("owner")}, wantErr: ErrInvalidRole},
		{name: "nothing to change", wantErr: ErrNoChanges},
		{name: "demoting another admin", setup: `UPDATE users SET role = 'admin' WHERE id = $1`, in: UpdateInput{Role: ptr(auth.RoleMember)},
			wantEmail: testEmail, wantRole: auth.RoleMember, wantStatus: StatusActive},
		{name: "the last admin can't be demoted", setup: onlyAdmin,
			in: UpdateInput{Role: ptr(auth.RoleMember)}, wantErr: ErrLastAdmin},
		{name: "the last admin can't be disabled", setup: onlyAdmin,
			in: UpdateInput{Disabled: &yes}, wantErr: ErrLastAdmin},
		{name: "the last admin can't get a new email", setup: onlyAdmin, in: UpdateInput{Email: ptr("new@example.com")}, wantErr: ErrLastAdmin},
		{name: "an admin who never set up doesn't count", setup: invitedOtherAdmin, in: UpdateInput{Role: ptr(auth.RoleMember)}, wantErr: ErrLastAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, id := withAccounts(t)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup, id)
				require.NoError(t, err)
			}
			before := accountState(t, pool, id)
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantInviteSent {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) { sent = msg }).Return(tt.sendErr).Once()
			}
			tt.in.ID = id

			got, err := NewUsers(pool, mailer, testBaseURL).Update(t.Context(), tt.in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, accountState(t, pool, id), "nothing changed")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, id, got.ID)
			assert.Equal(t, tt.wantEmail, got.Email)
			assert.Equal(t, tt.wantRole, got.Role)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, got, listed(t, pool, id), "the same as the list shows")
			sessions, resets := countFor(t, pool, `SELECT count(*) FROM sessions WHERE user_id = $1`, id), countFor(t, pool, `SELECT count(*) FROM auth_tokens WHERE user_id = $1 AND purpose = 'password_reset'`, id)
			if tt.wantRevoked {
				assert.Zero(t, sessions, "signed out")
				assert.Zero(t, resets, "links voided")
			} else {
				assert.Equal(t, 1, sessions)
				assert.Equal(t, 1, resets)
			}
			if !tt.wantInviteSent {
				return
			}
			assert.Equal(t, tt.wantEmail, sent.To)
			assert.Equal(t, inviteSubject, sent.Subject)
			require.NotNil(t, got.InviteExpiresAt)
			assert.Equal(t, "|", credentials(t, pool, id), "the old password is gone")
			_, user, err := auth.NewService(pool, auth.NewHasher()).SetPasswordWithToken(t.Context(), auth.SetPasswordInput{Token: inviteToken(t, sent.Body), Password: testPassword})
			require.NoError(t, err, "the new address's invite sets a password")
			assert.Equal(t, tt.wantEmail, user.Email)
		})
	}
}

func TestUpdateKeepsAnAdminWhenTwoDemoteEachOther(t *testing.T) {
	pool, id := withAccounts(t)
	_, err := pool.Exec(t.Context(), `UPDATE users SET role = 'admin' WHERE id = $1`, id)
	require.NoError(t, err)
	other := userID(t, pool, otherAdminEmail)
	member := auth.RoleMember
	users := NewUsers(pool, nil, testBaseURL)

	errs := make(chan error, 2)
	for _, target := range []uuid.UUID{id, other} {
		go func() {
			_, err := users.Update(t.Context(), UpdateInput{ID: target, Role: &member})
			errs <- err
		}()
	}
	var refused int
	for range 2 {
		if err := <-errs; err != nil {
			require.ErrorIs(t, err, ErrLastAdmin)
			refused++
		}
	}

	assert.Equal(t, 1, refused)
	var admins int
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&admins))
	assert.Equal(t, 1, admins)
}

func TestUpdateUnknownUser(t *testing.T) {
	pool, _ := withAccounts(t)
	member := auth.RoleMember

	_, err := NewUsers(pool, nil, testBaseURL).Update(t.Context(), UpdateInput{ID: uuid.New(), Role: &member})

	assert.ErrorIs(t, err, ErrUserNotFound)
}

// withAccounts adds another enabled admin and a member with a password, a session and a reset link.
func withAccounts(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	_, err := pool.Exec(t.Context(), `INSERT INTO users (email, role, password_hash, password_changed_at) VALUES ($1, 'admin', 'hash', now())`, otherAdminEmail)
	require.NoError(t, err)
	id := insertUser(t, pool, testEmail)
	for _, sql := range []string{
		`UPDATE users SET password_hash = 'hash', password_changed_at = now() WHERE id = $1`,
		`INSERT INTO sessions (id_hash, user_id, expires_at) VALUES (sha256($1::uuid::text::bytea), $1::uuid, now() + interval '1 day')`,
		`INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1::uuid, 'password_reset', sha256($1::uuid::text::bytea), now() + interval '30 minutes')`,
	} {
		_, err := pool.Exec(t.Context(), sql, id)
		require.NoError(t, err)
	}
	return pool, id
}

func accountState(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT concat_ws('|', u.email, u.role, u.disabled_at, u.password_hash,
		(SELECT count(*) FROM sessions WHERE user_id = u.id), (SELECT count(*) FROM auth_tokens WHERE user_id = u.id))
		FROM users u WHERE id = $1`, id).Scan(&state))
	return state
}

func credentials(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var creds string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT concat(password_hash, '|', password_changed_at) FROM users WHERE id = $1`, id).Scan(&creds))
	return creds
}

func listed(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) Account {
	t.Helper()
	page, err := NewUsers(pool, nil, testBaseURL).List(t.Context(), ListInput{Limit: DefaultPageSize})
	require.NoError(t, err)
	for _, a := range page.Accounts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("user %s not listed", id)
	return Account{}
}

func userID(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT id FROM users WHERE email = $1`, email).Scan(&id))
	return id
}

func countFor(t *testing.T, pool *pgxpool.Pool, sql string, id uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(t.Context(), sql, id).Scan(&n))
	return n
}
