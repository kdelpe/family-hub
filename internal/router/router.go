// Package router is the only place routes and middleware are wired (architecture §4, §6.1).
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kdelpe/family-hub/internal/handler"
)

type Handlers struct {
	Health *handler.Health
}

func New(h Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health.Live)
	r.Get("/ready", h.Health.Ready)

	return r
}
