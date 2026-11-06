package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var errNotImplemented = errors.New("not implemented yet")

const usage = `Usage: family-hub [command]

Commands:
  serve                       run the web server (default)
  migrate up                  apply all pending migrations
  migrate down [N]            roll back N migrations (default 1)
  admin create --email ADDR   create the first admin (prompts for the password)
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "family-hub:", err)
		os.Exit(1)
	}
}

// Each command parses its own arguments once it's implemented.
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

func serve() error             { return fmt.Errorf("serve: %w", errNotImplemented) }
func migrate(_ []string) error { return fmt.Errorf("migrate: %w", errNotImplemented) }
func admin(_ []string) error   { return fmt.Errorf("admin: %w", errNotImplemented) }
