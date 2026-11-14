package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type AdminService interface {
	Invite(ctx context.Context, in admin.InviteInput) (auth.User, error)
	List(ctx context.Context, in admin.ListInput) (admin.Page, error)
	Update(ctx context.Context, in admin.UpdateInput) (admin.Account, error)
	SendReset(ctx context.Context, id uuid.UUID) error
}

type Admin struct {
	svc AdminService
}

func NewAdmin(svc AdminService) *Admin {
	return &Admin{svc: svc}
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type accountResponse struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	InviteExpiresAt *time.Time `json:"invite_expires_at"`
	DisabledAt      *time.Time `json:"disabled_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type pageResponse struct {
	Users      []accountResponse `json:"users"`
	NextCursor *string           `json:"next_cursor"`
}

func (a *Admin) ListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	in := admin.ListInput{Limit: admin.DefaultPageSize, Cursor: query.Get("cursor")}
	if raw := query.Get("limit"); raw != "" {
		// Not a number becomes 0, which List refuses like any other bad limit.
		in.Limit, _ = strconv.Atoi(raw)
	}
	page, err := a.svc.List(r.Context(), in)
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, toPageResponse(page))
	case clientGone(r):
	case errors.Is(err, admin.ErrInvalidPageSize), errors.Is(err, admin.ErrInvalidCursor):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		internalError(w, r, err, "list users")
	}
}

type updateUserRequest struct {
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	Disabled *bool   `json:"disabled"`
}

func (a *Admin) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var req updateUserRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	account, err := a.svc.Update(r.Context(), admin.UpdateInput{ID: id, Email: req.Email, Role: req.Role, Disabled: req.Disabled})
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, toAccountResponse(account))
	// Only a plain cancel is dropped: a failed commit after the email went out must still be logged.
	case clientGone(r) && errors.Is(err, context.Canceled):
	case errors.Is(err, admin.ErrNoChanges), errors.Is(err, admin.ErrInvalidEmail), errors.Is(err, admin.ErrInvalidRole),
		errors.Is(err, admin.ErrDisabledEmailChange):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, admin.ErrUserNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, admin.ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, admin.ErrLastAdmin):
		httpx.Error(w, http.StatusConflict, "last_admin", err.Error())
	case errors.Is(err, admin.ErrEmailNotSent):
		zerolog.Ctx(r.Context()).Error().Err(err).Msg("send invite to the new email")
		httpx.Error(w, http.StatusBadGateway, "mail_failed", "the invitation to the new email couldn't be sent and nothing was changed; try again")
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, "update user")
	}
}

// SendReset answers 204 once the email is out: a new invite or a password reset, as the account needs.
func (a *Admin) SendReset(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	err := a.svc.SendReset(r.Context(), id)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	// Only a plain cancel is dropped: a send that failed after the commit must still be logged.
	case clientGone(r) && errors.Is(err, context.Canceled):
	case errors.Is(err, admin.ErrUserNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, admin.ErrEmailNotSent):
		zerolog.Ctx(r.Context()).Error().Err(err).Msg("send reset")
		httpx.Error(w, http.StatusBadGateway, "mail_failed", "the email couldn't be sent; try again")
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, "send reset")
	}
}

func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", admin.ErrUserNotFound.Error())
		return uuid.Nil, false
	}
	return id, true
}

func toPageResponse(page admin.Page) pageResponse {
	resp := pageResponse{Users: make([]accountResponse, 0, len(page.Accounts))}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	for _, acc := range page.Accounts {
		resp.Users = append(resp.Users, toAccountResponse(acc))
	}
	return resp
}

func toAccountResponse(acc admin.Account) accountResponse {
	return accountResponse{
		ID: acc.ID, Email: acc.Email, Role: acc.Role, Status: acc.Status, InviteExpiresAt: acc.InviteExpiresAt,
		DisabledAt: acc.DisabledAt, LastLoginAt: acc.LastLoginAt, CreatedAt: acc.CreatedAt,
	}
}

func (a *Admin) InviteUser(w http.ResponseWriter, r *http.Request) {
	var req inviteRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	user, err := a.svc.Invite(r.Context(), admin.InviteInput{Email: req.Email, Role: req.Role})
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusCreated, toUserResponse(user))
	// Only a plain cancel is dropped: a failed commit after the email went out must still be logged.
	case clientGone(r) && errors.Is(err, context.Canceled):
	case errors.Is(err, admin.ErrInvalidEmail), errors.Is(err, admin.ErrInvalidRole):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, admin.ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, admin.ErrEmailNotSent):
		zerolog.Ctx(r.Context()).Error().Err(err).Msg("send invite")
		httpx.Error(w, http.StatusBadGateway, "mail_failed", "the invitation email couldn't be sent and nothing was saved; try again")
	default:
		internalError(w, r, err, "invite user")
	}
}
