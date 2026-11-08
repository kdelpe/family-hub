package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

const (
	signInLockTimeout = 5 * time.Second
	lockNotAvailable  = "55P03"
)

type Service struct {
	pool        *pgxpool.Pool
	hasher      *Hasher
	lockTimeout time.Duration
}

func NewService(pool *pgxpool.Pool, hasher *Hasher) *Service {
	return &Service{pool: pool, hasher: hasher, lockTimeout: signInLockTimeout}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFC.String(email)))
}

type LoginInput struct {
	Email         string
	Password      string
	UserAgent     string
	ReplacesToken string
}

// Login returns a new session token. Every refusal is ErrInvalidCredentials after one Argon2
// run, so the answer doesn't say why (timing differs only mid parameter migration, §5).
func (s *Service) Login(ctx context.Context, in LoginInput) (string, User, error) {
	email := NormalizeEmail(in.Email)
	// Postgres refuses NUL in text, which would turn this into a 500 instead of a refusal.
	if strings.ContainsRune(email, 0) {
		return "", User{}, s.refuse(ctx, in.Password)
	}
	found, err := gen.New(s.pool).GetLoginUser(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, s.refuse(ctx, in.Password)
	case err != nil:
		return "", User{}, fmt.Errorf("find user: %w", err)
	case found.PasswordHash == nil || found.PasswordChangedAt == nil || found.DisabledAt != nil:
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
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout', $1, true)`, fmt.Sprintf("%dms", s.lockTimeout.Milliseconds())); err != nil {
		return "", User{}, fmt.Errorf("set lock timeout: %w", err)
	}

	// A reset, email change or disable that committed since the verify wins. A concurrent
	// login's rehash keeps password_changed_at, so it doesn't refuse this one.
	locked, err := qtx.LockUserForSignIn(ctx, found.ID)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable:
		return "", User{}, ErrBusy
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
		if !errors.Is(ctx.Err(), context.Canceled) {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("rehash password")
		}
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
