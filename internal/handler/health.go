package handler

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const readyTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

type Health struct {
	db Pinger
}

func NewHealth(db Pinger) *Health {
	return &Health{db: db}
}

func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	writeOK(w)
}

func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		zerolog.Ctx(r.Context()).Warn().Err(err).Msg("ready check failed")
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	writeOK(w)
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}
