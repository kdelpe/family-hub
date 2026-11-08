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
}

func New(h Handlers) http.Handler {
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
		r.NotFound(httpx.NotFound)
	})

	// Last, so any other GET is the app's, including its client-side routes.
	r.Get("/*", webui.Handler().ServeHTTP)
	return r
}
