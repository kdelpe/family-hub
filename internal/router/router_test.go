package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/router"
	"github.com/kdelpe/family-hub/internal/testdb"
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
		{name: "liveness answers HEAD", method: http.MethodHead, path: "/health", wantStatus: http.StatusOK},
		{name: "health is GET only", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed},
		{name: "health is not under /api", method: http.MethodGet, path: "/api/health", wantStatus: http.StatusNotFound},
		{name: "unknown path", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
	}

	// Unreachable, so /ready answers 503 without needing Postgres.
	routes := router.New(router.Handlers{Health: handler.NewHealth(testdb.UnreachablePool(t))})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
