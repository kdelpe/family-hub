package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	InviteLifetime       = 7 * 24 * time.Hour
	ResetLifetime        = 30 * time.Minute
	SetPasswordPath      = "/auth/set-password#t="
	purposeInvite        = "invite"
	purposePasswordReset = "password_reset"
	// Google owns this domain outright; a Workspace domain needs the ID token's hd claim (§5).
	gmailDomain = "@gmail.com"
)

var ErrInvalidToken = errors.New("link is invalid, used or expired")

// CreateInvite stores an invite for a new user and returns the token for the link.
func CreateInvite(ctx context.Context, db gen.DBTX, userID uuid.UUID) (string, error) {
	return createToken(ctx, db, userID, purposeInvite, InviteLifetime)
}

// SetPasswordLink is the emailed link; the token goes in the fragment so it never reaches a server log (§5).
func SetPasswordLink(baseURL, token string) string {
	return baseURL + SetPasswordPath + token
}

func createToken(ctx context.Context, db gen.DBTX, userID uuid.UUID, purpose string, lifetime time.Duration) (string, error) {
	raw, token := newToken()
	err := gen.New(db).CreateToken(ctx, gen.CreateTokenParams{UserID: userID, Purpose: purpose, TokenHash: hashToken(raw), Lifetime: lifetime})
	if err != nil {
		return "", fmt.Errorf("create %s token: %w", purpose, err)
	}
	return token, nil
}

type LinkPreview struct {
	Purpose        string
	Email          string
	GoogleEligible bool
}

// PreviewLink says what an invite or reset link is for, without using it.
func (s *Service) PreviewLink(ctx context.Context, token string) (LinkPreview, error) {
	found, err := s.usableToken(ctx, token)
	if err != nil {
		return LinkPreview{}, err
	}
	return LinkPreview{Purpose: found.Purpose, Email: found.Email, GoogleEligible: strings.HasSuffix(found.Email, gmailDomain)}, nil
}

// usableToken finds a link that works now; using it re-checks under the user's row lock.
func (s *Service) usableToken(ctx context.Context, token string) (gen.GetUsableTokenRow, error) {
	raw, ok := decodeToken(token)
	if !ok {
		return gen.GetUsableTokenRow{}, ErrInvalidToken
	}
	found, err := gen.New(s.pool).GetUsableToken(ctx, hashToken(raw))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return gen.GetUsableTokenRow{}, ErrInvalidToken
	case err != nil:
		return gen.GetUsableTokenRow{}, fmt.Errorf("find token: %w", err)
	}
	return found, nil
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
	token, user, err := s.setPasswordWithToken(ctx, in)
	return token, user, busyIfContended(err)
}

func (s *Service) setPasswordWithToken(ctx context.Context, in SetPasswordInput) (string, User, error) {
	found, err := s.usableToken(ctx, in.Token)
	if err != nil {
		return "", User{}, err
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
