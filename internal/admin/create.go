// Package admin manages users and their roles.
package admin

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/unicode/norm"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/database/gen"
)

// Postgres names the constraint of the inline UNIQUE on users.email.
const (
	usersEmailKey  = "users_email_key"
	usersPersonKey = "uq_users_person"
)

var (
	ErrCreateUsage      = errors.New("usage: family-hub admin create --email ADDR")
	ErrInvalidEmail     = errors.New("not a valid email address")
	ErrNoTerminal       = errors.New("needs an interactive terminal to read the password")
	ErrPasswordMismatch = errors.New("passwords do not match")
	ErrEmailTaken       = errors.New("a user with this email already exists")
)

// Terminal reads passwords without echoing them; Restore undoes what reading changed.
type Terminal interface {
	IsTerminal() bool
	ReadPassword(ctx context.Context, prompt string) (string, error)
	Restore() error
}

// RunCreate parses the arguments of `admin create`, asks for the password twice and creates the admin.
func RunCreate(ctx context.Context, args []string, getenv func(string) string, term Terminal, stdout io.Writer) error {
	flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rawEmail := flags.String("email", "", "")
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err = fmt.Fprintln(stdout, ErrCreateUsage)
		return err
	}
	if err != nil || flags.NArg() != 0 || *rawEmail == "" {
		return ErrCreateUsage
	}
	email := auth.NormalizeEmail(*rawEmail)
	if !auth.ValidEmail(email) {
		return ErrInvalidEmail
	}
	if !term.IsTerminal() {
		return ErrNoTerminal
	}
	defer func() { _ = term.Restore() }()

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Checked again by the insert; asking first spares typing a password for nothing.
	q := gen.New(pool)
	taken, err := q.EmailTaken(ctx, email)
	if err != nil {
		return fmt.Errorf("check email: %w", err)
	}
	if taken {
		return ErrEmailTaken
	}

	password, err := readNewPassword(ctx, term, email)
	if err != nil {
		return err
	}
	// Before any output: raw mode doesn't turn "\n" into a new line.
	if err := term.Restore(); err != nil {
		return fmt.Errorf("restore terminal: %w", err)
	}
	hash, err := auth.NewHasher().Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := createAdmin(ctx, q, email, hash); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Created admin %s\n", email)
	return err
}

func readNewPassword(ctx context.Context, term Terminal, email string) (string, error) {
	password, err := term.ReadPassword(ctx, "Password: ")
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if err := auth.CheckNewPassword(password, email); err != nil {
		return "", err
	}
	again, err := term.ReadPassword(ctx, "Password again: ")
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if norm.NFC.String(again) != norm.NFC.String(password) {
		return "", ErrPasswordMismatch
	}
	return password, nil
}

func createAdmin(ctx context.Context, q *gen.Queries, email, passwordHash string) error {
	err := q.CreateAdmin(ctx, gen.CreateAdminParams{Email: email, PasswordHash: passwordHash})
	if isEmailTaken(err) {
		return ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func isPersonTaken(err error) bool { return isUniqueViolation(err, usersPersonKey) }

func isEmailTaken(err error) bool { return isUniqueViolation(err, usersEmailKey) }

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == database.UniqueViolation && pgErr.ConstraintName == constraint
}
