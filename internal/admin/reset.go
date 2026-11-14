package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/mail"
)

// SendReset emails an enabled user a new invite, or a password reset once they've set up; earlier links stay valid.
// Sent after the commit, so a slow mail server doesn't hold the row lock their sign-in waits on.
func (s *Users) SendReset(ctx context.Context, id uuid.UUID) error {
	return auth.BusyIfContended(s.sendReset(ctx, id))
}

func (s *Users) sendReset(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin send reset: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, auth.LockTimeout); err != nil {
		return err
	}
	q := gen.New(tx)
	// Locked, so an email change or disable can't land between this read and the new link.
	locked, err := q.LockUser(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrUserNotFound
	case err != nil:
		return fmt.Errorf("lock user: %w", err)
	case locked.DisabledAt != nil:
		return ErrUserNotFound
	}

	var msg mail.Message
	if locked.SetUp {
		token, err := auth.CreatePasswordReset(ctx, tx, id)
		if err != nil {
			return err
		}
		msg = auth.ResetMessage(s.baseURL, locked.Email, token)
	} else {
		token, err := auth.CreateInvite(ctx, tx, id)
		if err != nil {
			return err
		}
		msg = s.inviteMessage(locked.Email, token)
	}
	if err := commit(ctx, tx, "send reset"); err != nil {
		return err
	}
	return s.send(ctx, msg)
}
