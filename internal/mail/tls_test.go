package mail

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const remoteHost = "smtp.example.com"

func TestSendOverTLS(t *testing.T) {
	var unknownAuthority x509.UnknownAuthorityError
	var wrongHost x509.HostnameError
	tests := []struct {
		name      string
		startTLS  bool
		certFor   string
		trustCert bool
		wantErr   func(t *testing.T, err error)
		wantCmds  []string
	}{
		{
			name:     "remote host without STARTTLS is refused before AUTH or MAIL",
			startTLS: false,
			wantErr:  func(t *testing.T, err error) { assert.ErrorIs(t, err, errNoTLS) },
			wantCmds: []string{"EHLO"},
		},
		{
			name:     "untrusted certificate is refused before AUTH or MAIL",
			startTLS: true, certFor: remoteHost,
			wantErr:  func(t *testing.T, err error) { assert.ErrorAs(t, err, &unknownAuthority) },
			wantCmds: []string{"EHLO", "STARTTLS"},
		},
		{
			name:     "certificate for another host is refused before AUTH or MAIL",
			startTLS: true, certFor: "other.example.com", trustCert: true,
			wantErr:  func(t *testing.T, err error) { assert.ErrorAs(t, err, &wrongHost) },
			wantCmds: []string{"EHLO", "STARTTLS"},
		},
		{
			name:     "authenticated delivery over verified TLS",
			startTLS: true, certFor: remoteHost, trustCert: true,
			wantCmds: []string{"EHLO", "STARTTLS", "EHLO", "AUTH", "MAIL", "RCPT", "DATA", "QUIT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, pool := selfSignedCert(t, tt.certFor)
			srv := startScriptedServer(t, tt.startTLS, cert)
			s := NewSMTP(SMTPConfig{Host: remoteHost, Port: "587", Username: "resend", Password: "hunter2", From: testFrom})
			s.dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.addr)
			}
			if tt.trustCert {
				s.roots = pool
			}

			err := s.Send(t.Context(), Message{To: "test@example.com", Subject: "Reset", Body: "https://family.example.com/reset?token=secret\n"})

			if tt.wantErr != nil {
				require.Error(t, err)
				tt.wantErr(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "\x00resend\x00hunter2", srv.credentials())
				assert.Contains(t, srv.body(), "Subject: Reset")
			}
			assert.Equal(t, tt.wantCmds, srv.commands())
		})
	}
}

// A minimal SMTP server that speaks STARTTLS and AUTH PLAIN and records the commands it gets.
type scriptedServer struct {
	addr     string
	startTLS bool
	cert     tls.Certificate

	mu    sync.Mutex
	cmds  []string
	creds string
	data  string
	done  chan struct{}
}

func startScriptedServer(t *testing.T, startTLS bool, cert tls.Certificate) *scriptedServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &scriptedServer{addr: ln.Addr().String(), startTLS: startTLS, cert: cert, done: make(chan struct{})}
	go func() {
		defer close(srv.done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		srv.serve(conn)
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-srv.done
	})
	return srv
}

func (s *scriptedServer) serve(conn net.Conn) {
	tp := textproto.NewConn(conn)
	secure := false
	_ = tp.PrintfLine("220 test ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		verb = strings.ToUpper(verb)
		s.record(verb)
		switch verb {
		case "EHLO":
			lines := []string{"250-test"}
			if s.startTLS && !secure {
				lines = append(lines, "250-STARTTLS")
			}
			if secure {
				lines = append(lines, "250-AUTH PLAIN")
			}
			_ = tp.PrintfLine("%s\r\n250 8BITMIME", strings.Join(lines, "\r\n"))
		case "STARTTLS":
			_ = tp.PrintfLine("220 go ahead")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn, tp, secure = tlsConn, textproto.NewConn(tlsConn), true
		case "AUTH":
			_, initial, _ := strings.Cut(arg, " ")
			decoded, _ := base64.StdEncoding.DecodeString(initial)
			s.mu.Lock()
			s.creds = string(decoded)
			s.mu.Unlock()
			_ = tp.PrintfLine("235 ok")
		case "MAIL", "RCPT":
			_ = tp.PrintfLine("250 ok")
		case "DATA":
			_ = tp.PrintfLine("354 go ahead")
			lines, err := tp.ReadDotLines()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.data = strings.Join(lines, "\n")
			s.mu.Unlock()
			_ = tp.PrintfLine("250 queued")
		case "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			_ = tp.PrintfLine("502 not implemented")
		}
	}
}

func (s *scriptedServer) record(verb string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cmds = append(s.cmds, verb)
}

// The getters wait for the connection to end, so a refused handshake is fully recorded.
func (s *scriptedServer) commands() []string {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmds
}

func (s *scriptedServer) credentials() string {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creds
}

func (s *scriptedServer) body() string {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// A self-signed certificate for host, and a pool that trusts it.
func selfSignedCert(t *testing.T, host string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: host},
		DNSNames:              []string{host},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
