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
		{name: "a user without a password can add one", email: testEmail, setup: `UPDATE users SET password_hash = NULL, password_changed_at = NULL`, wantSend: true},
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
			resets.Close(time.Minute)

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
	// The workers send concurrently.
	bodies := make(chan string, resetsPerEmail)
	mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) {
		bodies <- msg.Body
	}).Return(nil).Times(resetsPerEmail)
	resets := NewResets(svc, mailer, testBaseURL)

	for range resetsPerEmail + 1 {
		resets.Request(t.Context(), testEmail)
	}
	resets.Close(time.Minute)

	assert.Equal(t, resetsPerEmail, count(t, pool, `SELECT count(*) FROM auth_tokens`), "the request over the limit sends nothing")
	require.Len(t, bodies, resetsPerEmail)
	_, _, err := svc.SetPasswordWithToken(t.Context(), SetPasswordInput{Token: linkToken(t, <-bodies), Password: newPassword})
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

func TestResetsBoundTheWork(t *testing.T) {
	tests := []struct {
		name string
		// Sends still running, and requests waiting behind them, when the next request comes in.
		busy, queued int
		closed       bool
		wantStarted  bool
		wantSends    int
		wantRefunded bool
	}{
		{name: "a free worker starts at once", busy: resetWorkers - 1, wantStarted: true, wantSends: resetWorkers},
		{name: "busy workers: it waits in the queue", busy: resetWorkers, wantSends: resetWorkers + 1},
		{name: "a full queue drops it", busy: resetWorkers, queued: resetQueueLength, wantSends: resetWorkers + resetQueueLength, wantRefunded: true},
		{name: "after Close nothing starts", closed: true, wantRefunded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool, _ := withTestUser(t)
			others := make([]string, tt.busy+tt.queued)
			for i := range others {
				others[i] = fmt.Sprintf("other%d@example.com", i)
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email) VALUES ($1)`, others[i])
				require.NoError(t, err)
			}
			mailer := mailmocks.NewMailer(t)
			release := make(chan struct{})
			started := make(chan string, len(others)+1)
			if tt.wantSends > 0 {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) {
					started <- msg.To
					<-release
				}).Return(nil).Times(tt.wantSends)
			}
			resets := NewResets(svc, mailer, testBaseURL)
			for _, email := range others[:tt.busy] {
				resets.Request(t.Context(), email)
				<-started
			}
			for _, email := range others[tt.busy:] {
				resets.Request(t.Context(), email)
			}
			if tt.closed {
				resets.Close(time.Second)
			}

			resets.Request(t.Context(), testEmail)

			if tt.wantStarted {
				select {
				case to := <-started:
					assert.Equal(t, testEmail, to)
				case <-time.After(10 * time.Second):
					t.Fatal("the send with a free worker didn't start")
				}
			} else {
				// Nothing should start yet; give it a moment to prove that.
				time.Sleep(200 * time.Millisecond)
				assert.Empty(t, started)
			}
			if tt.wantRefunded {
				for range resetsPerEmail {
					ok, _ := resets.perEmail.Take(testEmail)
					assert.True(t, ok, "a dropped request gives its allowance back")
				}
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

	resets.workers.Wait()
	assert.ErrorIs(t, sendErr, context.Canceled)
}
