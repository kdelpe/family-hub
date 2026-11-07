package app

import (
	"context"
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
	go func() { done <- serve(ctx, newServer(pool), ln, time.Second) }()

	resp := waitForServer(t, "http://"+ln.Addr().String()+"/ready", done)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	require.NoError(t, <-done)
}

func TestRunFailsWithoutADatabase(t *testing.T) {
	err := Run(t.Context(), config.Config{DatabaseURL: testdb.UnreachableURL, HTTPAddr: "127.0.0.1:0"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database:")
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
