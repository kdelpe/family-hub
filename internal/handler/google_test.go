package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
)

type googleServiceStub struct {
	startHandle string
	startURL    string
	startErr    error
	completeIn  auth.GoogleCallbackInput
	completeErr error
	completeTok string
}

func (s *googleServiceStub) Start(context.Context) (string, string, error) {
	return s.startHandle, s.startURL, s.startErr
}

func (s *googleServiceStub) Complete(_ context.Context, in auth.GoogleCallbackInput) (string, auth.User, error) {
	s.completeIn = in
	return s.completeTok, testUser, s.completeErr
}

func TestGoogleStartSetsFlowCookieAndRedirects(t *testing.T) {
	google := &googleServiceStub{startHandle: "opaque", startURL: "https://accounts.google.com/authorize"}
	a := handler.NewAuth(nil, nil, handler.NewSessionCookie(true), google)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil)

	a.GoogleStart(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, google.startURL, rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "fh_oauth", cookies[0].Name)
	assert.Equal(t, "opaque", cookies[0].Value)
	assert.Equal(t, "/api/auth/google", cookies[0].Path)
	assert.Equal(t, 600, cookies[0].MaxAge)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
}

func TestGoogleCallbackConsumesFlowCookieAndSetsSession(t *testing.T) {
	google := &googleServiceStub{completeTok: "session-token"}
	a := handler.NewAuth(nil, nil, handler.NewSessionCookie(true), google)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=auth-code&state=state-value", nil)
	req.Header.Set("User-Agent", "browser")
	req.AddCookie(&http.Cookie{Name: "fh_oauth", Value: "opaque"})
	req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "old-session"})

	a.GoogleCallback(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Equal(t, auth.GoogleCallbackInput{
		Handle: "opaque", State: "state-value", Code: "auth-code", UserAgent: "browser", ReplacesToken: "old-session",
	}, google.completeIn)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 2)
	assert.Equal(t, "fh_oauth", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.Equal(t, "__Host-fh_session", cookies[1].Name)
	assert.Equal(t, "session-token", cookies[1].Value)
}

func TestGoogleCallbackRefusesProviderErrorAndClearsFlowCookie(t *testing.T) {
	google := &googleServiceStub{completeErr: auth.ErrGoogleAccess}
	a := handler.NewAuth(nil, nil, handler.NewSessionCookie(false), google)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?error=access_denied&state=state-value", nil)
	req.AddCookie(&http.Cookie{Name: "fh_oauth", Value: "opaque"})

	a.GoogleCallback(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login?google=unavailable", rec.Header().Get("Location"))
	assert.Empty(t, google.completeIn.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "fh_oauth", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.False(t, cookies[0].Secure)
}

func TestGoogleCallbackDoesNotExposeUnexpectedErrors(t *testing.T) {
	google := &googleServiceStub{completeErr: errors.New("database failure")}
	a := handler.NewAuth(nil, nil, handler.NewSessionCookie(true), google)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=code&state=state", nil)
	req.AddCookie(&http.Cookie{Name: "fh_oauth", Value: "opaque"})

	a.GoogleCallback(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login?google=unavailable", rec.Header().Get("Location"))
	assert.NotContains(t, rec.Body.String(), "database failure")
}
