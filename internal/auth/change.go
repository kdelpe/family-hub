package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

type ChangePasswordInput struct {
	UserID uuid.UUID
	// This device's session; it is replaced by the returned one.
	SessionToken    string
	CurrentPassword string
	NewPassword     string
	UserAgent       string
}

// ChangePassword needs the current password even with a session, which may be old or stolen (§5).
// It signs out every session, a stolen copy of this one included, and returns a new one for this device.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) (string, error) {
	// Per session, not per user: a stolen cookie's guesses mustn't lock the owner out of the fix.
	key := string(sessionHash(in.SessionToken))
	// Taken before the slow verify, so parallel guesses can't all pass an unspent limit.
	if ok, wait := s.failuresBySession.Take(key); !ok {
		return "", &RateLimitedError{RetryAfter: wait}
	}
	token, err := s.changePassword(ctx, in)
	err = busyIfContended(err)
	if !errors.Is(err, ErrInvalidCredentials) {
		s.failuresBySession.Refund(key)
	}
	// A reset, email change or disable won the lock; each signs this session out too.
	if errors.Is(err, errCredentialsChanged) {
		err = ErrNoSession
	}
	return token, err
}

func (s *Service) changePassword(ctx context.Context, in ChangePasswordInput) (string, error) {
	found, err := gen.New(s.pool).GetUserPassword(ctx, in.UserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrNoSession
	case err != nil:
		return "", fmt.Errorf("find user: %w", err)
	case found.DisabledAt != nil:
		return "", ErrNoSession
	}
	// Before any hashing, so a refused new password costs nothing.
	if err := CheckNewPassword(in.NewPassword, found.Email); err != nil {
		return "", err
	}
	if found.PasswordHash == nil || found.PasswordChangedAt == nil {
		// Google-only: no current password to give. Forgot password lets them add one.
		return "", s.refuse(ctx, in.CurrentPassword)
	}
	ok, _, err := s.hasher.Verify(ctx, *found.PasswordHash, in.CurrentPassword)
	switch {
	case errors.Is(err, errMalformedHash):
		zerolog.Ctx(ctx).Warn().Stringer("user_id", in.UserID).Msg("stored password hash is malformed")
		return "", ErrInvalidCredentials
	case err != nil:
		return "", err
	case !ok:
		return "", ErrInvalidCredentials
	}
	hash, err := s.hasher.Hash(ctx, in.NewPassword)
	if err != nil {
		return "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin change password: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)
	// A reset, email change or disable that committed since the verify wins.
	locked, err := s.lockUser(ctx, tx, qtx, in.UserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrNoSession
	case err != nil:
		return "", err
	case locked.DisabledAt != nil:
		return "", ErrNoSession
	case locked.Email != found.Email || locked.PasswordChangedAt == nil || !locked.PasswordChangedAt.Equal(*found.PasswordChangedAt):
		return "", errCredentialsChanged
	}
	// Signed out or expired since the session gate.
	active, err := qtx.SessionActive(ctx, gen.SessionActiveParams{IDHash: sessionHash(in.SessionToken), UserID: in.UserID, IdleTimeout: sessionIdleTimeout})
	switch {
	case err != nil:
		return "", fmt.Errorf("check session: %w", err)
	case !active:
		return "", ErrNoSession
	}
	if err := qtx.SetPassword(ctx, gen.SetPasswordParams{ID: in.UserID, PasswordHash: hash}); err != nil {
		return "", fmt.Errorf("set password: %w", err)
	}
	if err := qtx.DeleteUserTokens(ctx, in.UserID); err != nil {
		return "", fmt.Errorf("delete tokens: %w", err)
	}
	if err := qtx.DeleteUserSessions(ctx, in.UserID); err != nil {
		return "", fmt.Errorf("delete sessions: %w", err)
	}
	token, err := NewSessions(tx).Create(ctx, in.UserID, in.UserAgent)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit change password: %w", err)
	}
	return token, nil
}
