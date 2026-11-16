package handler

import (
	"context"

	"github.com/kdelpe/family-hub/internal/auth"
)

// WithSession puts a session in the context as RequireSession does, for tests that call a handler directly.
func WithSession(ctx context.Context, s auth.Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}
