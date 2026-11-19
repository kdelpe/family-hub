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
	Health   *handler.Health
	Auth     *handler.Auth
	Admin    *handler.Admin
	Family   *handler.Family
	Contacts *handler.Contacts
	Access   *handler.Access
}

type Middleware struct {
	RequireSession func(http.Handler) http.Handler
	LimitLogin     func(http.Handler) http.Handler
	LimitForgot    func(http.Handler) http.Handler
	LimitReset     func(http.Handler) http.Handler
	LimitPreview   func(http.Handler) http.Handler
	LimitAccess    func(http.Handler) http.Handler
	TrustedProxy   bool
}

func New(h Handlers, mw Middleware) http.Handler {
	if mw.RequireSession == nil || mw.LimitLogin == nil || mw.LimitForgot == nil || mw.LimitReset == nil || mw.LimitPreview == nil || mw.LimitAccess == nil {
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
		r.Use(httpx.NoStore)
		r.Use(csrf.Handler)
		r.NotFound(httpx.NotFound)
		r.MethodNotAllowed(httpx.MethodNotAllowed)

		r.With(mw.LimitLogin).Post("/auth/login", h.Auth.Login)
		r.With(mw.LimitForgot).Post("/auth/password/forgot", h.Auth.ForgotPassword)
		r.With(mw.LimitReset).Post("/auth/password/reset", h.Auth.ResetPassword)
		r.With(mw.LimitPreview).Post("/auth/invite/preview", h.Auth.PreviewLink)
		r.With(mw.LimitAccess).Post("/access-requests", h.Access.Submit)
		// Outside the session group, so an expired session's cookie still gets cleared.
		r.Post("/auth/logout", h.Auth.Logout)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireSession)
			r.Get("/auth/me", h.Auth.Me)
			r.Post("/auth/password/change", h.Auth.ChangePassword)
			r.Get("/persons", h.Family.SearchPersons)
			r.Get("/persons/{id}", h.Family.GetPerson)
			r.Get("/persons/{id}/ancestors", h.Family.Ancestors)
			r.Get("/persons/{id}/descendants", h.Family.Descendants)
			r.Get("/tree", h.Family.Tree)
			r.Get("/family/summary", h.Family.Summary)
			// Members may change their own bio; the service checks (§4).
			r.Patch("/persons/{id}", h.Family.UpdatePerson)
			r.Get("/persons/{id}/contacts", h.Contacts.ListFor)
			r.Get("/me/contacts", h.Contacts.ListMine)
			r.Post("/me/contacts", h.Contacts.CreateMine)
			r.Patch("/me/contacts/{cid}", h.Contacts.UpdateMine)
			r.Delete("/me/contacts/{cid}", h.Contacts.DeleteMine)
			r.Post("/persons/{id}/contact-requests", h.Contacts.Ask)
			r.Get("/me/contact-requests", h.Contacts.ListRequests)
			r.Post("/me/suggestions", h.Access.Suggest)
			r.Post("/me/contact-requests/{rid}/share", h.Contacts.Share)
			r.Post("/me/contact-requests/{rid}/decline", h.Contacts.Decline)
			r.Post("/me/contact-requests/{rid}/revoke", h.Contacts.Revoke)

			r.Group(func(r chi.Router) {
				r.Use(handler.RequireAdmin)
				r.Get("/admin/users", h.Admin.ListUsers)
				r.Post("/admin/users", h.Admin.InviteUser)
				r.Patch("/admin/users/{id}", h.Admin.UpdateUser)
				r.Post("/admin/users/{id}/send-reset", h.Admin.SendReset)
				r.Get("/admin/access-requests", h.Access.List)
				r.Post("/admin/access-requests/{id}/decline", h.Access.Decline)
				r.Post("/admin/access-requests/{id}/reopen", h.Access.Reopen)
				r.Post("/persons", h.Family.CreatePerson)
				r.Delete("/persons/{id}", h.Family.DeletePerson)
				r.Post("/persons/{id}/restore", h.Family.RestorePerson)
				r.Post("/relationships", h.Family.CreateRelationship)
				r.Patch("/relationships/{id}", h.Family.UpdateRelationship)
				r.Delete("/relationships/{id}", h.Family.DeleteRelationship)
				r.Post("/persons/{id}/contacts", h.Contacts.CreateFor)
				r.Patch("/persons/{id}/contacts/{cid}", h.Contacts.UpdateFor)
				r.Delete("/persons/{id}/contacts/{cid}", h.Contacts.DeleteFor)
			})
		})
	})

	// Last, so any other GET is the app's, including its client-side routes.
	r.Get("/*", webui.Handler().ServeHTTP)
	return r
}
