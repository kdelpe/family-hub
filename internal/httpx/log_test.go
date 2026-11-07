package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		handler     http.HandlerFunc
		wantLogged  bool
		wantLevel   string
		wantStatus  float64
		wantBytes   float64
		wantAborted bool
	}{
		{name: "nothing written is a 200", path: "/persons", handler: func(http.ResponseWriter, *http.Request) {},
			wantLogged: true, wantLevel: "info", wantStatus: 200},
		{name: "status and size", path: "/persons", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("gone"))
		}, wantLogged: true, wantLevel: "info", wantStatus: 404, wantBytes: 4},
		{name: "a panic is logged as the 500 it got", path: "/persons", handler: func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}, wantLogged: true, wantLevel: "info", wantStatus: 500},
		{name: "an aborted request is not a 200", path: "/persons", handler: func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}, wantLogged: true, wantLevel: "warn", wantStatus: 0, wantAborted: true},
		{name: "a quiet path that succeeds is left out", path: "/health", handler: func(http.ResponseWriter, *http.Request) {}},
		{name: "a quiet path that fails is logged", path: "/health", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}, wantLogged: true, wantLevel: "info", wantStatus: 503},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, logs := newRequest(tt.path + "?q=secret")
			h := RequestID(RequestLogger("/health")(Recover(tt.handler)))

			serve(h, r)

			line, ok := logs.find(t, "request")
			require.Equal(t, tt.wantLogged, ok)
			if !ok {
				return
			}
			assert.Equal(t, tt.wantLevel, line["level"])
			assert.Equal(t, "GET", line["method"])
			assert.Equal(t, tt.path, line["path"], "the query is left out")
			assert.Equal(t, tt.wantStatus, line["status"])
			assert.Equal(t, tt.wantBytes, line["bytes"])
			assert.Equal(t, tt.wantAborted, line["aborted"] == true)
			assert.NotEmpty(t, line["request_id"])
			assert.Contains(t, line, "duration_ms")
		})
	}
}

func TestRecover(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantAbort  bool
		wantLogged bool
	}{
		{name: "a panic answers 500", handler: func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}, wantStatus: 500, wantLogged: true},
		{name: "the 500 drops the handler's headers", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Set-Cookie", "session=x")
			panic("boom")
		}, wantStatus: 500, wantLogged: true},
		{name: "a panic after the response started drops the connection", handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("partial"))
			panic("boom")
		}, wantStatus: 200, wantAbort: true, wantLogged: true},
		{name: "an abort passes through unlogged", handler: func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}, wantStatus: 200, wantAbort: true},
		{name: "a wrapped abort passes on the exact value", handler: func(http.ResponseWriter, *http.Request) {
			panic(fmt.Errorf("proxy: %w", http.ErrAbortHandler))
		}, wantStatus: 200, wantAbort: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, logs := newRequest("/")

			rec, aborted := serve(withHeader("X-Before", "kept", Recover(tt.handler)), r)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantAbort, aborted)
			if !aborted {
				assert.Equal(t, "kept", rec.Header().Get("X-Before"))
				assert.Empty(t, rec.Header().Get("Set-Cookie"))
			}
			line, ok := logs.find(t, "handler panicked")
			require.Equal(t, tt.wantLogged, ok)
			if ok {
				assert.Equal(t, "error", line["level"])
				assert.Equal(t, "boom", line["panic"])
				assert.Contains(t, line["stack"], "log_test.go")
			}
		})
	}
}

func TestRequestIDIgnoresTheClients(t *testing.T) {
	r, logs := newRequest("/")
	r.Header.Set(requestIDHeader, "forged")

	rec, _ := serve(RequestID(RequestLogger()(http.NotFoundHandler())), r)

	id := rec.Header().Get(requestIDHeader)
	assert.NotEmpty(t, id)
	assert.NotEqual(t, "forged", id)
	line, ok := logs.find(t, "request")
	require.True(t, ok)
	assert.Equal(t, id, line["request_id"])
}

func withHeader(key, value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(key, value)
		next.ServeHTTP(w, r)
	})
}

type logBuffer struct{ bytes.Buffer }

// The logger travels in the request's context, as RequestID expects.
func newRequest(target string) (*http.Request, *logBuffer) {
	logs := &logBuffer{}
	r := httptest.NewRequest(http.MethodGet, target, nil)
	return r.WithContext(zerolog.New(logs).WithContext(r.Context())), logs
}

// aborted reports a panic with exactly http.ErrAbortHandler: net/http drops the connection quietly only for that value.
func serve(h http.Handler, r *http.Request) (rec *httptest.ResponseRecorder, aborted bool) {
	rec = httptest.NewRecorder()
	defer func() {
		if v := recover(); v != nil {
			if v != http.ErrAbortHandler {
				panic(v)
			}
			aborted = true
		}
	}()
	h.ServeHTTP(rec, r)
	return rec, false
}

func (b *logBuffer) find(t *testing.T, msg string) (map[string]any, bool) {
	t.Helper()
	dec := json.NewDecoder(&b.Buffer)
	for dec.More() {
		var line map[string]any
		require.NoError(t, dec.Decode(&line))
		if line["message"] == msg {
			return line, true
		}
	}
	return nil, false
}
