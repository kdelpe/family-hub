package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	authCleanupInterval = 15 * time.Minute
	authCleanupBatch    = 1000
)

type Cleanup struct {
	q *gen.Queries
}

func NewCleanup(pool *pgxpool.Pool) *Cleanup {
	return &Cleanup{q: gen.New(pool)}
}

func (c *Cleanup) Run(ctx context.Context) {
	ticker := time.NewTicker(authCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.prune(ctx); err != nil && ctx.Err() == nil {
				zerolog.Ctx(ctx).Warn().Err(err).Msg("auth cleanup")
			}
		}
	}
}

func (c *Cleanup) prune(ctx context.Context) error {
	if _, err := c.q.DeleteOldExpiredTokens(ctx, int32(authCleanupBatch)); err != nil {
		return fmt.Errorf("delete expired auth tokens: %w", err)
	}
	if _, err := c.q.DeleteExpiredSessions(ctx, int32(authCleanupBatch)); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
