package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	SessionLifetime    = 180 * 24 * time.Hour
	sessionIdleTimeout = 30 * 24 * time.Hour
	sessionTouchAfter  = time.Hour
	sessionTokenLen    = 32
	maxUserAgentLen    = 512
)

var ErrNoSession = errors.New("no active session")

type Session struct {
	UserID uuid.UUID
	Role   string
}

type Sessions struct {
	q *gen.Queries
}

func NewSessions(db gen.DBTX) *Sessions {
	return &Sessions{q: gen.New(db)}
}

// Create returns the token for the cookie; only its hash is stored.
func (s *Sessions) Create(ctx context.Context, userID uuid.UUID, userAgent string) (string, error) {
	raw := make([]byte, sessionTokenLen)
	_, _ = rand.Read(raw)
	var agent *string
	if userAgent != "" {
		// Postgres rejects invalid UTF-8, which a header can carry.
		ua := strings.ToValidUTF8(userAgent[:min(len(userAgent), maxUserAgentLen)], "")
		agent = &ua
	}
	if err := s.q.CreateSession(ctx, gen.CreateSessionParams{
		IDHash: hashToken(raw), UserID: userID, Lifetime: SessionLifetime, UserAgent: agent,
	}); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Sessions) Authenticate(ctx context.Context, token string) (Session, error) {
	raw, ok := decodeToken(token)
	if !ok {
		return Session{}, ErrNoSession
	}
	idHash := hashToken(raw)
	row, err := s.q.GetActiveSession(ctx, gen.GetActiveSessionParams{IDHash: idHash, IdleTimeout: sessionIdleTimeout, TouchAfter: sessionTouchAfter})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	if row.Stale {
		// The session is valid either way; a failed touch only delays sliding the idle window.
		err := s.q.TouchSession(ctx, gen.TouchSessionParams{IDHash: idHash, TouchAfter: sessionTouchAfter})
		if err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("touch session")
		}
	}
	return Session{UserID: row.UserID, Role: row.Role}, nil
}

func (s *Sessions) Delete(ctx context.Context, token string) error {
	raw, ok := decodeToken(token)
	if !ok {
		return nil
	}
	if err := s.q.DeleteSession(ctx, hashToken(raw)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func decodeToken(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return raw, err == nil && len(raw) == sessionTokenLen
}

func hashToken(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}
