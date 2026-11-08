package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
)

type authenticatorFunc func(ctx context.Context, token string) (auth.Session, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, token string) (auth.Session, error) {
	return f(ctx, token)
}

const (
	unauthorizedBody = `{"error":{"code":"unauthorized","message":"sign in required"}}`
	internalBody     = `{"error":{"code":"internal","message":"internal error"}}`
)

func TestSessionGateRequire(t *testing.T) {
	member := auth.Session{UserID: uuid.New(), Role: "member"}
	acceptOnly := func(want string) authenticatorFunc {
		return func(_ context.Context, token string) (auth.Session, error) {
			if token != want {
				return auth.Session{}, auth.ErrNoSession
			}
			return member, nil
		}
	}

	tests := []struct {
		name         string
		secureCookie bool
		cookie       *http.Cookie
		sessions     authenticatorFunc
		wantStatus   int
		wantBody     string
	}{
		{name: "valid secure cookie", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, sessions: acceptOnly("tok"), wantStatus: http.StatusOK},
		{name: "valid local cookie", cookie: &http.Cookie{Name: "fh_session", Value: "tok"}, sessions: acceptOnly("tok"), wantStatus: http.StatusOK},
		{name: "no cookie", secureCookie: true, sessions: acceptOnly("tok"), wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "local cookie name when secure", secureCookie: true, cookie: &http.Cookie{Name: "fh_session", Value: "tok"}, sessions: acceptOnly("tok"), wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "no active session", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "old"}, sessions: acceptOnly("tok"), wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{
			name: "database error", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"},
			sessions:   func(context.Context, string) (auth.Session, error) { return auth.Session{}, errors.New("db down") },
			wantStatus: http.StatusInternalServerError, wantBody: internalBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen auth.Session
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				var ok bool
				seen, ok = handler.CurrentSession(r.Context())
				require.True(t, ok)
			})
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()

			handler.NewSessionGate(tt.sessions, tt.secureCookie).Require(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Equal(t, member, seen)
				return
			}
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}
