package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/httpx"
)

const (
	secureSessionCookie = "__Host-fh_session"
	// Safari refuses Secure cookies on http://localhost, and __Host- requires Secure.
	insecureSessionCookie = "fh_session"
)

type SessionCookie struct {
	name   string
	secure bool
}

func NewSessionCookie(secure bool) SessionCookie {
	if secure {
		return SessionCookie{name: secureSessionCookie, secure: true}
	}
	return SessionCookie{name: insecureSessionCookie}
}

func (c SessionCookie) set(w http.ResponseWriter, token string) {
	http.SetCookie(w, c.cookie(token, int(auth.SessionLifetime.Seconds())))
}

func (c SessionCookie) clear(w http.ResponseWriter) {
	http.SetCookie(w, c.cookie("", -1))
}

func (c SessionCookie) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: c.name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	}
}

type SessionAuthenticator interface {
	Authenticate(ctx context.Context, token string) (auth.Session, error)
}

type SessionGate struct {
	sessions SessionAuthenticator
	cookie   SessionCookie
}

func NewSessionGate(sessions SessionAuthenticator, cookie SessionCookie) *SessionGate {
	return &SessionGate{sessions: sessions, cookie: cookie}
}

func (g *SessionGate) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(g.cookie.name)
		if err != nil {
			unauthorized(w)
			return
		}
		session, err := g.sessions.Authenticate(r.Context(), cookie.Value)
		if clientGone(r) {
			return
		}
		if errors.Is(err, auth.ErrNoSession) {
			unauthorized(w)
			return
		}
		if err != nil {
			internalError(w, r, err, "authenticate session")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, session)))
	})
}

// RequireAdmin goes after SessionGate.Require, which reads the role afresh on every request.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := CurrentSession(r.Context())
		if !ok {
			internalError(w, r, errors.New("no session in context"), "RequireAdmin mounted without RequireSession")
			return
		}
		if session.Role != auth.RoleAdmin {
			httpx.Error(w, http.StatusForbidden, "forbidden", "admins only")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type sessionKey struct{}

func CurrentSession(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(auth.Session)
	return s, ok
}

func unauthorized(w http.ResponseWriter) {
	httpx.Error(w, http.StatusUnauthorized, "unauthorized", "sign in required")
}

func clientGone(r *http.Request) bool {
	return errors.Is(r.Context().Err(), context.Canceled)
}

func internalError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	zerolog.Ctx(r.Context()).Error().Err(err).Msg(msg)
	httpx.Error(w, http.StatusInternalServerError, "internal", "internal error")
}
