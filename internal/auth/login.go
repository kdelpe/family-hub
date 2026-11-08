package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	return strings.ToLower(strings.TrimSpace(email))
}

// Login returns a new session token. Every failure is ErrInvalidCredentials after
// exactly one Argon2 run, so neither the answer nor its timing says why.
func (s *Service) Login(ctx context.Context, email, password, userAgent string) (string, User, error) {
	q := gen.New(s.pool)
	found, err := q.GetLoginUser(ctx, NormalizeEmail(email))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, s.refuse(ctx, password)
	case err != nil:
		return "", User{}, fmt.Errorf("find user: %w", err)
	case found.PasswordHash == nil || found.DisabledAt != nil:
		return "", User{}, s.refuse(ctx, password)
	}

	ok, rehash, err := s.hasher.Verify(ctx, *found.PasswordHash, password)
	switch {
	case errors.Is(err, errMalformedHash):
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, err
	case !ok:
		return "", User{}, ErrInvalidCredentials
	}
	var newHash *string
	if rehash {
		h, err := s.hasher.Hash(ctx, password)
		if err != nil {
			return "", User{}, err
		}
		newHash = &h
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin login: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)

	// A reset, email change or disable that committed since the verify wins.
	locked, err := qtx.LockUserForSignIn(ctx, found.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, fmt.Errorf("lock user: %w", err)
	case locked.DisabledAt != nil || locked.PasswordHash == nil || *locked.PasswordHash != *found.PasswordHash:
		return "", User{}, ErrInvalidCredentials
	}
	if err := qtx.RecordLogin(ctx, gen.RecordLoginParams{ID: found.ID, NewPasswordHash: newHash}); err != nil {
		return "", User{}, fmt.Errorf("record login: %w", err)
	}
	token, err := NewSessions(tx).Create(ctx, found.ID, userAgent)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit login: %w", err)
	}
	return token, User{ID: found.ID, Email: locked.Email, Role: locked.Role}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return NewSessions(s.pool).Delete(ctx, token)
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := gen.New(s.pool).GetUser(ctx, id)
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
