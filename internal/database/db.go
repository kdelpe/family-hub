package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var errInvalidConnString = errors.New("DATABASE_URL is not a valid connection string")

// NewPool opens the app's connection pool and fails unless the database answers.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// pgx's redaction of the password is best effort, so its error is dropped.
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errInvalidConnString
	}
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = connectTimeout
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: %w", err)
	}
	return pool, nil
}
