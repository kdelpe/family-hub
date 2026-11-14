package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
	"github.com/kdelpe/family-hub/internal/httpx"
	"github.com/kdelpe/family-hub/internal/ratelimit"
	"github.com/kdelpe/family-hub/internal/router"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		header     http.Header
		wantStatus int
		wantType   string
	}{
		{name: "liveness", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "readiness reaches the database check", method: http.MethodGet, path: "/ready", wantStatus: http.StatusServiceUnavailable},
		{name: "liveness answers HEAD", method: http.MethodHead, path: "/health", wantStatus: http.StatusOK},
		{name: "health is GET only", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed},
		{name: "health is not under /api", method: http.MethodGet, path: "/api/health", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "unknown /api path is a JSON 404", method: http.MethodGet, path: "/api/nope", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "/api itself is a JSON 404", method: http.MethodGet, path: "/api", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "login reaches its handler", method: http.MethodPost, path: "/api/auth/login", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "forgot password reaches its handler", method: http.MethodPost, path: "/api/auth/password/forgot", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "invite preview reaches its handler", method: http.MethodPost, path: "/api/auth/invite/preview", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "providers needs no session", method: http.MethodGet, path: "/api/auth/providers", wantStatus: http.StatusOK, wantType: "application/json"},
		{name: "Google sign-in is disabled without configuration", method: http.MethodGet, path: "/api/auth/google/start", wantStatus: http.StatusSeeOther},
		{name: "Google callback is disabled without configuration", method: http.MethodGet, path: "/api/auth/google/callback", wantStatus: http.StatusSeeOther},
		{name: "password reset reaches its handler", method: http.MethodPost, path: "/api/auth/password/reset", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "wrong method on an /api route is a JSON 405", method: http.MethodGet, path: "/api/auth/login", wantStatus: http.StatusMethodNotAllowed, wantType: "application/json"},
		{name: "me needs a session", method: http.MethodGet, path: "/api/auth/me", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "changing the password needs a session", method: http.MethodPost, path: "/api/auth/password/change", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "listing users needs a session", method: http.MethodGet, path: "/api/admin/users", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "changing a user needs a session", method: http.MethodPatch, path: "/api/admin/users/01890000-0000-7000-8000-000000000001", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "sending a reset needs a session", method: http.MethodPost, path: "/api/admin/users/01890000-0000-7000-8000-000000000001/send-reset", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "inviting needs a session", method: http.MethodPost, path: "/api/admin/users", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "logout without a session still clears", method: http.MethodPost, path: "/api/auth/logout", wantStatus: http.StatusNoContent},
		{name: "cross-site POST to /api is refused", method: http.MethodPost, path: "/api/auth/login", header: http.Header{"Sec-Fetch-Site": {"cross-site"}}, wantStatus: http.StatusForbidden, wantType: "application/json"},
		{name: "any other path is the app", method: http.MethodGet, path: "/persons/42", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8"},
		{name: "the app answers HEAD", method: http.MethodHead, path: "/", wantStatus: http.StatusOK},
		{name: "the app is GET only", method: http.MethodPost, path: "/", wantStatus: http.StatusMethodNotAllowed},
	}

	routes := newRoutes(t, 100, false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.header {
				req.Header[k] = v
			}

			routes.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantType != "" {
				assert.Equal(t, tt.wantType, rec.Header().Get("Content-Type"))
			}
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), "security headers on every response")
		})
	}
}

func TestSignInRoutesAreRateLimitedPerClientIP(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		trustedProxy bool
		secondPath   string
		secondFrom   http.Header
		wantStatus   int
	}{
		{name: "login, same client", path: "/api/auth/login", wantStatus: http.StatusTooManyRequests},
		{name: "login, a spoofed X-Forwarded-For without a proxy", path: "/api/auth/login", secondFrom: http.Header{"X-Forwarded-For": {"198.51.100.1"}}, wantStatus: http.StatusTooManyRequests},
		{name: "login, another client behind the proxy", path: "/api/auth/login", trustedProxy: true, secondFrom: http.Header{"X-Forwarded-For": {"198.51.100.1"}}, wantStatus: http.StatusUnsupportedMediaType},
		{name: "reset, same client", path: "/api/auth/password/reset", wantStatus: http.StatusTooManyRequests},
		{name: "reset, another client behind the proxy", path: "/api/auth/password/reset", trustedProxy: true, secondFrom: http.Header{"X-Forwarded-For": {"198.51.100.1"}}, wantStatus: http.StatusUnsupportedMediaType},
		{name: "forgot, same client", path: "/api/auth/password/forgot", wantStatus: http.StatusTooManyRequests},
		{name: "forgot and login have their own limits", path: "/api/auth/password/forgot", secondPath: "/api/auth/login", wantStatus: http.StatusUnsupportedMediaType},
		{name: "preview, same client", path: "/api/auth/invite/preview", wantStatus: http.StatusTooManyRequests},
		{name: "reset and preview have their own limits", path: "/api/auth/password/reset", secondPath: "/api/auth/invite/preview", wantStatus: http.StatusUnsupportedMediaType},
		{name: "login and reset have their own limits", path: "/api/auth/login", secondPath: "/api/auth/password/reset", wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := newRoutes(t, 1, tt.trustedProxy)
			post := func(path string, header http.Header) int {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.7")
				for k, v := range header {
					req.Header[k] = v
				}
				rec := httptest.NewRecorder()
				routes.ServeHTTP(rec, req)
				return rec.Code
			}
			second := tt.path
			if tt.secondPath != "" {
				second = tt.secondPath
			}

			require.Equal(t, http.StatusUnsupportedMediaType, post(tt.path, nil), "first attempt reaches the handler")
			assert.Equal(t, tt.wantStatus, post(second, tt.secondFrom))
		})
	}
}

func TestGoogleRoutesSharePerIPRateLimit(t *testing.T) {
	routes := newRoutes(t, 1, false)
	for i, path := range []string{"/api/auth/google/start", "/api/auth/google/callback"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)
		if i == 0 {
			assert.Equal(t, http.StatusSeeOther, rec.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		}
	}
}

func newRoutes(t *testing.T, perHour int, trustedProxy bool) http.Handler {
	t.Helper()
	cookie := handler.NewSessionCookie(true)
	return router.New(
		router.Handlers{
			// Unreachable, so /ready answers 503 without needing Postgres.
			Health: handler.NewHealth(testdb.UnreachablePool(t)),
			Auth:   handler.NewAuth(mocks.NewAuthService(t), mocks.NewResetRequester(t), cookie),
			Admin:  handler.NewAdmin(mocks.NewAdminService(t)),
		},
		router.Middleware{
			RequireSession: handler.NewSessionGate(mocks.NewSessionAuthenticator(t), cookie).Require,
			LimitLogin:     httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitForgot:    httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitReset:     httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitPreview:   httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitGoogle:    httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			TrustedProxy:   trustedProxy,
		},
	)
}
