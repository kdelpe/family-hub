package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestServerAnswersReadyUntilCanceled(t *testing.T) {
	pool := testdb.Pool(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	srv, resets := newServer(pool, config.Config{CookieSecure: true}, nil)
	t.Cleanup(func() { resets.Close(time.Second) })
	go func() { done <- serve(ctx, srv, ln, time.Second) }()

	resp := waitForServer(t, "http://"+ln.Addr().String()+"/ready", done)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	noRedirects := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	googleResp, err := noRedirects.Get("http://" + ln.Addr().String() + "/api/auth/google/start")
	require.NoError(t, err)
	defer func() { _ = googleResp.Body.Close() }()
	assert.Equal(t, http.StatusSeeOther, googleResp.StatusCode)
	assert.Equal(t, "/auth/login?google=unavailable", googleResp.Header.Get("Location"))
	callbackReq, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+"/api/auth/google/callback?state=state", nil)
	require.NoError(t, err)
	callbackReq.AddCookie(&http.Cookie{Name: "fh_oauth", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	callbackResp, err := noRedirects.Do(callbackReq)
	require.NoError(t, err)
	defer func() { _ = callbackResp.Body.Close() }()
	assert.Equal(t, http.StatusSeeOther, callbackResp.StatusCode)

	cancel()
	require.NoError(t, <-done)
}

func TestRunFailsWithoutADatabase(t *testing.T) {
	err := Run(t.Context(), config.Config{DatabaseURL: testdb.UnreachableURL, HTTPAddr: "127.0.0.1:0"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database:")
}

func TestRunCanceledBeforeListening(t *testing.T) {
	tests := []struct {
		name        string
		autoMigrate bool
		wantErr     bool
	}{
		{name: "is a clean stop", autoMigrate: false},
		{name: "fails during AUTO_MIGRATE", autoMigrate: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := Run(ctx, config.Config{DatabaseURL: testdb.UnreachableURL, HTTPAddr: "127.0.0.1:0", AutoMigrate: tt.autoMigrate})

			if tt.wantErr {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServeLetsInFlightRequestsFinish(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, srv, ln, time.Second) }()

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err != nil {
			body <- err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()
	<-started
	cancel()
	select {
	case err := <-served:
		t.Fatalf("serve returned before the request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	assert.Equal(t, "done", <-body)
	require.NoError(t, <-served)
}

func TestServeCancelsRequestsThatOutlastTheGrace(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, srv, ln, 100*time.Millisecond) }()
	go func() {
		if resp, err := http.Get("http://" + ln.Addr().String()); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started

	cancel()

	err = <-served
	require.ErrorIs(t, err, context.DeadlineExceeded)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("the stuck request's context was never canceled")
	}
}

func TestServeReportsAFailedListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, ln.Close())

	err = serve(t.Context(), &http.Server{}, ln, time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "http:")
}

func TestServeCancelsRequestsWhenTheListenerFails(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ln := &failingListener{Listener: inner, fail: make(chan struct{})}
	served := make(chan error, 1)
	go func() { served <- serve(t.Context(), srv, ln, time.Second) }()
	go func() {
		if resp, err := http.Get("http://" + inner.Addr().String()); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started

	close(ln.fail)

	require.Error(t, <-served)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("the in-flight request's context was never canceled")
	}
}

// Accepts the first connection, then fails once fail is closed.
type failingListener struct {
	net.Listener
	fail     chan struct{}
	accepted bool
}

func (l *failingListener) Accept() (net.Conn, error) {
	if !l.accepted {
		l.accepted = true
		return l.Listener.Accept()
	}
	<-l.fail
	_ = l.Close()
	return nil, errors.New("listener failed")
}

// Fails with Run's own error if it returns before the server answers.
func waitForServer(t *testing.T, url string, done <-chan error) *http.Response {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			return resp
		}
		select {
		case err := <-done:
			t.Fatalf("serve returned before answering: %v", err)
		case <-deadline:
			t.Fatalf("server never answered: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
