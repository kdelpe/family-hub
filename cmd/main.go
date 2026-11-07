package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/app"
	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
)

var (
	errNotImplemented = errors.New("not implemented yet")
	errMigrateUsage   = errors.New("usage: family-hub migrate up")
)

// Milliseconds, so lines from the same second keep their order.
const logTimeFormat = "2006-01-02T15:04:05.000Z07:00"

const usage = `Usage: family-hub [command]

Commands:
  serve                       run the web server (default)
  migrate up                  apply all pending migrations
  admin create --email ADDR   create the first admin (prompts for the password)
`

func main() {
	zerolog.TimeFieldFormat = logTimeFormat
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	// What zerolog.Ctx returns for a context that carries no logger of its own.
	zerolog.DefaultContextLogger = &logger
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// After the first signal, a second one kills the process as usual.
	context.AfterFunc(ctx, stop)
	err := run(ctx, os.Args[1:], os.Stdout)
	// Read before stop(), which cancels ctx itself.
	signaled := ctx.Err() != nil
	stop()
	// Stopped by a signal before the work finished: an ordinary stop, not a failure.
	if signaled && errors.Is(err, context.Canceled) {
		logger.Info().Msg("stopped")
		return
	}
	if err != nil {
		logger.Error().Err(err).Msg("failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return serve(ctx)
	}
	switch args[0] {
	case "serve":
		return serve(ctx)
	case "migrate":
		return migrate(ctx, args[1:])
	case "admin":
		return admin(args[1:])
	case "-h", "--help", "help":
		_, err := io.WriteString(stdout, usage)
		return err
	default:
		return fmt.Errorf("unknown command %q (see family-hub -h)", args[0])
	}
}

func admin(_ []string) error { return fmt.Errorf("admin: %w", errNotImplemented) }

func serve(ctx context.Context) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	if err := app.Run(ctx, cfg); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func migrate(ctx context.Context, args []string) error {
	if len(args) != 1 || args[0] != "up" {
		return fmt.Errorf("migrate: %w", errMigrateUsage)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return database.MigrateUp(ctx, cfg.DatabaseURL)
}
