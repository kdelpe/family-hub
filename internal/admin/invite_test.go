package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const testBaseURL = "https://family.example.com"

func TestInvite(t *testing.T) {
	tests := []struct {
		name             string
		in               InviteInput
		existing         string
		sendErr          error
		wantSend         bool
		wantErr          error
		cancelDuringSend bool
		wantUser         auth.User
	}{
		{name: "a member", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "an admin", in: InviteInput{Email: testEmail, Role: auth.RoleAdmin}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleAdmin}},
		{name: "normalizes the email", in: InviteInput{Email: "  Test@Example.COM ", Role: auth.RoleMember}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "sent and committed even if the admin leaves during the send", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, cancelDuringSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "invalid email", in: InviteInput{Email: "Test <test@example.com>", Role: auth.RoleMember}, wantErr: ErrInvalidEmail},
		{name: "unknown role", in: InviteInput{Email: testEmail, Role: "owner"}, wantErr: ErrInvalidRole},
		{name: "no role", in: InviteInput{Email: testEmail}, wantErr: ErrInvalidRole},
		{name: "email taken", in: InviteInput{Email: "TEST@example.com", Role: auth.RoleMember}, existing: testEmail, wantErr: ErrEmailTaken},
		{name: "email not sent saves nothing", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, sendErr: errors.New("smtp: 550"), wantErr: ErrInviteNotSent},
		{name: "a refused message is not a send failure", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, sendErr: fmt.Errorf("%w: subject", mail.ErrInvalidMessage), wantErr: mail.ErrInvalidMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			if tt.existing != "" {
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email) VALUES ($1)`, tt.existing)
				require.NoError(t, err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantSend {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(sendCtx context.Context, msg mail.Message) {
					sent = msg
					if tt.cancelDuringSend {
						cancel()
						assert.NoError(t, sendCtx.Err(), "the send isn't cut short")
					}
				}).Return(tt.sendErr).Once()
			}

			user, err := NewInvites(pool, mailer, testBaseURL).Invite(ctx, tt.in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				if errors.Is(tt.wantErr, mail.ErrInvalidMessage) {
					assert.NotErrorIs(t, err, ErrInviteNotSent)
				}
				var newUsers, tokens int
				require.NoError(t, pool.QueryRow(t.Context(),
					`SELECT (SELECT count(*) FROM users WHERE email <> $1), (SELECT count(*) FROM auth_tokens)`, tt.existing).Scan(&newUsers, &tokens))
				assert.Zero(t, newUsers, "no user left behind")
				assert.Zero(t, tokens)
				return
			}
			require.NoError(t, err)
			assert.NotZero(t, user.ID)
			tt.wantUser.ID = user.ID
			assert.Equal(t, tt.wantUser, user)
			assert.Equal(t, testEmail, sent.To)
			assert.Equal(t, "You're invited to the family hub", sent.Subject)
			assert.Contains(t, sent.Body, "This link works for 7 days.")
			token := inviteToken(t, sent.Body)

			_, signedIn, err := auth.NewService(pool, auth.NewHasher()).SetPasswordWithToken(t.Context(), auth.SetPasswordInput{Token: token, Password: testPassword})

			require.NoError(t, err, "the emailed link sets the password")
			assert.Equal(t, user, signedIn)
		})
	}
}

func inviteToken(t *testing.T, body string) string {
	t.Helper()
	prefix := testBaseURL + setPasswordPath
	for line := range strings.Lines(body) {
		if token, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return token
		}
	}
	t.Fatalf("no %s link in the email:\n%s", prefix, body)
	return ""
}
