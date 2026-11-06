package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

var errNotImplemented = errors.New("not implemented yet")

const usage = `Usage: family-hub [command]

Commands:
  serve                       run the web server (default)
  migrate up                  apply all pending migrations
  migrate down [N]            roll back N migrations (default 1)
  migrate version             print the current migration version
  migrate force N             set the migration version without running any
  admin create --email ADDR   create the first admin (prompts for the password)
  seed                        load the sample family into an empty database (dev only)
  version                     print the build version
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "family-hub:", err)
		os.Exit(1)
	}
}

// run prints the usage for -h at any level, so subcommands just return flag.ErrHelp.
func run(args []string, stdout io.Writer) error {
	err := dispatch(args, stdout)
	if errors.Is(err, flag.ErrHelp) {
		_, err = io.WriteString(stdout, usage)
	}
	return err
}

func dispatch(args []string, stdout io.Writer) error {
	fs := newFlagSet("family-hub")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return serve(nil)
	}

	command, commandArgs := fs.Arg(0), fs.Args()[1:]
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
		if err := parseNoArgs("version", commandArgs); err != nil {
			return err
		}
		_, err := fmt.Fprintln(stdout, version)
		return err
	default:
		return fmt.Errorf("unknown command %q (see family-hub -h)", command)
	}
}

// newFlagSet discards the flag package's own output; run prints errors and usage once.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parseNoArgs(name string, args []string) error {
	fs := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%s: unexpected arguments %q", name, fs.Args())
	}
	return nil
}

func serve(args []string) error {
	if err := parseNoArgs("serve", args); err != nil {
		return err
	}
	return fmt.Errorf("serve: %w", errNotImplemented)
}

type migrateAction struct {
	name string
	n    int
}

func migrate(args []string) error {
	fs := newFlagSet("migrate")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if _, err := parseMigrateAction(fs.Args()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return fmt.Errorf("migrate: %w", errNotImplemented)
}

func parseMigrateAction(args []string) (migrateAction, error) {
	invalid := fmt.Errorf("invalid action %q (want up, down [N], version or force N)", strings.Join(args, " "))
	if len(args) == 0 {
		return migrateAction{}, invalid
	}

	name, rest := args[0], args[1:]
	switch {
	case (name == "up" || name == "version") && len(rest) == 0:
		return migrateAction{name: name}, nil
	case name == "down" && len(rest) == 0:
		return migrateAction{name: name, n: 1}, nil
	case name == "down" && len(rest) == 1:
		n, err := strconv.Atoi(rest[0])
		if err != nil || n < 1 {
			return migrateAction{}, fmt.Errorf("down: N must be a positive integer, got %q", rest[0])
		}
		return migrateAction{name: name, n: n}, nil
	case name == "force" && len(rest) == 1:
		// golang-migrate accepts -1 here to mean "no version".
		n, err := strconv.Atoi(rest[0])
		if err != nil || n < -1 {
			return migrateAction{}, fmt.Errorf("force: N must be an integer ≥ -1, got %q", rest[0])
		}
		return migrateAction{name: name, n: n}, nil
	default:
		return migrateAction{}, invalid
	}
}

func admin(args []string) error {
	fs := newFlagSet("admin")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if fs.Arg(0) != "create" {
		return errors.New("admin: want admin create --email ADDR")
	}

	createFs := newFlagSet("admin create")
	email := createFs.String("email", "", "email of the first admin")
	if err := createFs.Parse(fs.Args()[1:]); err != nil {
		return fmt.Errorf("admin create: %w", err)
	}
	if createFs.NArg() > 0 {
		return fmt.Errorf("admin create: unexpected arguments %q", createFs.Args())
	}
	if *email == "" {
		return errors.New("admin create: --email is required")
	}
	return fmt.Errorf("admin create: %w", errNotImplemented)
}

func seed(args []string) error {
	if err := parseNoArgs("seed", args); err != nil {
		return err
	}
	return fmt.Errorf("seed: %w", errNotImplemented)
}
