package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestSeedLoadsTwice(t *testing.T) {
	pool := testdb.MigratedPool(t, MigrateUp)
	seed, err := os.ReadFile("seed.sql")
	require.NoError(t, err)
	const counts = `SELECT (SELECT count(*) FROM persons), (SELECT count(*) FROM relationships), (SELECT count(*) FROM users),
		(SELECT count(*) FROM contact_methods), (SELECT count(*) FROM access_requests)`
	load := func() [5]int {
		t.Helper()
		_, err := pool.Exec(t.Context(), string(seed))
		require.NoError(t, err)
		var n [5]int
		require.NoError(t, pool.QueryRow(t.Context(), counts).Scan(&n[0], &n[1], &n[2], &n[3], &n[4]))
		return n
	}

	first := load()
	again := load()

	assert.Equal(t, [5]int{26, 40, 2, 12, 2}, first)
	assert.Equal(t, first, again, "running it again adds nothing")
	var overParented int
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM (SELECT person_b FROM relationships
		WHERE type = 'parent_child' GROUP BY person_b HAVING count(*) > 2) x`).Scan(&overParented))
	assert.Zero(t, overParented, "nobody has more than two parents")
}
