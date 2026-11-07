package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		reachable  bool
		check      func(h *handler.Health) http.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{name: "live without a database", check: func(h *handler.Health) http.HandlerFunc { return h.Live }, wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "ready with a database", reachable: true, check: func(h *handler.Health) http.HandlerFunc { return h.Ready }, wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "not ready without a database", check: func(h *handler.Health) http.HandlerFunc { return h.Ready }, wantStatus: http.StatusServiceUnavailable, wantBody: "database unavailable\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.UnreachablePool(t)
			if tt.reachable {
				pool = testPool(t)
			}
			rec := httptest.NewRecorder()

			tt.check(handler.NewHealth(pool))(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := database.NewPool(context.Background(), testdb.URL(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
