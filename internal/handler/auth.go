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
	SetPasswordWithToken(ctx context.Context, in auth.SetPasswordInput) (string, auth.User, error)
	PreviewLink(ctx context.Context, token string) (auth.LinkPreview, error)
	ChangePassword(ctx context.Context, in auth.ChangePasswordInput) (string, error)
	Logout(ctx context.Context, token string) error
	User(ctx context.Context, id uuid.UUID) (auth.User, error)
}

type ResetRequester interface {
	Request(ctx context.Context, email string)
}

type Auth struct {
	svc    AuthService
	resets ResetRequester
	cookie SessionCookie
}

func NewAuth(svc AuthService, resets ResetRequester, cookie SessionCookie) *Auth {
	return &Auth{svc: svc, resets: resets, cookie: cookie}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type previewRequest struct {
	Token string `json:"token"`
}

type previewResponse struct {
	Purpose          string  `json:"purpose"`
	InviteeEmail     string  `json:"invitee_email"`
	InviteeFirstName *string `json:"invitee_first_name"`
	FromName         *string `json:"from_name"`
}

type userResponse struct {
	ID       uuid.UUID  `json:"id"`
	Email    string     `json:"email"`
	Role     string     `json:"role"`
	PersonID *uuid.UUID `json:"person_id"`
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	in := auth.LoginInput{
		Email: req.Email, Password: req.Password, UserAgent: r.UserAgent(), ClientIP: httpx.ClientIP(r), ReplacesToken: a.cookieToken(r),
	}
	token, user, err := a.svc.Login(r.Context(), in)
	var limited *auth.RateLimitedError
	switch {
	case err == nil:
		// Even for a client that just left: the session exists and replaced the old one.
		a.cookie.set(w, token)
		httpx.JSON(w, http.StatusOK, toUserResponse(user))
	case clientGone(r):
	case errors.As(err, &limited):
		httpx.TooManyRequests(w, limited.RetryAfter)
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, "login")
	}
}

// ForgotPassword answers the same whether or not the email has an account (§5).
func (a *Auth) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	a.resets.Request(r.Context(), req.Email)
	w.WriteHeader(http.StatusAccepted)
}

// ResetPassword uses an invite or password-reset link's token to set the password and sign in.
func (a *Auth) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	in := auth.SetPasswordInput{Token: req.Token, Password: req.Password, UserAgent: r.UserAgent(), ReplacesToken: a.cookieToken(r)}
	token, user, err := a.svc.SetPasswordWithToken(r.Context(), in)
	refused := passwordRefusedCode(err)
	switch {
	case err == nil:
		a.cookie.set(w, token)
		httpx.JSON(w, http.StatusOK, toUserResponse(user))
	case clientGone(r):
	case errors.Is(err, auth.ErrInvalidToken):
		invalidToken(w)
	case refused != "":
		httpx.Error(w, http.StatusBadRequest, refused, err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, "reset password")
	}
}

// PreviewLink tells the set-password page whether its link is an invite or a reset; the token is in the URL fragment, so the page can't GET it.
func (a *Auth) PreviewLink(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	preview, err := a.svc.PreviewLink(r.Context(), req.Token)
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, previewResponse{
			Purpose: preview.Purpose, InviteeEmail: preview.Email, InviteeFirstName: preview.InviteeFirstName, FromName: preview.FromName,
		})
	case clientGone(r):
	case errors.Is(err, auth.ErrInvalidToken):
		invalidToken(w)
	default:
		internalError(w, r, err, "preview link")
	}
}

// ChangePassword answers with a new session cookie: every old session, this one included, is gone.
func (a *Auth) ChangePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "change password mounted without RequireSession")
		return
	}
	var req changePasswordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	in := auth.ChangePasswordInput{
		UserID: session.UserID, SessionToken: a.cookieToken(r), CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword, UserAgent: r.UserAgent(),
	}
	token, err := a.svc.ChangePassword(r.Context(), in)
	refused := passwordRefusedCode(err)
	var limited *auth.RateLimitedError
	switch {
	case err == nil:
		// Even for a client that just left: its old session is gone either way.
		a.cookie.set(w, token)
		w.WriteHeader(http.StatusNoContent)
	case clientGone(r):
	case errors.As(err, &limited):
		httpx.TooManyRequests(w, limited.RetryAfter)
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Error(w, http.StatusBadRequest, "wrong_password", "current password is incorrect")
	case refused != "":
		httpx.Error(w, http.StatusBadRequest, refused, err.Error())
	case errors.Is(err, auth.ErrNoSession):
		// Not cleared: a double-submit's winner may have just set a newer cookie.
		unauthorized(w)
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, "change password")
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

// passwordRefusedCode names the rule a refused password broke, so the page can say which in plain words;
// "" when err isn't a refusal.
func passwordRefusedCode(err error) string {
	switch {
	case errors.Is(err, auth.ErrPasswordTooShort):
		return "password_too_short"
	case errors.Is(err, auth.ErrPasswordTooLong):
		return "password_too_long"
	case errors.Is(err, auth.ErrPasswordTooCommon):
		return "password_too_common"
	default:
		return ""
	}
}

func invalidToken(w http.ResponseWriter) {
	httpx.Error(w, http.StatusBadRequest, "invalid_token", "this link is invalid, used or expired; ask for a new one")
}

func busy(w http.ResponseWriter) {
	httpx.Error(w, http.StatusServiceUnavailable, "busy", "try again in a moment")
}

func (a *Auth) cookieToken(r *http.Request) string {
	if c, err := r.Cookie(a.cookie.name); err == nil {
		return c.Value
	}
	return ""
}

func toUserResponse(u auth.User) userResponse {
	return userResponse(u)
}
