package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

const (
	unauthorizedBody = `{"error":{"code":"unauthorized","message":"sign in required"}}`
	internalBody     = `{"error":{"code":"internal","message":"internal error"}}`
)

func TestSessionGateRequire(t *testing.T) {
	member := auth.Session{UserID: uuid.New(), Role: "member"}

	tests := []struct {
		name         string
		secureCookie bool
		cookie       *http.Cookie
		authenticate bool
		authErr      error
		canceled     bool
		wantStatus   int
		wantBody     string
	}{
		{name: "valid secure cookie", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, authenticate: true, wantStatus: http.StatusOK},
		{name: "valid local cookie", cookie: &http.Cookie{Name: "fh_session", Value: "tok"}, authenticate: true, wantStatus: http.StatusOK},
		{name: "no cookie", secureCookie: true, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "secure cookie name in local dev", cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "local cookie name when secure", secureCookie: true, cookie: &http.Cookie{Name: "fh_session", Value: "tok"}, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "no active session", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, authenticate: true, authErr: auth.ErrNoSession, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "client gone", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, authenticate: true, authErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
		{name: "client gone after a valid session", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, authenticate: true, canceled: true, wantStatus: http.StatusOK},
		{name: "database error", secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "tok"}, authenticate: true, authErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := mocks.NewSessionAuthenticator(t)
			if tt.authenticate {
				sessions.EXPECT().Authenticate(mock.Anything, tt.cookie.Value).Return(member, tt.authErr)
			}
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
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewSessionGate(sessions, tt.secureCookie).Require(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.canceled {
				assert.Empty(t, rec.Body.String(), "nothing written for a client that left")
				assert.Zero(t, seen, "protected handler not run")
				return
			}
			if tt.wantBody == "" {
				assert.Equal(t, member, seen)
				return
			}
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}
