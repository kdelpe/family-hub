package database

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func seededPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool := testdb.MigratedPool(t, MigrateUp)
	seed, err := os.ReadFile("seed.sql")
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), string(seed))
	require.NoError(t, err)
	return pool, string(seed)
}

func TestSeed(t *testing.T) {
	pool, seed := seededPool(t)
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "people", query: `SELECT count(*) FROM persons`, want: 26},
		{name: "who has passed", query: `SELECT count(*) FROM persons WHERE is_deceased`, want: 5},
		{name: "parent links", query: `SELECT count(*) FROM relationships WHERE type = 'parent_child'`, want: 31},
		{name: "couples", query: `SELECT count(*) FROM relationships WHERE type = 'couple'`, want: 9},
		{name: "logins", query: `SELECT count(*) FROM users`, want: 9},
		{name: "admins", query: `SELECT count(*) FROM users WHERE role = 'admin'`, want: 2},
		{name: "invited, no password yet", query: `SELECT count(*) FROM users WHERE password_hash IS NULL`, want: 2},
		{name: "contacts", query: `SELECT count(*) FROM contact_methods`, want: 28},
		{name: "requests", query: `SELECT count(*) FROM access_requests WHERE status = 'pending'`, want: 2},
		{name: "a suggestion names its member", query: `SELECT count(*) FROM access_requests WHERE suggested_by IS NOT NULL`, want: 1},
		{name: "nobody has more than two parents",
			query: `SELECT count(*) FROM (SELECT 1 FROM relationships WHERE type = 'parent_child' GROUP BY person_b HAVING count(*) > 2) x`},
		{name: "every parent is at least 15 years older than their child, so no link is backwards",
			query: `SELECT count(*) FROM relationships r JOIN persons p ON p.id = r.person_a JOIN persons c ON c.id = r.person_b
				WHERE r.type = 'parent_child' AND p.birth_date > c.birth_date - interval '15 years'`},
		{name: "invites as the mockup shows them, one open and one expired",
			query: `SELECT count(*) FROM auth_tokens WHERE purpose = 'invite' AND consumed_at IS NULL`, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			require.NoError(t, pool.QueryRow(t.Context(), tt.query).Scan(&got))

			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("the dev password signs Maya in", func(t *testing.T) {
		var hash string
		require.NoError(t, pool.QueryRow(t.Context(), `SELECT password_hash FROM users WHERE email = 'maya.pluviose@example.com'`).Scan(&hash))

		ok, _, err := auth.NewHasher().Verify(t.Context(), hash, "sample-family-dev")

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("it refuses a database that isn't empty", func(t *testing.T) {
		_, err := pool.Exec(t.Context(), seed)

		require.ErrorContains(t, err, "only loads into an empty one")
		var n int
		require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM persons`).Scan(&n))
		assert.Equal(t, 26, n)
	})
}
