package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
)

const testBaseURL = "https://family.example.com"

func TestResetsRequest(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		setup    string
		sendErr  error
		wantSend bool
	}{
		{name: "an enabled user gets a link", email: testEmail, wantSend: true},
		{name: "the email is normalized", email: "  TEST@Example.com ", wantSend: true},
		{name: "a user without a password can add one", email: testEmail, setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL, google_sub = 'sub-1'`, wantSend: true},
		{name: "unknown email", email: "nobody@example.com"},
		{name: "disabled user", email: testEmail, setup: `UPDATE users SET disabled_at = now()`},
		{name: "not an email", email: "Test <test@example.com>"},
		{name: "a failed send keeps the token", email: testEmail, sendErr: errors.New("smtp: 550"), wantSend: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, userID := withTestUser(t)
			if tt.setup != "" {
				_, err := pool.Exec(t.Context(), tt.setup)
				require.NoError(t, err)
			}
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantSend {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) { sent = msg }).Return(tt.sendErr).Once()
			}
			resets := NewResets(svc, mailer, testBaseURL)
			ctx, cancel := context.WithCancel(t.Context())

			resets.Request(ctx, tt.email)
			cancel()
			resets.sending.Wait()

			if !tt.wantSend {
				assert.Zero(t, count(t, pool, `SELECT count(*) FROM auth_tokens`))
				return
			}
			assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM auth_tokens WHERE purpose = 'password_reset'
				AND expires_at BETWEEN now() + interval '29 minutes' AND now() + interval '30 minutes'`))
			assert.Equal(t, testEmail, sent.To)
			assert.Equal(t, "Reset your family hub password", sent.Subject)
			assert.Contains(t, sent.Body, "It works for 30 minutes.")
			_, user, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: linkToken(t, sent.Body), Password: newPassword})
			require.NoError(t, err, "the emailed link sets the password")
			assert.Equal(t, userID, user.ID)
		})
	}
}

func TestResetsRequestKeepsEarlierLinksAndLimitsPerEmail(t *testing.T) {
	svc, pool, _ := withTestUser(t)
	mailer := mailmocks.NewMailer(t)
	var bodies []string
	mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) {
		bodies = append(bodies, msg.Body)
	}).Return(nil).Times(resetsPerEmail)
	resets := NewResets(svc, mailer, testBaseURL)

	for range resetsPerEmail + 1 {
		resets.Request(t.Context(), testEmail)
		resets.sending.Wait()
	}

	assert.Equal(t, resetsPerEmail, count(t, pool, `SELECT count(*) FROM auth_tokens`), "the request over the limit sends nothing")
	require.Len(t, bodies, resetsPerEmail)
	_, _, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: linkToken(t, bodies[0]), Password: newPassword})
	assert.NoError(t, err, "a newer request doesn't cancel the first link")
}

func linkToken(t *testing.T, body string) string {
	t.Helper()
	prefix := testBaseURL + SetPasswordPath
	for line := range strings.Lines(body) {
		if token, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return token
		}
	}
	t.Fatalf("no %s link in the email:\n%s", prefix, body)
	return ""
}

func TestResetsLimitWhatRunsAtOnce(t *testing.T) {
	tests := []struct {
		name string
		// Sends that are still running when the next request comes in.
		busy        int
		closed      bool
		wantSends   int
		wantStarted int
	}{
		{name: "under the cap starts at once", busy: maxResetsSending - 1, wantSends: maxResetsSending, wantStarted: maxResetsSending},
		{name: "over the cap waits for a slot", busy: maxResetsSending, wantSends: maxResetsSending + 1, wantStarted: maxResetsSending},
		{name: "after Close nothing starts", closed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, _ := withTestUser(t)
			for i := range maxResetsSending + 1 {
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email) VALUES ($1)`, fmt.Sprintf("busy%d@example.com", i))
				require.NoError(t, err)
			}
			mailer := mailmocks.NewMailer(t)
			release := make(chan struct{})
			started := make(chan struct{}, maxResetsSending+2)
			if tt.wantSends > 0 {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(context.Context, mail.Message) {
					started <- struct{}{}
					<-release
				}).Return(nil).Times(tt.wantSends)
			}
			resets := NewResets(svc, mailer, testBaseURL)
			for i := range tt.busy {
				resets.Request(t.Context(), fmt.Sprintf("busy%d@example.com", i))
				<-started
			}
			if tt.closed {
				resets.Close(time.Second)
			}

			resets.Request(t.Context(), testEmail)

			if tt.wantStarted > tt.busy {
				select {
				case <-started:
				case <-time.After(10 * time.Second):
					t.Fatal("the send under the cap didn't start")
				}
			} else {
				// Nothing should start; give it a moment to prove that.
				time.Sleep(200 * time.Millisecond)
				assert.Empty(t, started)
			}
			close(release)
			resets.Close(time.Minute)
		})
	}
}

func TestResetsCloseCancelsSendsPastTheGrace(t *testing.T) {
	svc, _, _ := withTestUser(t)
	mailer := mailmocks.NewMailer(t)
	started := make(chan struct{})
	var sendErr error
	mailer.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ mail.Message) error {
		close(started)
		<-ctx.Done()
		sendErr = ctx.Err()
		return sendErr
	}).Once()
	resets := NewResets(svc, mailer, testBaseURL)
	resets.Request(t.Context(), testEmail)
	<-started

	resets.Close(10 * time.Millisecond)

	resets.sending.Wait()
	assert.ErrorIs(t, sendErr, context.Canceled)
}
