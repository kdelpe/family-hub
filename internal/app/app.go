package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/router"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 2 * time.Minute
	// Under docker stop's default 10s grace before SIGKILL.
	shutdownTimeout = 8 * time.Second
)

// Run serves until ctx is canceled. The port is taken first, so a clash fails before migrating.
func Run(ctx context.Context, cfg config.Config) error {
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return run(ctx, cfg, ln)
}

// Takes the listener so tests can pass one on a port they already hold.
func run(ctx context.Context, cfg config.Config, ln net.Listener) error {
	defer func() { _ = ln.Close() }()
	if cfg.AutoMigrate {
		if err := database.MigrateUp(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := &http.Server{
		Handler:           router.New(router.Handlers{Health: handler.NewHealth(pool)}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	slog.InfoContext(ctx, "listening", "addr", ln.Addr().String())
	return serve(ctx, srv, ln, shutdownTimeout)
}

// serve lets in-flight requests finish, up to grace, once ctx is done.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, grace time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case err := <-served:
		return fmt.Errorf("http: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Closing the connections cancels the requests' contexts, so their queries give back pool connections.
		_ = srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http: %w", err)
	}
	slog.Info("stopped")
	return nil
}
