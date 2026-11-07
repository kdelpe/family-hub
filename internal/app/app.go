package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

// Run serves until ctx is canceled or the process gets SIGINT or SIGTERM.
func Run(ctx context.Context, cfg config.Config) error {
	// Before the signal handler, so Ctrl-C still ends a migration waiting on the lock.
	if cfg.AutoMigrate {
		if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
			return err
		}
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           router.New(router.Handlers{Health: handler.NewHealth(pool)}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, a second one kills the process as usual.
	context.AfterFunc(ctx, stop)

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
