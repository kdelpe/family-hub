package handler_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

func TestInviteUser(t *testing.T) {
	const body = `{"email":"test@example.com","role":"member"}`

	tests := []struct {
		name       string
		body       string
		inviteErr  error
		wantInvite bool
		canceled   bool
		wantStatus int
		wantBody   string
	}{
		{name: "invited", body: body, wantInvite: true, wantStatus: http.StatusCreated, wantBody: testUserBody},
		{name: "invalid email", body: body, wantInvite: true, inviteErr: admin.ErrInvalidEmail, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"not a valid email address"}}`},
		{name: "invalid role", body: body, wantInvite: true, inviteErr: admin.ErrInvalidRole, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"role must be admin or member"}}`},
		{name: "email taken", body: body, wantInvite: true, inviteErr: admin.ErrEmailTaken, wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"email_taken","message":"a user with this email already exists"}}`},
		{name: "email not sent", body: body, wantInvite: true, inviteErr: fmt.Errorf("%w: smtp: 550", admin.ErrInviteNotSent), wantStatus: http.StatusBadGateway,
			wantBody: `{"error":{"code":"mail_failed","message":"the invitation email couldn't be sent and nothing was saved; try again"}}`},
		{name: "database error", body: body, wantInvite: true, inviteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", body: body, wantInvite: true, inviteErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
		{name: "a failed commit is reported after the client left", body: body, wantInvite: true, inviteErr: errors.New("commit invite: conn closed"), canceled: true,
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "unknown field", body: `{"email":"test@example.com","role":"member","password":"x"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAdminService(t)
			if tt.wantInvite {
				svc.EXPECT().Invite(mock.Anything, admin.InviteInput{Email: "test@example.com", Role: "member"}).Return(testUser, tt.inviteErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewAdmin(svc).InviteUser(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String())
				return
			}
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		session    *auth.Session
		wantStatus int
		wantBody   string
	}{
		{name: "admin passes", session: &auth.Session{UserID: uuid.New(), Role: auth.RoleAdmin}, wantStatus: http.StatusNoContent},
		{name: "member refused", session: &auth.Session{UserID: uuid.New(), Role: auth.RoleMember}, wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"admins only"}}`},
		{name: "mounted without a session gate", wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			sessions := mocks.NewSessionAuthenticator(t)
			h := handler.RequireAdmin(next)
			req := httptest.NewRequest(http.MethodPost, "/api/admin/users", nil)
			if tt.session != nil {
				sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(*tt.session, nil)
				h = handler.NewSessionGate(sessions, handler.NewSessionCookie(true)).Require(h)
				req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, rec.Body.String())
			}
		})
	}
}
