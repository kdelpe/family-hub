package auth

import (
	"context"
	"errors"
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

// prune runs every delete even when one fails, so one table's trouble doesn't let the others grow.
func (c *Cleanup) prune(ctx context.Context) error {
	var errs []error
	if _, err := c.q.DeleteOldExpiredTokens(ctx, int32(authCleanupBatch)); err != nil {
		errs = append(errs, fmt.Errorf("delete expired auth tokens: %w", err))
	}
	if _, err := c.q.DeleteExpiredSessions(ctx, int32(authCleanupBatch)); err != nil {
		errs = append(errs, fmt.Errorf("delete expired sessions: %w", err))
	}
	// Access requests are the admin package's, but one ticker for every table is simpler than two (§5).
	if _, err := c.q.DeleteOldDeclinedAccessRequests(ctx, int32(authCleanupBatch)); err != nil {
		errs = append(errs, fmt.Errorf("delete old declined access requests: %w", err))
	}
	return errors.Join(errs...)
}
