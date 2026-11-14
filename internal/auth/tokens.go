package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

var ErrInvalidToken = errors.New("link is invalid, used or expired")

type SetPasswordInput struct {
	Token         string
	Password      string
	UserAgent     string
	ReplacesToken string
}

// SetPasswordWithToken uses an invite or reset link: all or nothing, it consumes the token, sets
// the password, drops the user's other tokens and sessions, and returns a new session (§5).
func (s *Service) SetPasswordWithToken(ctx context.Context, in SetPasswordInput) (string, User, error) {
	token, user, err := s.setPasswordWithToken(ctx, in)
	return token, user, busyIfContended(err)
}

func (s *Service) setPasswordWithToken(ctx context.Context, in SetPasswordInput) (string, User, error) {
	raw, ok := decodeToken(in.Token)
	if !ok {
		return "", User{}, ErrInvalidToken
	}
	found, err := gen.New(s.pool).GetUsableToken(ctx, hashToken(raw))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, ErrInvalidToken
	case err != nil:
		return "", User{}, fmt.Errorf("find token: %w", err)
	}
	if err := CheckNewPassword(in.Password, found.Email); err != nil {
		return "", User{}, err
	}
	// Hashed before the lock: it's slow.
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return "", User{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin set password: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)

	// A use, disable or email change that committed since the lookup wins.
	locked, err := s.lockUser(ctx, tx, qtx, found.UserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, ErrInvalidToken
	case err != nil:
		return "", User{}, err
	case locked.DisabledAt != nil || locked.Email != found.Email:
		return "", User{}, ErrInvalidToken
	}
	usable, err := qtx.UseToken(ctx, gen.UseTokenParams{UserID: found.UserID, ID: found.ID})
	switch {
	case err != nil:
		return "", User{}, fmt.Errorf("use token: %w", err)
	case !usable:
		return "", User{}, ErrInvalidToken
	}
	if err := qtx.SetPasswordAndRecordLogin(ctx, gen.SetPasswordAndRecordLoginParams{ID: found.UserID, PasswordHash: hash}); err != nil {
		return "", User{}, fmt.Errorf("set password: %w", err)
	}
	if err := qtx.DeleteUserSessions(ctx, found.UserID); err != nil {
		return "", User{}, fmt.Errorf("delete sessions: %w", err)
	}
	sessions := NewSessions(tx)
	// It may be another user's session on this device.
	if err := sessions.Delete(ctx, in.ReplacesToken); err != nil {
		return "", User{}, err
	}
	token, err := sessions.Create(ctx, found.UserID, in.UserAgent)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit set password: %w", err)
	}
	return token, User{ID: found.UserID, Email: locked.Email, Role: locked.Role}, nil
}
