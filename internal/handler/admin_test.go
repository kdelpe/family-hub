package handler_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestListUsers(t *testing.T) {
	expires := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	page := admin.Page{
		Accounts: []admin.Account{
			{ID: testUser.ID, Email: "test@example.com", Role: "member", Status: admin.StatusInvited, InviteExpiresAt: &expires, CreatedAt: expires.AddDate(0, 0, -7)},
		},
		NextCursor: "next",
	}
	const pageBody = `{"users":[{"id":"01890000-0000-7000-8000-000000000001","email":"test@example.com","role":"member","status":"invited",
		"invite_expires_at":"2026-11-21T12:00:00Z","disabled_at":null,"last_login_at":null,"created_at":"2026-11-14T12:00:00Z"}],"next_cursor":"next"}`

	tests := []struct {
		name       string
		query      string
		wantIn     *admin.ListInput
		page       admin.Page
		listErr    error
		canceled   bool
		wantStatus int
		wantBody   string
	}{
		{name: "a page", query: "?limit=1&cursor=abc", wantIn: &admin.ListInput{Limit: 1, Cursor: "abc"}, page: page, wantStatus: http.StatusOK, wantBody: pageBody},
		{name: "the last page", wantIn: &admin.ListInput{}, wantStatus: http.StatusOK, wantBody: `{"users":[],"next_cursor":null}`},
		{name: "limit not a number", query: "?limit=ten", wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "limit zero", query: "?limit=0", wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "limit out of range", query: "?limit=500", wantIn: &admin.ListInput{Limit: 500}, listErr: admin.ErrInvalidPageSize, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "bad cursor", query: "?cursor=x", wantIn: &admin.ListInput{Cursor: "x"}, listErr: admin.ErrInvalidCursor, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"invalid cursor"}}`},
		{name: "database error", wantIn: &admin.ListInput{}, listErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", wantIn: &admin.ListInput{}, listErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAdminService(t)
			if tt.wantIn != nil {
				svc.EXPECT().List(mock.Anything, *tt.wantIn).Return(tt.page, tt.listErr)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/admin/users"+tt.query, nil)
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewAdmin(svc).ListUsers(rec, req)

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
