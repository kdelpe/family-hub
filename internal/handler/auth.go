package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type AuthService interface {
	Login(ctx context.Context, in auth.LoginInput) (string, auth.User, error)
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
	in := auth.LoginInput{Email: req.Email, Password: req.Password, UserAgent: r.UserAgent()}
	if c, err := r.Cookie(a.cookie.name); err == nil {
		in.ReplacesToken = c.Value
	}
	token, user, err := a.svc.Login(r.Context(), in)
	switch {
	case err == nil:
		// Even for a client that just left: the session exists and replaced the old one.
		a.cookie.set(w, token)
		httpx.JSON(w, http.StatusOK, toUserResponse(user))
	case clientGone(r):
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, auth.ErrBusy):
		httpx.Error(w, http.StatusServiceUnavailable, "busy", "try again in a moment")
	default:
		internalError(w, r, err, "login")
	}
}

// The cookie is cleared even when deleting the session fails, so this browser is signed out either way.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	a.cookie.clear(w)
	c, err := r.Cookie(a.cookie.name)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch err := a.svc.Logout(r.Context(), c.Value); {
	case clientGone(r):
	case err != nil:
		internalError(w, r, err, "logout")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "me mounted without RequireSession")
		return
	}
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

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, Role: u.Role}
}
