package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const (
	unauthorizedBody = `{"error":{"code":"unauthorized","message":"sign in required"}}`
	internalBody     = `{"error":{"code":"internal","message":"internal error"}}`
)

func TestSessionGateRequire(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email) VALUES ('ana@example.com') RETURNING id`).Scan(&userID))
	sessions := auth.NewSessions(pool)
	token, err := sessions.Create(t.Context(), userID, "")
	require.NoError(t, err)
	unreachable := auth.NewSessions(testdb.UnreachablePool(t))

	tests := []struct {
		name         string
		sessions     handler.SessionAuthenticator
		secureCookie bool
		cookie       *http.Cookie
		wantStatus   int
		wantBody     string
	}{
		{name: "valid secure cookie", sessions: sessions, secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: token}, wantStatus: http.StatusOK},
		{name: "valid local cookie", sessions: sessions, cookie: &http.Cookie{Name: "fh_session", Value: token}, wantStatus: http.StatusOK},
		{name: "no cookie", sessions: sessions, secureCookie: true, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "local cookie name when secure", sessions: sessions, secureCookie: true, cookie: &http.Cookie{Name: "fh_session", Value: token}, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "unknown token", sessions: sessions, secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "database down", sessions: unreachable, secureCookie: true, cookie: &http.Cookie{Name: "__Host-fh_session", Value: token}, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
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
				assert.Equal(t, auth.Session{UserID: userID, Role: "member"}, seen)
				return
			}
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}
