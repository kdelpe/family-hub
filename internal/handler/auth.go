package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type AuthService interface {
	Login(ctx context.Context, email, password, userAgent string) (string, auth.User, error)
	Logout(ctx context.Context, token string) error
	User(ctx context.Context, id uuid.UUID) (auth.User, error)
}

type Auth struct {
	svc    AuthService
	cookie SessionCookie
}

func NewAuth(svc AuthService, cookie SessionCookie) *Auth {
	return &Auth{svc: svc, cookie: cookie}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	token, user, err := a.svc.Login(r.Context(), req.Email, req.Password, r.UserAgent())
	if err == nil {
		a.endPreviousSession(r)
	}
	switch {
	case clientGone(r):
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, auth.ErrBusy):
		httpx.Error(w, http.StatusServiceUnavailable, "busy", "try again in a moment")
	case err != nil:
		internalError(w, r, err, "login")
	default:
		a.cookie.set(w, token)
		httpx.JSON(w, http.StatusOK, toUserResponse(user))
	}
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(a.cookie.name); err == nil {
		if err := a.svc.Logout(r.Context(), c.Value); err != nil {
			if !clientGone(r) {
				internalError(w, r, err, "logout")
			}
			return
		}
	}
	a.cookie.clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	session, _ := CurrentSession(r.Context())
	user, err := a.svc.User(r.Context(), session.UserID)
	switch {
	case clientGone(r):
	case errors.Is(err, auth.ErrNoSession):
		unauthorized(w)
	case err != nil:
		internalError(w, r, err, "get current user")
	default:
		httpx.JSON(w, http.StatusOK, toUserResponse(user))
	}
}

// A new sign-in replaces the browser's cookie, so the old session must not outlive it.
func (a *Auth) endPreviousSession(r *http.Request) {
	c, err := r.Cookie(a.cookie.name)
	if err != nil {
		return
	}
	if err := a.svc.Logout(r.Context(), c.Value); err != nil && !clientGone(r) {
		zerolog.Ctx(r.Context()).Warn().Err(err).Msg("end previous session")
	}
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, Role: u.Role}
}
