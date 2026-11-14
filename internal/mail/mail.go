// Package mail sends the app's emails (invites, password resets) over SMTP.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	sendTimeout = 30 * time.Second
	// Keeps every Subject line, ASCII or folded encoded words, below RFC 5322's 998-character limit.
	maxSubjectRunes = 200
)

var (
	ErrInvalidMessage = errors.New("invalid message")
	errNoTLS          = errors.New("SMTP server offers no STARTTLS; refusing to send in clear text to a remote host")
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string `json:"-"`
	From     mail.Address
}

// String, GoString and the json tag leave the password out, so logging a config can't leak it.
func (c SMTPConfig) String() string {
	return fmt.Sprintf("{Host:%s Port:%s Username:%s Password:[redacted] From:%s}", c.Host, c.Port, c.Username, c.From.String())
}

func (c SMTPConfig) GoString() string { return "mail.SMTPConfig" + c.String() }

type SMTP struct {
	cfg SMTPConfig
}

func (s *SMTP) String() string   { return "mail.SMTP" + s.cfg.String() }
func (s *SMTP) GoString() string { return s.String() }

func NewSMTP(cfg SMTPConfig) *SMTP {
	return &SMTP{cfg: cfg}
}

func (s *SMTP) Send(ctx context.Context, msg Message) error {
	if err := msg.validate(); err != nil {
		return err
	}
	data, err := s.build(msg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := s.send(ctx, msg.To, data); err != nil {
		// A cancel or timeout shows up as an I/O error; report the cause instead.
		// A reply from the server, like a 550, stays as it is.
		var reply *textproto.Error
		if ctx.Err() != nil && !errors.As(err, &reply) {
			err = ctx.Err()
		}
		return fmt.Errorf("smtp: %w", err)
	}
	return nil
}

func (s *SMTP) send(ctx context.Context, to string, data []byte) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, s.cfg.Port))
	if err != nil {
		return err
	}
	// net/smtp takes no context; a past deadline makes its blocked read or write return.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	// Explicit, so a failed EHLO is reported as itself rather than as "no STARTTLS". Strict
	// servers distrust "EHLO localhost" from afar; the sending domain is the usual name.
	if err := c.Hello(s.heloName()); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if !isLoopback(s.cfg.Host) {
		return errNoTLS
	}
	if s.cfg.Username != "" {
		// PlainAuth itself refuses to send the password without TLS, except to localhost.
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.cfg.From.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	// The server has accepted the message; failing now would invite a retry that sends it twice.
	_ = c.Quit()
	return nil
}

func (s *SMTP) heloName() string {
	if _, domain, ok := strings.Cut(s.cfg.From.Address, "@"); ok && !isLoopback(s.cfg.Host) {
		return domain
	}
	return "localhost"
}

func (m Message) validate() error {
	addr, err := mail.ParseAddress(m.To)
	switch {
	case err != nil || addr.Address != m.To:
		return fmt.Errorf("%w: recipient must be a bare address", ErrInvalidMessage)
	case strings.ContainsAny(m.Subject, "\r\n"):
		return fmt.Errorf("%w: subject must be one line", ErrInvalidMessage)
	case utf8.RuneCountInString(m.Subject) > maxSubjectRunes:
		return fmt.Errorf("%w: subject is longer than %d characters", ErrInvalidMessage, maxSubjectRunes)
	}
	return nil
}

func (s *SMTP) build(msg Message) ([]byte, error) {
	_, domain, _ := strings.Cut(s.cfg.From.Address, "@")
	var b bytes.Buffer
	headers := [][2]string{
		{"From", s.cfg.From.String()},
		{"To", (&mail.Address{Address: msg.To}).String()},
		// Folded between encoded words: encoding can triple the length of a non-ASCII subject.
		{"Subject", strings.ReplaceAll(mime.QEncoding.Encode("utf-8", msg.Subject), "?= =?", "?=\r\n =?")},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", "<" + rand.Text() + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=utf-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
	}
	for _, h := range headers {
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	b.WriteString("\r\n")
	// Quoted-printable keeps every line short and ASCII, and turns line ends into CRLF.
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(msg.Body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// The same hosts smtp.PlainAuth accepts without TLS, so the two checks never disagree
// (config lowercases the host).
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
