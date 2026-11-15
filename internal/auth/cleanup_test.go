package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupPrunesStaleAuthDataInBoundedBatches(t *testing.T) {
	_, pool, userID := withTestUser(t)
	queries := []string{
		`INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
		SELECT $1, 'invite', decode(md5(n::text), 'hex'), clock_timestamp() - interval '8 days'
		FROM generate_series(1, 1001) AS n`,
		`INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
		VALUES ($1, 'invite', decode(md5('recent'), 'hex'), clock_timestamp() - interval '6 days')`,
		`INSERT INTO sessions (id_hash, user_id, expires_at)
		SELECT decode(md5(n::text), 'hex'), $1, clock_timestamp() - interval '1 day'
		FROM generate_series(1, 1001) AS n`,
		`INSERT INTO sessions (id_hash, user_id, expires_at)
		VALUES (decode(md5('active'), 'hex'), $1, clock_timestamp() + interval '1 day')`,
	}
	for _, query := range queries {
		var err error
		if strings.Contains(query, "$1") {
			_, err = pool.Exec(t.Context(), query, userID)
		} else {
			_, err = pool.Exec(t.Context(), query)
		}
		require.NoError(t, err)
	}

	cleanup := NewCleanup(pool)
	require.NoError(t, cleanup.prune(t.Context()))
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM auth_tokens WHERE expires_at < clock_timestamp() - interval '7 days'`))
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM auth_tokens WHERE expires_at >= clock_timestamp() - interval '7 days'`))
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM sessions WHERE expires_at <= clock_timestamp()`))
	assert.Equal(t, 1, count(t, pool, `SELECT count(*) FROM sessions WHERE expires_at > clock_timestamp()`))

	require.NoError(t, cleanup.prune(t.Context()))
	assert.Zero(t, count(t, pool, `SELECT count(*) FROM auth_tokens WHERE expires_at < clock_timestamp() - interval '7 days'`))
	assert.Zero(t, count(t, pool, `SELECT count(*) FROM sessions WHERE expires_at <= clock_timestamp()`))
}

func TestCleanupRunStopsWhenContextIsCanceled(t *testing.T) {
	_, pool, _ := withTestUser(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan struct{})
	go func() {
		NewCleanup(pool).Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not stop after cancellation")
	}
}
