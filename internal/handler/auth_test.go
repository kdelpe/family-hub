package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var testUser = auth.User{ID: uuid.MustParse("01890000-0000-7000-8000-000000000001"), Email: "test@example.com", Role: "member"}

const testUserBody = `{"id":"01890000-0000-7000-8000-000000000001","email":"test@example.com","role":"member"}`

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
		{name: "signed in", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, wantStatus: http.StatusOK, wantBody: testUserBody, wantCookie: true},
		{name: "replaces the session of an old cookie", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, oldCookie: true, wantStatus: http.StatusOK, wantBody: testUserBody, wantCookie: true},
		{name: "invalid credentials", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, loginErr: auth.ErrInvalidCredentials, wantStatus: http.StatusUnauthorized,
			wantBody: `{"error":{"code":"invalid_credentials","message":"email or password is incorrect"}}`},
		{name: "too many failures", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, loginErr: &auth.RateLimitedError{RetryAfter: 90 * time.Second}, wantStatus: http.StatusTooManyRequests,
			wantBody: `{"error":{"code":"rate_limited","message":"too many attempts, try again later"}}`},
		{name: "hashing busy", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, loginErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", body: `{"email":"test@example.com","password":"pw"}`, wantLogin: true, loginErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
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
				in := auth.LoginInput{Email: "test@example.com", Password: "pw", UserAgent: "test-agent", ClientIP: "192.0.2.1"}
				if tt.oldCookie {
					in.ReplacesToken = "old"
				}
				svc.EXPECT().Login(mock.Anything, in).Return("tok", testUser, tt.loginErr)
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

			handler.NewAuth(svc, nil, handler.NewSessionCookie(true)).Login(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
			if tt.wantStatus == http.StatusTooManyRequests {
				assert.Equal(t, "90", rec.Header().Get("Retry-After"))
			}
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

func TestResetPassword(t *testing.T) {
	const body = `{"token":"link-token","password":"purple staple orbit"}`
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantCall   bool
		oldCookie  bool
		wantStatus int
		wantBody   string
	}{
		{name: "password set", body: body, wantCall: true, wantStatus: http.StatusOK, wantBody: testUserBody},
		{name: "replaces the session of an old cookie", body: body, wantCall: true, oldCookie: true, wantStatus: http.StatusOK, wantBody: testUserBody},
		{name: "invalid token", body: body, wantCall: true, svcErr: auth.ErrInvalidToken, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"invalid_token","message":"this link is invalid, used or expired; ask for a new one"}}`},
		{name: "password too short", body: body, wantCall: true, svcErr: auth.ErrPasswordTooShort, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"password must be at least 8 characters"}}`},
		{name: "password too long", body: body, wantCall: true, svcErr: auth.ErrPasswordTooLong, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"password must be at most 128 characters"}}`},
		{name: "password too common", body: body, wantCall: true, svcErr: auth.ErrPasswordTooCommon, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"password is too common"}}`},
		{name: "busy", body: body, wantCall: true, svcErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", body: body, wantCall: true, svcErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "unknown field", body: `{"token":"a","password":"b","role":"admin"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			if tt.wantCall {
				in := auth.SetPasswordInput{Token: "link-token", Password: "purple staple orbit", UserAgent: "test-agent"}
				if tt.oldCookie {
					in.ReplacesToken = "old"
				}
				svc.EXPECT().SetPasswordWithToken(mock.Anything, in).Return("tok", testUser, tt.svcErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/auth/password/reset", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")
			if tt.oldCookie {
				req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "old"})
			}
			rec := httptest.NewRecorder()

			handler.NewAuth(svc, nil, handler.NewSessionCookie(true)).ResetPassword(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
			cookies := rec.Result().Cookies()
			if tt.wantStatus != http.StatusOK {
				assert.Empty(t, cookies)
				return
			}
			require.Len(t, cookies, 1)
			assert.Equal(t, "__Host-fh_session", cookies[0].Name)
			assert.Equal(t, "tok", cookies[0].Value)
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
		{name: "database error still clears", cookie: true, logoutErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantClear: true},
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

			handler.NewAuth(svc, nil, handler.NewSessionCookie(false)).Logout(rec, req)

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
		{name: "the signed-in user", wantStatus: http.StatusOK, wantBody: testUserBody},
		{name: "user gone since the session check", userErr: auth.ErrNoSession, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "database error", userErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			svc.EXPECT().User(mock.Anything, testUser.ID).Return(testUser, tt.userErr)
			sessions := mocks.NewSessionAuthenticator(t)
			sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(auth.Session{UserID: testUser.ID, Role: testUser.Role}, nil)
			cookie := handler.NewSessionCookie(true)
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
			req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
			rec := httptest.NewRecorder()

			handler.NewSessionGate(sessions, cookie).Require(http.HandlerFunc(handler.NewAuth(svc, nil, cookie).Me)).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestPreviewLink(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		preview     auth.LinkPreview
		previewErr  error
		wantPreview bool
		canceled    bool
		wantStatus  int
		wantBody    string
	}{
		{name: "invite", body: `{"token":"tok"}`, wantPreview: true, preview: auth.LinkPreview{Purpose: "invite", Email: "test@gmail.com", GoogleEligible: true}, wantStatus: http.StatusOK,
			wantBody: `{"purpose":"invite","invitee_email":"test@gmail.com","invitee_first_name":null,"from_name":null,"google_eligible":true}`},
		{name: "password reset", body: `{"token":"tok"}`, wantPreview: true, preview: auth.LinkPreview{Purpose: "password_reset", Email: "test@example.com"}, wantStatus: http.StatusOK,
			wantBody: `{"purpose":"password_reset","invitee_email":"test@example.com","invitee_first_name":null,"from_name":null,"google_eligible":false}`},
		{name: "invalid token", body: `{"token":"tok"}`, wantPreview: true, previewErr: auth.ErrInvalidToken, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"invalid_token","message":"this link is invalid, used or expired; ask for a new one"}}`},
		{name: "database error", body: `{"token":"tok"}`, wantPreview: true, previewErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", body: `{"token":"tok"}`, wantPreview: true, previewErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
		{name: "unknown field", body: `{"token":"tok","password":"x"}`, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			if tt.wantPreview {
				svc.EXPECT().PreviewLink(mock.Anything, "tok").Return(tt.preview, tt.previewErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/auth/invite/preview", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewAuth(svc, nil, handler.NewSessionCookie(true)).PreviewLink(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String())
				return
			}
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
			assert.Empty(t, rec.Result().Cookies(), "previewing never signs in")
		})
	}
}

func TestForgotPassword(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantRequest string
		wantStatus  int
		wantBody    string
	}{
		{name: "always accepted", body: `{"email":"test@example.com"}`, wantRequest: "test@example.com", wantStatus: http.StatusAccepted},
		{name: "unknown field", body: `{"email":"test@example.com","admin":true}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resets := mocks.NewResetRequester(t)
			if tt.wantRequest != "" {
				resets.EXPECT().Request(mock.Anything, tt.wantRequest).Once()
			}
			req := httptest.NewRequest(http.MethodPost, "/api/auth/password/forgot", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.NewAuth(mocks.NewAuthService(t), resets, handler.NewSessionCookie(true)).ForgotPassword(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String(), "the same empty answer for every address")
				return
			}
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestChangePassword(t *testing.T) {
	const body = `{"current_password":"old pw","new_password":"new pw"}`

	tests := []struct {
		name       string
		session    bool
		body       string
		changeErr  error
		wantChange bool
		wantStatus int
		wantBody   string
	}{
		{name: "changed", session: true, body: body, wantChange: true, wantStatus: http.StatusNoContent},
		{name: "wrong current password", session: true, body: body, wantChange: true, changeErr: auth.ErrInvalidCredentials, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"wrong_password","message":"current password is incorrect"}}`},
		{name: "new password refused", session: true, body: body, wantChange: true, changeErr: auth.ErrPasswordTooShort, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"` + auth.ErrPasswordTooShort.Error() + `"}}`},
		{name: "too many wrong attempts", session: true, body: body, wantChange: true, changeErr: &auth.RateLimitedError{RetryAfter: time.Minute}, wantStatus: http.StatusTooManyRequests,
			wantBody: `{"error":{"code":"rate_limited","message":"too many attempts, try again later"}}`},
		{name: "signed out meanwhile", session: true, body: body, wantChange: true, changeErr: auth.ErrNoSession, wantStatus: http.StatusUnauthorized, wantBody: unauthorizedBody},
		{name: "busy", session: true, body: body, wantChange: true, changeErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", session: true, body: body, wantChange: true, changeErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "unknown field", session: true, body: `{"current_password":"a","new_password":"b","user_id":"x"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "mounted without a session gate", body: body, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAuthService(t)
			if tt.wantChange {
				in := auth.ChangePasswordInput{UserID: testUser.ID, SessionToken: "tok", CurrentPassword: "old pw", NewPassword: "new pw", UserAgent: "test-agent"}
				svc.EXPECT().ChangePassword(mock.Anything, in).Return("fresh", tt.changeErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")
			req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
			h := http.HandlerFunc(handler.NewAuth(svc, nil, handler.NewSessionCookie(true)).ChangePassword)
			rec := httptest.NewRecorder()

			if tt.session {
				sessions := mocks.NewSessionAuthenticator(t)
				sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(auth.Session{UserID: testUser.ID, Role: auth.RoleMember}, nil)
				handler.NewSessionGate(sessions, handler.NewSessionCookie(true)).Require(h).ServeHTTP(rec, req)
			} else {
				h.ServeHTTP(rec, req)
			}

			assert.Equal(t, tt.wantStatus, rec.Code)
			cookies := rec.Result().Cookies()
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String())
				require.Len(t, cookies, 1, "the new session replaces the old cookie")
				assert.Equal(t, "fresh", cookies[0].Value)
				return
			}
			assert.Empty(t, cookies)
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}
