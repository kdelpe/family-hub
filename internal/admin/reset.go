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

// SendReset emails an enabled user a fresh link: a new invite if they never set up their
// account (the old invites stop working), otherwise a password reset that leaves earlier
// ones valid, as Forgot password does. Sent before the commit, so a failed send saves nothing.
func (s *Users) SendReset(ctx context.Context, id uuid.UUID) error {
	return auth.BusyIfContended(s.sendReset(ctx, id))
}

func (s *Users) sendReset(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin send reset: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, lockTimeout); err != nil {
		return err
	}
	q := gen.New(tx)
	// Locked, so an email change or disable can't land between this read and the new link.
	locked, err := q.LockAccount(ctx, id)
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
		if err := q.DeleteUserInvites(ctx, id); err != nil {
			return fmt.Errorf("delete old invites: %w", err)
		}
		token, err := auth.CreateInvite(ctx, tx, id)
		if err != nil {
			return err
		}
		msg = s.inviteMessage(locked.Email, token)
	}
	if err := s.send(ctx, msg); err != nil {
		return err
	}
	return commit(ctx, tx)
}
