// Package router is the only place routes and middleware are wired (architecture §4, §6.1).
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/httpx"
	"github.com/kdelpe/family-hub/internal/webui"
)

const (
	livePath  = "/health"
	readyPath = "/ready"
)

type Handlers struct {
	Health *handler.Health
	Auth   *handler.Auth
}

type Middleware struct {
	RequireSession func(http.Handler) http.Handler
}

func New(h Handlers, mw Middleware) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	// Both outside Recover: its 500 keeps the headers set before it, and the logger records that 500.
	r.Use(httpx.SecurityHeaders)
	r.Use(httpx.RequestLogger(livePath, readyPath))
	r.Use(httpx.Recover)
	r.Use(middleware.GetHead)

	r.Get(livePath, h.Health.Live)
	r.Get(readyPath, h.Health.Ready)

	r.Route("/api", func(r chi.Router) {
		r.Use(http.NewCrossOriginProtection().Handler)
		r.NotFound(httpx.NotFound)
		r.MethodNotAllowed(httpx.MethodNotAllowed)

		r.Post("/auth/login", h.Auth.Login)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireSession)
			r.Get("/auth/me", h.Auth.Me)
			r.Post("/auth/logout", h.Auth.Logout)
		})
	})

	// Last, so any other GET is the app's, including its client-side routes.
	r.Get("/*", webui.Handler().ServeHTTP)
	return r
}
