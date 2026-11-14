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

	"github.com/go-chi/chi/v5"
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
		{name: "email not sent", body: body, wantInvite: true, inviteErr: fmt.Errorf("%w: smtp: 550", admin.ErrEmailNotSent), wantStatus: http.StatusBadGateway,
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
		{name: "the last page, at the default size", wantIn: &admin.ListInput{Limit: admin.DefaultPageSize}, wantStatus: http.StatusOK, wantBody: `{"users":[],"next_cursor":null}`},
		{name: "limit not a number", query: "?limit=ten", wantIn: &admin.ListInput{}, listErr: admin.ErrInvalidPageSize, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "limit zero", query: "?limit=0", wantIn: &admin.ListInput{}, listErr: admin.ErrInvalidPageSize, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "limit out of range", query: "?limit=500", wantIn: &admin.ListInput{Limit: 500}, listErr: admin.ErrInvalidPageSize, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"limit must be between 1 and 100"}}`},
		{name: "bad cursor", query: "?cursor=x", wantIn: &admin.ListInput{Limit: admin.DefaultPageSize, Cursor: "x"}, listErr: admin.ErrInvalidCursor, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"invalid cursor"}}`},
		{name: "database error", wantIn: &admin.ListInput{Limit: admin.DefaultPageSize}, listErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", wantIn: &admin.ListInput{Limit: admin.DefaultPageSize}, listErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
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

func TestUpdateUser(t *testing.T) {
	const body = `{"role":"admin","disabled":false}`
	role, enabled := "admin", false
	updated := admin.Account{ID: testUser.ID, Email: "test@example.com", Role: "admin", Status: admin.StatusActive, CreatedAt: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	const updatedBody = `{"id":"01890000-0000-7000-8000-000000000001","email":"test@example.com","role":"admin","status":"active",
		"invite_expires_at":null,"disabled_at":null,"last_login_at":null,"created_at":"2026-11-14T12:00:00Z"}`

	tests := []struct {
		name       string
		id         string
		body       string
		updateErr  error
		wantUpdate bool
		canceled   bool
		wantStatus int
		wantBody   string
	}{
		{name: "updated", body: body, wantUpdate: true, wantStatus: http.StatusOK, wantBody: updatedBody},
		{name: "nothing to change", body: body, wantUpdate: true, updateErr: admin.ErrNoChanges, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"nothing to change"}}`},
		{name: "invalid role", body: body, wantUpdate: true, updateErr: admin.ErrInvalidRole, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"role must be admin or member"}}`},
		{name: "a disabled user's email", body: body, wantUpdate: true, updateErr: admin.ErrDisabledEmailChange, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"a disabled user's email can't be changed; enable them first"}}`},
		{name: "unknown user", body: body, wantUpdate: true, updateErr: admin.ErrUserNotFound, wantStatus: http.StatusNotFound,
			wantBody: `{"error":{"code":"not_found","message":"user not found"}}`},
		{name: "not a user id", id: "42", body: body, wantStatus: http.StatusNotFound, wantBody: `{"error":{"code":"not_found","message":"user not found"}}`},
		{name: "email taken", body: body, wantUpdate: true, updateErr: admin.ErrEmailTaken, wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"email_taken","message":"a user with this email already exists"}}`},
		{name: "the last admin", body: body, wantUpdate: true, updateErr: admin.ErrLastAdmin, wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"last_admin","message":"the last admin who can sign in can't be demoted, disabled or given a new email"}}`},
		{name: "invite not sent", body: body, wantUpdate: true, updateErr: fmt.Errorf("%w: smtp: 550", admin.ErrEmailNotSent), wantStatus: http.StatusBadGateway,
			wantBody: `{"error":{"code":"mail_failed","message":"the invitation to the new email couldn't be sent and nothing was changed; try again"}}`},
		{name: "busy", body: body, wantUpdate: true, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", body: body, wantUpdate: true, updateErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", body: body, wantUpdate: true, updateErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
		{name: "a failed commit is reported after the client left", body: body, wantUpdate: true, updateErr: errors.New("commit: conn closed"), canceled: true,
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "unknown field", body: `{"role":"admin","password":"x"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAdminService(t)
			if tt.wantUpdate {
				in := admin.UpdateInput{ID: testUser.ID, Role: &role, Disabled: &enabled}
				svc.EXPECT().Update(mock.Anything, in).Return(updated, tt.updateErr)
			}
			id := testUser.ID.String()
			if tt.id != "" {
				id = tt.id
			}
			req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+id, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParam(req, "id", id)
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewAdmin(svc).UpdateUser(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String())
				return
			}
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestSendReset(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		resetErr   error
		wantReset  bool
		canceled   bool
		wantStatus int
		wantBody   string
	}{
		{name: "sent", wantReset: true, wantStatus: http.StatusNoContent},
		{name: "unknown or disabled user", wantReset: true, resetErr: admin.ErrUserNotFound, wantStatus: http.StatusNotFound,
			wantBody: `{"error":{"code":"not_found","message":"user not found"}}`},
		{name: "not a user id", id: "42", wantStatus: http.StatusNotFound, wantBody: `{"error":{"code":"not_found","message":"user not found"}}`},
		{name: "email not sent", wantReset: true, resetErr: fmt.Errorf("%w: smtp: 550", admin.ErrEmailNotSent), wantStatus: http.StatusBadGateway,
			wantBody: `{"error":{"code":"mail_failed","message":"the email couldn't be sent; try again"}}`},
		{name: "busy", wantReset: true, resetErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "database error", wantReset: true, resetErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "client gone", wantReset: true, resetErr: context.Canceled, canceled: true, wantStatus: http.StatusOK},
		{name: "a failed commit is reported after the client left", wantReset: true, resetErr: errors.New("commit: conn closed"), canceled: true,
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAdminService(t)
			if tt.wantReset {
				svc.EXPECT().SendReset(mock.Anything, testUser.ID).Return(tt.resetErr)
			}
			id := testUser.ID.String()
			if tt.id != "" {
				id = tt.id
			}
			req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/admin/users/"+id+"/send-reset", nil), "id", id)
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.NewAdmin(svc).SendReset(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody == "" {
				assert.Empty(t, rec.Body.String())
				return
			}
			assert.JSONEq(t, tt.wantBody, rec.Body.String())
		})
	}
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
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
