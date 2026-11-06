package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

var errNotImplemented = errors.New("not implemented yet")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "family-hub:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	command, commandArgs := "serve", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, commandArgs = args[0], args[1:]
	}

	switch command {
	case "serve":
		return serve(commandArgs)
	case "migrate":
		return migrate(commandArgs)
	case "admin":
		return admin(commandArgs)
	case "seed":
		return seed(commandArgs)
	case "version":
		_, err := fmt.Fprintln(stdout, version)
		return err
	default:
		return fmt.Errorf("unknown command %q (want serve, migrate, admin, seed or version)", command)
	}
}

func serve(_ []string) error   { return fmt.Errorf("serve: %w", errNotImplemented) }
func migrate(_ []string) error { return fmt.Errorf("migrate: %w", errNotImplemented) }
func admin(_ []string) error   { return fmt.Errorf("admin: %w", errNotImplemented) }
func seed(_ []string) error    { return fmt.Errorf("seed: %w", errNotImplemented) }
