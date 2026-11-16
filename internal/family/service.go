// Package family holds the people in the tree and their relationships (architecture §3, §10).
package family

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
)

// Service is the family tree: its people and the links between them.
type Service struct {
	pool        *pgxpool.Pool
	lockTimeout time.Duration
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, lockTimeout: auth.LockTimeout}
}

// write runs fn in a transaction whose lock waits give up after lockTimeout with ErrBusy,
// so a held row can't pin a pool connection. Writes read their result back inside fn, so
// what they answer is what committed.
func (s *Service) write(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, s.lockTimeout); err != nil {
		return err
	}
	if err := fn(gen.New(tx)); err != nil {
		return auth.BusyIfContended(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	return nil
}

// read runs fn's queries on one snapshot, so a link added between two of them can't name someone the other missed.
func (s *Service) read(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	return nil
}
