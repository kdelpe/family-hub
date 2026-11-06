package database

import (
	"errors"
	"fmt"
	"strconv"
)

var ErrMigrateUsage = errors.New("usage: family-hub migrate up | migrate down [N]")

// MigrateCommand is a parsed `migrate up` or `migrate down [N]`, checked before any connection.
type MigrateCommand struct {
	down  bool
	steps int
}

func ParseMigrateCommand(args []string) (MigrateCommand, error) {
	switch {
	case len(args) == 1 && args[0] == "up":
		return MigrateCommand{}, nil
	case len(args) == 1 && args[0] == "down":
		return MigrateCommand{down: true, steps: 1}, nil
	case len(args) == 2 && args[0] == "down":
		n, err := strconv.Atoi(args[1])
		// Round-trip so "+3" and "007" are rejected for a destructive command.
		if err != nil || n < 1 || strconv.Itoa(n) != args[1] {
			return MigrateCommand{}, fmt.Errorf("down: steps must be a positive integer, got %q", args[1])
		}
		return MigrateCommand{down: true, steps: n}, nil
	default:
		return MigrateCommand{}, ErrMigrateUsage
	}
}

func (c MigrateCommand) Run(databaseURL string) error {
	if c.down {
		return migrateDown(embedded, databaseURL, c.steps)
	}
	return MigrateUp(databaseURL)
}
