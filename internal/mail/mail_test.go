package mail

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFrom = mail.Address{Name: "Família Hub", Address: "noreply@family.example.com"}

func TestSendDeliversTheMessage(t *testing.T) {
	longLine := strings.Repeat("word ", 40)
	tests := []struct {
		name    string
		subject string
		body    string
	}{
		{name: "plain text", subject: "Your invite", body: "Hello,\nyour link: https://family.example.com/invite?token=abc\n"},
		{name: "non-ASCII subject and body", subject: "Bienvenue, André", body: "Ça marche — émoji 🌳\n"},
		{name: "long line and a leading dot", subject: "Long", body: longLine + "\n.just a dot line\n"},
		{name: "longest accented subject", subject: strings.Repeat("é", maxSubjectRunes), body: "x\n"},
		{name: "longest emoji subject", subject: strings.Repeat("🌳", maxSubjectRunes), body: "x\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailpit := mailpitURL(t)
			to := uniqueRecipient()

			err := NewSMTP(mailpitConfig(t, mailpit)).Send(t.Context(), Message{To: to, Subject: tt.subject, Body: tt.body})

			require.NoError(t, err)
			got := fetchMessage(t, mailpit, to)
			assert.Equal(t, tt.subject, got.Subject)
			assert.Equal(t, strings.ReplaceAll(tt.body, "\n", "\r\n"), got.Text)
			assert.Equal(t, testFrom.Address, got.From.Address)
			assert.Equal(t, testFrom.Name, got.From.Name)
			require.Len(t, got.To, 1)
			assert.Equal(t, to, got.To[0].Address)
			assert.NotEmpty(t, got.MessageID)
			assert.WithinDuration(t, time.Now(), got.Date, time.Minute)
		})
	}
}

func TestSendRefusesABadMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
	}{
		{name: "display name in the recipient", msg: Message{To: "Test <test@example.com>", Subject: "Hi"}},
		{name: "header injected via the recipient", msg: Message{To: "test@example.com\r\nBcc: x@example.com", Subject: "Hi"}},
		{name: "two recipients", msg: Message{To: "test@example.com, other@example.com", Subject: "Hi"}},
		{name: "header injected via the subject", msg: Message{To: "test@example.com", Subject: "Hi\r\nBcc: x@example.com"}},
		{name: "newline in the subject", msg: Message{To: "test@example.com", Subject: "Hi\nthere"}},
		{name: "subject too long", msg: Message{To: "test@example.com", Subject: strings.Repeat("é", maxSubjectRunes+1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Refused before connecting, so this server is never dialed.
			s := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: "1", From: testFrom})

			err := s.Send(t.Context(), tt.msg)

			assert.ErrorIs(t, err, ErrInvalidMessage)
		})
	}
}

func TestSendFailsWhenItCantDeliver(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func(t *testing.T) SMTPConfig
		ctx     func(t *testing.T) context.Context
		wantErr error
	}{
		{
			name: "nothing listening",
			cfg:  func(*testing.T) SMTPConfig { return SMTPConfig{Host: "127.0.0.1", Port: "1", From: testFrom} },
		},
		{
			name: "canceled context",
			cfg:  func(*testing.T) SMTPConfig { return SMTPConfig{Host: "127.0.0.1", Port: "1", From: testFrom} },
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "server stops answering",
			cfg:  func(t *testing.T) SMTPConfig { return silentServer(t) },
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
				t.Cleanup(cancel)
				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx(t)
			}

			err := NewSMTP(tt.cfg(t)).Send(ctx, Message{To: "test@example.com", Subject: "Hi"})

			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestBuildKeepsEveryLineShort(t *testing.T) {
	tests := []struct {
		name    string
		subject string
	}{
		{name: "longest ASCII subject", subject: strings.Repeat("a", maxSubjectRunes)},
		{name: "longest accented subject", subject: strings.Repeat("é", maxSubjectRunes)},
		{name: "longest emoji subject", subject: strings.Repeat("🌳", maxSubjectRunes)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := NewSMTP(SMTPConfig{From: testFrom}).build(Message{To: "test@example.com", Subject: tt.subject, Body: strings.Repeat("é", 2000)})

			require.NoError(t, err)
			for line := range strings.SplitSeq(string(data), "\r\n") {
				assert.LessOrEqual(t, len(line), 998)
			}
		})
	}
}

func TestSMTPConfigNeverPrintsThePassword(t *testing.T) {
	cfg := SMTPConfig{Host: "smtp.example.com", Port: "587", Username: "resend", Password: "hunter2", From: testFrom}

	for _, format := range []string{"%v", "%+v", "%s", "%#v"} {
		assert.NotContains(t, fmt.Sprintf(format, cfg), "hunter2", format)
		assert.NotContains(t, fmt.Sprintf(format, NewSMTP(cfg)), "hunter2", format)
	}
	asJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.NotContains(t, string(asJSON), "hunter2")
}

func TestHeloName(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "remote provider gets the sending domain", host: "smtp.example.com", want: "family.example.com"},
		{name: "local Mailpit", host: "localhost", want: "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NewSMTP(SMTPConfig{Host: tt.host, From: testFrom}).heloName())
		})
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "localhost", want: true},
		{host: "LocalHost"},
		{host: "127.0.0.2"},
		{host: "127.0.0.1", want: true},
		{host: "::1", want: true},
		{host: "smtp.example.com"},
		{host: "192.0.2.1"},
		{host: "localhost.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			assert.Equal(t, tt.want, isLoopback(tt.host))
		})
	}
}

// Accepts connections and never sends the SMTP greeting.
func silentServer(t *testing.T) SMTPConfig {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return SMTPConfig{Host: host, Port: port, From: testFrom}
}

// mailpitURL returns TEST_MAILPIT_URL (Mailpit's HTTP API), skipping without it, except in CI.
func mailpitURL(t *testing.T) *url.URL {
	t.Helper()
	raw := os.Getenv("TEST_MAILPIT_URL")
	if raw == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_MAILPIT_URL must be set in CI")
		}
		t.Skip("TEST_MAILPIT_URL not set; run `make up` and `make test`")
	}
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func mailpitConfig(t *testing.T, mailpit *url.URL) SMTPConfig {
	t.Helper()
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "1025"
	}
	return SMTPConfig{Host: mailpit.Hostname(), Port: port, From: testFrom}
}

// Unique per test, so parallel packages and earlier runs never see each other's mail.
func uniqueRecipient() string {
	return "test-" + strings.ToLower(rand.Text()) + "@example.com"
}

type mailpitMessage struct {
	MessageID string
	From      mail.Address
	To        []mail.Address
	Subject   string
	Text      string
	Date      time.Time
}

func fetchMessage(t *testing.T, mailpit *url.URL, to string) mailpitMessage {
	t.Helper()
	type found struct {
		Messages []struct{ ID string }
	}
	search := mailpit.JoinPath("api/v1/search").String() + "?query=" + url.QueryEscape("to:"+to)
	var ids found
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ids = found{}
		assert.NoError(c, getJSON(t.Context(), search, &ids))
		assert.Len(c, ids.Messages, 1, "messages to %s", to)
	}, 5*time.Second, 50*time.Millisecond)
	var msg mailpitMessage
	require.NoError(t, getJSON(t.Context(), mailpit.JoinPath("api/v1/message", ids.Messages[0].ID).String(), &msg))
	return msg
}

var mailpitClient = &http.Client{Timeout: 5 * time.Second}

func getJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := mailpitClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
