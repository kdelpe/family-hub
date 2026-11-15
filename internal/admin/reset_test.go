package admin

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
)

const (
	notSetUp  = `UPDATE users SET password_hash = NULL, password_changed_at = NULL WHERE id = $1`
	oldInvite = `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, 'invite', 'old invite', now() + interval '7 days')`
)

func TestSendReset(t *testing.T) {
	tests := []struct {
		name        string
		setup       []string
		unknown     bool
		wantSend    bool
		sendErr     error
		wantErr     error
		wantSubject string
		// Usable links of each kind afterwards.
		wantResets, wantInvites int
	}{
		{name: "a password user gets a reset; the earlier one still works", wantSend: true, wantSubject: "Reset your family hub password", wantResets: 2},
		{name: "a user who never set up gets a new invite; the old one still works", setup: []string{notSetUp, oldInvite}, wantSend: true,
			wantSubject: inviteSubject, wantResets: 1, wantInvites: 1},
		// Saved first, so the admin can just send again.
		{name: "email not sent", wantSend: true, sendErr: errors.New("smtp: 550"), wantErr: ErrEmailNotSent, wantSubject: "Reset your family hub password", wantResets: 2},
		{name: "invite not sent", setup: []string{notSetUp, oldInvite}, wantSend: true, sendErr: errors.New("smtp: 550"), wantErr: ErrEmailNotSent,
			wantSubject: inviteSubject, wantResets: 1, wantInvites: 1},
		{name: "disabled", setup: []string{`UPDATE users SET disabled_at = now() WHERE id = $1`}, wantErr: ErrUserNotFound},
		{name: "unknown user", unknown: true, wantErr: ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, id := withAccounts(t)
			for _, sql := range tt.setup {
				_, err := pool.Exec(t.Context(), sql, id)
				require.NoError(t, err)
			}
			before := accountState(t, pool, id)
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantSend {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) { sent = msg }).Return(tt.sendErr).Once()
			}
			target := id
			if tt.unknown {
				target = uuid.New()
			}

			err := NewUsers(pool, mailer, testBaseURL).SendReset(t.Context(), target)

			if errors.Is(tt.wantErr, ErrUserNotFound) {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, accountState(t, pool, id), "nothing saved")
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, testEmail, sent.To)
			assert.Equal(t, tt.wantSubject, sent.Subject)
			assert.Equal(t, tt.wantResets, countFor(t, pool, `SELECT count(*) FROM auth_tokens WHERE user_id = $1 AND purpose = 'password_reset'`, id))
			assert.Equal(t, tt.wantInvites, countFor(t, pool, `SELECT count(*) FROM auth_tokens WHERE user_id = $1 AND purpose = 'invite' AND token_hash <> 'old invite'`, id))
			wantOld := 0
			if slices.Contains(tt.setup, oldInvite) {
				wantOld = 1
			}
			assert.Equal(t, wantOld, countFor(t, pool, `SELECT count(*) FROM auth_tokens WHERE user_id = $1 AND token_hash = 'old invite'`, id), "earlier links stay valid")
			assert.Equal(t, 1, countFor(t, pool, `SELECT count(*) FROM sessions WHERE user_id = $1`, id), "nobody is signed out")
			_, user, err := auth.NewService(pool, auth.NewHasher()).SetPasswordWithToken(t.Context(), auth.SetPasswordInput{Token: inviteToken(t, sent.Body), Password: testPassword})
			require.NoError(t, err, "the emailed link sets a password")
			assert.Equal(t, id, user.ID)
		})
	}
}
