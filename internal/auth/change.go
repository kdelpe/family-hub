package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	idHash := sessionHash(in.SessionToken)
	if idHash == nil {
		return "", ErrNoSession
	}
	// Per session, not per user: a stolen cookie's guesses mustn't lock the owner out of the fix.
	key := string(idHash)
	// Taken before the slow verify, so parallel guesses can't all pass an unspent limit.
	if ok, wait := s.failuresBySession.Take(key); !ok {
		return "", &RateLimitedError{RetryAfter: wait}
	}
	session := gen.SessionActiveParams{IDHash: idHash, UserID: in.UserID, IdleTimeout: sessionIdleTimeout}
	token, err := s.changePassword(ctx, in, session)
	err = busyIfContended(err)
	if errors.Is(err, ErrInvalidCredentials) {
		// Serialize with resets and logout before deciding this was only a wrong password.
		if checkErr := s.currentSessionAfterFailedVerify(ctx, in.UserID, session); checkErr != nil {
			err = busyIfContended(checkErr)
		}
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		s.failuresBySession.Refund(key)
	}
	// A reset, email change or disable won the lock; each signs this session out too.
	if errors.Is(err, errCredentialsChanged) {
		err = ErrNoSession
	}
	return token, err
}

func (s *Service) currentSessionAfterFailedVerify(ctx context.Context, userID uuid.UUID, session gen.SessionActiveParams) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin failed password check: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)

	found, err := qtx.GetUserPassword(ctx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNoSession
	case err != nil:
		return fmt.Errorf("find user after failed password check: %w", err)
	}
	locked, err := s.lockUser(ctx, tx, qtx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNoSession
	case err != nil:
		return err
	case locked.DisabledAt != nil || locked.Email != found.Email || !samePasswordChangeTime(locked.PasswordChangedAt, found.PasswordChangedAt):
		return ErrNoSession
	}
	if err := activeOrNoSession(qtx.LockActiveSession(ctx, gen.LockActiveSessionParams(session))); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed password check: %w", err)
	}
	return nil
}

func samePasswordChangeTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func (s *Service) changePassword(ctx context.Context, in ChangePasswordInput, session gen.SessionActiveParams) (string, error) {
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
	if err := activeOrNoSession(qtx.LockActiveSession(ctx, gen.LockActiveSessionParams(session))); err != nil {
		return "", err
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

func activeOrNoSession(active bool, err error) error {
	switch {
	case err != nil:
		return fmt.Errorf("check session: %w", err)
	case !active:
		return ErrNoSession
	}
	return nil
}
