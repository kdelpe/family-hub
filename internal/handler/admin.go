package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type AdminService interface {
	Invite(ctx context.Context, in admin.InviteInput) (auth.User, error)
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
	case errors.Is(err, admin.ErrInviteNotSent):
		zerolog.Ctx(r.Context()).Error().Err(err).Msg("send invite")
		httpx.Error(w, http.StatusBadGateway, "mail_failed", "the invitation email couldn't be sent and nothing was saved; try again")
	default:
		internalError(w, r, err, "invite user")
	}
}
