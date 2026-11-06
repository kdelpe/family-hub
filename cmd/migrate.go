package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
)

var errMigrateUsage = errors.New("usage: family-hub migrate up | migrate down [N]")

func migrate(args []string) error {
	run, err := parseMigrateArgs(args)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	return run(cfg.DatabaseURL)
}

func parseMigrateArgs(args []string) (func(databaseURL string) error, error) {
	switch {
	case len(args) == 1 && args[0] == "up":
		return database.MigrateUp, nil
	case len(args) >= 1 && len(args) <= 2 && args[0] == "down":
		steps := 1
		if len(args) == 2 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return nil, fmt.Errorf("down: steps must be a positive integer, got %q", args[1])
			}
			steps = n
		}
		return func(databaseURL string) error { return database.MigrateDown(databaseURL, steps) }, nil
	default:
		return nil, errMigrateUsage
	}
}
