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

type SessionAuthenticator interface {
	Authenticate(ctx context.Context, token string) (auth.Session, error)
}

type SessionGate struct {
	sessions   SessionAuthenticator
	cookieName string
}

func NewSessionGate(sessions SessionAuthenticator, secureCookie bool) *SessionGate {
	name := insecureSessionCookie
	if secureCookie {
		name = secureSessionCookie
	}
	return &SessionGate{sessions: sessions, cookieName: name}
}

func (g *SessionGate) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(g.cookieName)
		if err != nil {
			unauthorized(w)
			return
		}
		session, err := g.sessions.Authenticate(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrNoSession) {
			unauthorized(w)
			return
		}
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			zerolog.Ctx(r.Context()).Error().Err(err).Msg("authenticate session")
			httpx.Error(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, session)))
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
