package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	purposeInvite = "invite"
	purposeReset  = "password_reset"
)

var tokenTTL = map[string]time.Duration{
	purposeInvite: 7 * 24 * time.Hour,
	purposeReset:  30 * time.Minute,
}

var ErrInvalidToken = errors.New("link is invalid, used or expired")

// issueToken returns the token for an emailed link; only its hash is stored.
func issueToken(ctx context.Context, db gen.DBTX, userID uuid.UUID, purpose string) (string, error) {
	ttl, ok := tokenTTL[purpose]
	if !ok {
		return "", fmt.Errorf("issue token: unknown purpose %q", purpose)
	}
	raw, token := newToken()
	if err := gen.New(db).CreateAuthToken(ctx, gen.CreateAuthTokenParams{
		UserID: userID, Purpose: purpose, TokenHash: hashToken(raw), Ttl: ttl,
	}); err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}
	return token, nil
}

type SetPasswordInput struct {
	Token         string
	Password      string
	UserAgent     string
	ReplacesToken string
}

// SetPasswordWithToken uses an invite or reset link: all or nothing, it consumes the token, sets
// the password, drops the user's other tokens and sessions, and returns a new session (§5).
func (s *Service) SetPasswordWithToken(ctx context.Context, in SetPasswordInput) (string, User, error) {
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
	locked, err := s.lockUser(ctx, tx, found.UserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, ErrInvalidToken
	case err != nil:
		return "", User{}, err
	case locked.DisabledAt != nil || locked.Email != found.Email:
		return "", User{}, ErrInvalidToken
	}
	consumed, err := qtx.ConsumeToken(ctx, found.ID)
	switch {
	case err != nil:
		return "", User{}, fmt.Errorf("consume token: %w", err)
	case consumed == 0:
		return "", User{}, ErrInvalidToken
	}
	if err := qtx.SetPassword(ctx, gen.SetPasswordParams{ID: found.UserID, PasswordHash: hash}); err != nil {
		return "", User{}, fmt.Errorf("set password: %w", err)
	}
	if err := qtx.DeleteOtherTokens(ctx, gen.DeleteOtherTokensParams{UserID: found.UserID, KeepID: found.ID}); err != nil {
		return "", User{}, fmt.Errorf("delete other tokens: %w", err)
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
