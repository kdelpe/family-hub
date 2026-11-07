package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/handler"
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
			pool := unreachablePool(t)
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

// Nothing listens on port 1, so every ping is refused at once.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://app@127.0.0.1:1/familyhub")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL must be set in CI")
		}
		t.Skip("TEST_DATABASE_URL not set; run `make up` and `make test`")
	}
	pool, err := database.NewPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
