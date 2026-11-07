package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kdelpe/family-hub/internal/app"
	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
)

var (
	errNotImplemented = errors.New("not implemented yet")
	errMigrateUsage   = errors.New("usage: family-hub migrate up")
)

const usage = `Usage: family-hub [command]

Commands:
  serve                       run the web server (default)
  migrate up                  apply all pending migrations
  admin create --email ADDR   create the first admin (prompts for the password)
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "family-hub:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return serve()
	}
	switch args[0] {
	case "serve":
		return serve()
	case "migrate":
		return migrate(args[1:])
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

func serve() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return app.Run(context.Background(), cfg)
}

func migrate(args []string) error {
	if len(args) != 1 || args[0] != "up" {
		return fmt.Errorf("migrate: %w", errMigrateUsage)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return database.MigrateUp(cfg.DatabaseURL)
}
