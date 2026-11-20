package router_test

import (
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
	"github.com/kdelpe/family-hub/internal/httpx"
	"github.com/kdelpe/family-hub/internal/ratelimit"
	"github.com/kdelpe/family-hub/internal/router"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const testID = "01890000-0000-7000-8000-000000000001"

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
		{name: "password reset reaches its handler", method: http.MethodPost, path: "/api/auth/password/reset", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "wrong method on an /api route is a JSON 405", method: http.MethodGet, path: "/api/auth/login", wantStatus: http.StatusMethodNotAllowed, wantType: "application/json"},
		{name: "me needs a session", method: http.MethodGet, path: "/api/auth/me", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "changing the password needs a session", method: http.MethodPost, path: "/api/auth/password/change", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "listing users needs a session", method: http.MethodGet, path: "/api/admin/users", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "changing a user needs a session", method: http.MethodPatch, path: "/api/admin/users/01890000-0000-7000-8000-000000000001", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "sending a reset needs a session", method: http.MethodPost, path: "/api/admin/users/01890000-0000-7000-8000-000000000001/send-reset", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "inviting needs a session", method: http.MethodPost, path: "/api/admin/users", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "listing people needs a session", method: http.MethodGet, path: "/api/persons", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "a profile needs a session", method: http.MethodGet, path: "/api/persons/" + testID, wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "ancestors need a session", method: http.MethodGet, path: "/api/persons/" + testID + "/ancestors", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "descendants need a session", method: http.MethodGet, path: "/api/persons/" + testID + "/descendants", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "the tree needs a session", method: http.MethodGet, path: "/api/tree", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "home needs a session", method: http.MethodGet, path: "/api/home", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "the family summary needs a session", method: http.MethodGet, path: "/api/family/summary", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "contacts need a session", method: http.MethodGet, path: "/api/persons/" + testID + "/contacts", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "the directory needs a session", method: http.MethodGet, path: "/api/directory", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "my contacts need a session", method: http.MethodGet, path: "/api/me/contacts", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "contact requests need a session", method: http.MethodGet, path: "/api/me/contact-requests", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "requesting access needs no session", method: http.MethodPost, path: "/api/access-requests", wantStatus: http.StatusUnsupportedMediaType, wantType: "application/json"},
		{name: "suggesting needs a session", method: http.MethodPost, path: "/api/me/suggestions", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "the requests queue needs a session", method: http.MethodGet, path: "/api/admin/access-requests", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "adding a person needs a session", method: http.MethodPost, path: "/api/persons", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
		{name: "linking people needs a session", method: http.MethodPost, path: "/api/relationships", wantStatus: http.StatusUnauthorized, wantType: "application/json"},
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
			if strings.HasPrefix(tt.path, "/api/") {
				assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "API answers are never cached")
			}
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
		{name: "access requests, same client", path: "/api/access-requests", wantStatus: http.StatusTooManyRequests},
		{name: "access requests and forgot have their own limits", path: "/api/access-requests", secondPath: "/api/auth/password/forgot", wantStatus: http.StatusUnsupportedMediaType},
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

func TestFamilyWritesAreAdminOnly(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		role       string
		wantStatus int
	}{
		{name: "a member can't add", method: http.MethodPost, path: "/api/persons", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member reaches the edit, which checks whose it is", method: http.MethodPatch, path: "/api/persons/" + testID, role: auth.RoleMember, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member can't delete", method: http.MethodDelete, path: "/api/persons/" + testID, role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't restore", method: http.MethodPost, path: "/api/persons/" + testID + "/restore", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can read a profile", method: http.MethodGet, path: "/api/persons/42", role: auth.RoleMember, wantStatus: http.StatusNotFound},
		{name: "a member can read ancestors", method: http.MethodGet, path: "/api/persons/42/ancestors", role: auth.RoleMember, wantStatus: http.StatusNotFound},
		{name: "a member can read descendants", method: http.MethodGet, path: "/api/persons/42/descendants", role: auth.RoleMember, wantStatus: http.StatusNotFound},
		{name: "an admin can add", method: http.MethodPost, path: "/api/persons", role: auth.RoleAdmin, wantStatus: http.StatusUnsupportedMediaType},
		{name: "an admin can edit", method: http.MethodPatch, path: "/api/persons/" + testID, role: auth.RoleAdmin, wantStatus: http.StatusUnsupportedMediaType},
		{name: "an admin can delete", method: http.MethodDelete, path: "/api/persons/42", role: auth.RoleAdmin, wantStatus: http.StatusNotFound},
		{name: "an admin can restore", method: http.MethodPost, path: "/api/persons/42/restore", role: auth.RoleAdmin, wantStatus: http.StatusNotFound},
		{name: "a member can read contacts", method: http.MethodGet, path: "/api/persons/42/contacts", role: auth.RoleMember, wantStatus: http.StatusNotFound},
		{name: "a member reaches their own contacts", method: http.MethodPost, path: "/api/me/contacts", role: auth.RoleMember, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member reaches an edit of theirs", method: http.MethodPatch, path: "/api/me/contacts/" + testID, role: auth.RoleMember, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member can't add for someone", method: http.MethodPost, path: "/api/persons/" + testID + "/contacts", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't edit someone's", method: http.MethodPatch, path: "/api/persons/" + testID + "/contacts/" + testID, role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't delete someone's", method: http.MethodDelete, path: "/api/persons/" + testID + "/contacts/" + testID, role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "an admin can add for someone", method: http.MethodPost, path: "/api/persons/" + testID + "/contacts", role: auth.RoleAdmin, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member can ask for contact details", method: http.MethodPost, path: "/api/persons/" + testID + "/contact-requests", role: auth.RoleMember, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member can answer a request", method: http.MethodPost, path: "/api/me/contact-requests/42/share", role: auth.RoleMember, wantStatus: http.StatusNotFound},
		{name: "a member can suggest a relative", method: http.MethodPost, path: "/api/me/suggestions", role: auth.RoleMember, wantStatus: http.StatusUnsupportedMediaType},
		{name: "a member can't see access requests", method: http.MethodGet, path: "/api/admin/access-requests", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't decline one", method: http.MethodPost, path: "/api/admin/access-requests/" + testID + "/decline", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't reopen one", method: http.MethodPost, path: "/api/admin/access-requests/" + testID + "/reopen", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "an admin can decline one", method: http.MethodPost, path: "/api/admin/access-requests/42/decline", role: auth.RoleAdmin, wantStatus: http.StatusNotFound},
		{name: "a member can't link people", method: http.MethodPost, path: "/api/relationships", role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't edit a link", method: http.MethodPatch, path: "/api/relationships/" + testID, role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "a member can't delete a link", method: http.MethodDelete, path: "/api/relationships/" + testID, role: auth.RoleMember, wantStatus: http.StatusForbidden},
		{name: "an admin can link people", method: http.MethodPost, path: "/api/relationships", role: auth.RoleAdmin, wantStatus: http.StatusUnsupportedMediaType},
		{name: "an admin can edit a link", method: http.MethodPatch, path: "/api/relationships/" + testID, role: auth.RoleAdmin, wantStatus: http.StatusUnsupportedMediaType},
		{name: "an admin can delete a link", method: http.MethodDelete, path: "/api/relationships/42", role: auth.RoleAdmin, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := mocks.NewSessionAuthenticator(t)
			sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(auth.Session{UserID: uuid.New(), Role: tt.role, PersonID: new(uuid.New())}, nil)
			routes := newRoutesWith(t, sessions, 100, false)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func newRoutes(t *testing.T, perHour int, trustedProxy bool) http.Handler {
	t.Helper()
	return newRoutesWith(t, mocks.NewSessionAuthenticator(t), perHour, trustedProxy)
}

func newRoutesWith(t *testing.T, sessions handler.SessionAuthenticator, perHour int, trustedProxy bool) http.Handler {
	t.Helper()
	cookie := handler.NewSessionCookie(true)
	return router.New(
		router.Handlers{
			// Unreachable, so /ready answers 503 without needing Postgres.
			Health:   handler.NewHealth(testdb.UnreachablePool(t)),
			Auth:     handler.NewAuth(mocks.NewAuthService(t), mocks.NewResetRequester(t), cookie),
			Admin:    handler.NewAdmin(mocks.NewAdminService(t)),
			Family:   handler.NewFamily(mocks.NewFamilyService(t)),
			Contacts: handler.NewContacts(mocks.NewContactsService(t)),
			Access:   handler.NewAccess(mocks.NewAccessService(t)),
			Home:     handler.NewHome(mocks.NewHomeService(t), time.UTC),
		},
		router.Middleware{
			RequireSession: handler.NewSessionGate(sessions, cookie).Require,
			LimitLogin:     httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitForgot:    httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitReset:     httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitPreview:   httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			LimitAccess:    httpx.LimitByIP(ratelimit.New(perHour, time.Hour)),
			TrustedProxy:   trustedProxy,
		},
	)
}
