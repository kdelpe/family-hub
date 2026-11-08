package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var ana = auth.User{ID: uuid.MustParse("01890000-0000-7000-8000-000000000001"), Email: "ana@example.com", Role: "member"}

const anaBody = `{"id":"01890000-0000-7000-8000-000000000001","email":"ana@example.com","role":"member"}`

func TestLogin(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		loginErr    error
		wantLogin   bool
		oldCookie   bool
		wantStatus  int
		wantBody    string
		wantCookie  bool
	}{
		{name: "signed in", body: `{"email":"ana@example.com","password":"pw"}`, wantLogin: true, wantStatus: http.StatusOK, wantBody: anaBody, wantCookie: true},
		{name: "replaces the session of an old cookie", body: `{"email":"ana@example.com","password":"pw"}`, wantLogin: true, oldCookie: true, wantStatus: http.StatusOK, wantBody: anaBody, wantCookie: true},
		{name: "invalid credentials", body: `{"email":"ana@example.com","password":"pw"}`, wantLogin: true, loginErr: auth.ErrInvalidCredentials, wantStatus: http.StatusUnauthorized,
			wantBody: `{"error":{"code":"invalid_credentials","message":"email or password is incorrect"}}`},
		{name: "hashing busy", body: `{"email":"ana@example.com","password":"pw"}`, wantLogin: true, loginErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", body: `{"email":"ana@example.com","password":"pw"}`, wantLogin: true, loginErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "not JSON", contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType,
			wantBody: `{"error":{"code":"unsupported_media_type","message":"send application/json"}}`},
		{name: "malformed JSON", body: `{"email":`, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "unknown field", body: `{"email":"a","password":"b","admin":true}`, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "two JSON values", body: `{"email":"a","password":"b"}{}`, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "trailing data over 1 MiB", body: `{"email":"a","password":"b"} "` + strings.Repeat("a", 1<<20) + `"`, wantStatus: http.StatusRequestEntityTooLarge,
			wantBody: `{"error":{"code":"too_large","message":"request body too large"}}`},
		{name: "body over 1 MiB", body: `{"email":"` + strings.Repeat("a", 1<<20) + `"}`, wantStatus: http.StatusRequestEntityTooLarge,
			wantBody: `{"error":{"code":"too_large","message":"request body too large"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			if tt.wantLogin {
				svc.EXPECT().Login(mock.Anything, "ana@example.com", "pw", "test-agent").Return("tok", ana, tt.loginErr)
			}
			if tt.oldCookie {
				svc.EXPECT().Logout(mock.Anything, "old").Return(nil)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			req.Header.Set("User-Agent", "test-agent")
			if tt.oldCookie {
				req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "old"})
			}
			rec := httptest.NewRecorder()

			handler.NewAuth(svc, handler.NewSessionCookie(true)).Login(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
			cookies := rec.Result().Cookies()
			if !tt.wantCookie {
				assert.Empty(t, cookies)
				return
			}
			require.Len(t, cookies, 1)
			c := cookies[0]
			assert.Equal(t, "__Host-fh_session", c.Name)
			assert.Equal(t, "tok", c.Value)
			assert.Equal(t, "/", c.Path)
			assert.Equal(t, int(auth.SessionLifetime.Seconds()), c.MaxAge)
			assert.True(t, c.HttpOnly)
			assert.True(t, c.Secure)
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name       string
		cookie     bool
		logoutErr  error
		wantStatus int
		wantClear  bool
	}{
		{name: "with a session cookie", cookie: true, wantStatus: http.StatusNoContent, wantClear: true},
		{name: "without a cookie", wantStatus: http.StatusNoContent, wantClear: true},
		{name: "database error", cookie: true, logoutErr: errors.New("db down"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			if tt.cookie {
				svc.EXPECT().Logout(mock.Anything, "tok").Return(tt.logoutErr)
				req.AddCookie(&http.Cookie{Name: "fh_session", Value: "tok"})
			}
			rec := httptest.NewRecorder()

			handler.NewAuth(svc, handler.NewSessionCookie(false)).Logout(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			cookies := rec.Result().Cookies()
			if !tt.wantClear {
				assert.Empty(t, cookies)
				return
			}
			require.Len(t, cookies, 1)
			assert.Equal(t, "fh_session", cookies[0].Name)
			assert.Negative(t, cookies[0].MaxAge)
		})
	}
}

func TestMe(t *testing.T) {
	tests := []struct {
		name       string
		userErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "the signed-in user", wantStatus: http.StatusOK, wantBody: anaBody},
		{name: "user gone since the session check", userErr: auth.ErrNoSession, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "database error", userErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			svc.EXPECT().User(mock.Anything, ana.ID).Return(ana, tt.userErr)
			sessions := mocks.NewSessionAuthenticator(t)
			sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(auth.Session{UserID: ana.ID, Role: ana.Role}, nil)
			cookie := handler.NewSessionCookie(true)
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
			req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
			rec := httptest.NewRecorder()

			handler.NewSessionGate(sessions, cookie).Require(http.HandlerFunc(handler.NewAuth(svc, cookie).Me)).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}
