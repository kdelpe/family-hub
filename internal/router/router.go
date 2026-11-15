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
	Admin  *handler.Admin
}

type Middleware struct {
	RequireSession func(http.Handler) http.Handler
	LimitLogin     func(http.Handler) http.Handler
	LimitForgot    func(http.Handler) http.Handler
	LimitReset     func(http.Handler) http.Handler
	LimitPreview   func(http.Handler) http.Handler
	TrustedProxy   bool
}

func New(h Handlers, mw Middleware) http.Handler {
	if mw.RequireSession == nil || mw.LimitLogin == nil || mw.LimitForgot == nil || mw.LimitReset == nil || mw.LimitPreview == nil {
		panic("router: RequireSession and every Limit middleware are required")
	}
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	if mw.TrustedProxy {
		r.Use(httpx.ClientIPFromProxy)
	}
	// Both outside Recover: its 500 keeps the headers set before it, and the logger records that 500.
	r.Use(httpx.SecurityHeaders)
	r.Use(httpx.RequestLogger(livePath, readyPath))
	r.Use(httpx.Recover)
	r.Use(middleware.GetHead)

	r.Get(livePath, h.Health.Live)
	r.Get(readyPath, h.Health.Ready)

	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(httpx.CrossOriginRefused))

	r.Route("/api", func(r chi.Router) {
		r.Use(csrf.Handler)
		r.NotFound(httpx.NotFound)
		r.MethodNotAllowed(httpx.MethodNotAllowed)

		r.With(mw.LimitLogin).Post("/auth/login", h.Auth.Login)
		r.With(mw.LimitForgot).Post("/auth/password/forgot", h.Auth.ForgotPassword)
		r.With(mw.LimitReset).Post("/auth/password/reset", h.Auth.ResetPassword)
		r.With(mw.LimitPreview).Post("/auth/invite/preview", h.Auth.PreviewLink)
		// Outside the session group, so an expired session's cookie still gets cleared.
		r.Post("/auth/logout", h.Auth.Logout)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireSession)
			r.Get("/auth/me", h.Auth.Me)
			r.Post("/auth/password/change", h.Auth.ChangePassword)

			r.Group(func(r chi.Router) {
				r.Use(handler.RequireAdmin)
				r.Get("/admin/users", h.Admin.ListUsers)
				r.Post("/admin/users", h.Admin.InviteUser)
				r.Patch("/admin/users/{id}", h.Admin.UpdateUser)
				r.Post("/admin/users/{id}/send-reset", h.Admin.SendReset)
			})
		})
	})

	// Last, so any other GET is the app's, including its client-side routes.
	r.Get("/*", webui.Handler().ServeHTTP)
	return r
}
