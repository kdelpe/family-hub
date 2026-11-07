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
		wantType   string
	}{
		{name: "liveness", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "readiness reaches the database check", method: http.MethodGet, path: "/ready", wantStatus: http.StatusServiceUnavailable},
		{name: "liveness answers HEAD", method: http.MethodHead, path: "/health", wantStatus: http.StatusOK},
		{name: "health is GET only", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed},
		{name: "health is not under /api", method: http.MethodGet, path: "/api/health", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "unknown /api path is a JSON 404", method: http.MethodGet, path: "/api/nope", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "/api itself is a JSON 404", method: http.MethodGet, path: "/api", wantStatus: http.StatusNotFound, wantType: "application/json"},
		{name: "any other path is the app", method: http.MethodGet, path: "/persons/42", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8"},
		{name: "the app answers HEAD", method: http.MethodHead, path: "/", wantStatus: http.StatusOK},
		{name: "the app is GET only", method: http.MethodPost, path: "/", wantStatus: http.StatusMethodNotAllowed},
	}

	// Unreachable, so /ready answers 503 without needing Postgres.
	routes := router.New(router.Handlers{Health: handler.NewHealth(testdb.UnreachablePool(t))})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			routes.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantType != "" {
				assert.Equal(t, tt.wantType, rec.Header().Get("Content-Type"))
			}
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), "security headers on every response")
		})
	}
}
