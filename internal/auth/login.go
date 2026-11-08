package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"golang.org/x/text/unicode/norm"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	ID    uuid.UUID
	Email string
	Role  string
}

type Service struct {
	pool   *pgxpool.Pool
	hasher *Hasher
}

func NewService(pool *pgxpool.Pool, hasher *Hasher) *Service {
	return &Service{pool: pool, hasher: hasher}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFC.String(email)))
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	// The browser's existing session token, ended in the same transaction as the new one starts.
	ReplacesToken string
}

// Login returns a new session token. Every refusal is ErrInvalidCredentials and costs the
// same Argon2 work whatever the reason, so neither the answer nor its timing says why.
func (s *Service) Login(ctx context.Context, in LoginInput) (string, User, error) {
	email := NormalizeEmail(in.Email)
	found, err := gen.New(s.pool).GetLoginUser(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, s.refuse(ctx, in.Password)
	case err != nil:
		return "", User{}, fmt.Errorf("find user: %w", err)
	case found.PasswordHash == nil || found.DisabledAt != nil:
		return "", User{}, s.refuse(ctx, in.Password)
	}

	ok, rehash, err := s.hasher.Verify(ctx, *found.PasswordHash, in.Password)
	switch {
	case errors.Is(err, errMalformedHash):
		zerolog.Ctx(ctx).Warn().Stringer("user_id", found.ID).Msg("stored password hash is malformed")
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, err
	case !ok:
		return "", User{}, ErrInvalidCredentials
	}
	newHash := s.rehash(ctx, rehash, in.Password)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin login: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)

	// A reset, email change or disable that committed since the verify wins. A concurrent
	// login's rehash keeps password_changed_at, so it doesn't refuse this one.
	locked, err := qtx.LockUserForSignIn(ctx, found.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, fmt.Errorf("lock user: %w", err)
	case locked.DisabledAt != nil || locked.Email != email ||
		locked.PasswordChangedAt == nil || !locked.PasswordChangedAt.Equal(*found.PasswordChangedAt):
		return "", User{}, ErrInvalidCredentials
	}
	if err := qtx.RecordLogin(ctx, gen.RecordLoginParams{ID: found.ID, NewPasswordHash: newHash}); err != nil {
		return "", User{}, fmt.Errorf("record login: %w", err)
	}
	sessions := NewSessions(tx)
	if err := sessions.Delete(ctx, in.ReplacesToken); err != nil {
		return "", User{}, err
	}
	token, err := sessions.Create(ctx, found.ID, in.UserAgent)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit login: %w", err)
	}
	return token, User{ID: found.ID, Email: locked.Email, Role: locked.Role}, nil
}

// A failed rehash only means the old parameters stay until the next sign-in.
func (s *Service) rehash(ctx context.Context, needed bool, password string) *string {
	if !needed {
		return nil
	}
	h, err := s.hasher.Hash(ctx, password)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("rehash password")
		return nil
	}
	return &h
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return NewSessions(s.pool).Delete(ctx, token)
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := gen.New(s.pool).GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return User{ID: u.ID, Email: u.Email, Role: u.Role}, nil
}

func (s *Service) refuse(ctx context.Context, password string) error {
	if err := s.hasher.verifyNothing(ctx, password); err != nil {
		return err
	}
	return ErrInvalidCredentials
}
