package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/router"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "liveness", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "readiness reaches the database check", method: http.MethodGet, path: "/ready", wantStatus: http.StatusServiceUnavailable},
		{name: "health is GET only", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed},
		{name: "health is not under /api", method: http.MethodGet, path: "/api/health", wantStatus: http.StatusNotFound},
		{name: "unknown path", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
	}

	// Unreachable, so /ready answers 503 without needing Postgres.
	pool, err := pgxpool.New(context.Background(), "postgres://app@127.0.0.1:1/familyhub")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	routes := router.New(router.Handlers{Health: handler.NewHealth(pool)})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
