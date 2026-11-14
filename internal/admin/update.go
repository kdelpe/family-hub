package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	// Serializes the changes that can remove an enabled admin, so two can't each leave the other as the last.
	lastAdminLockKey = 7_004_001
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrNoChanges           = errors.New("nothing to change")
	ErrLastAdmin           = errors.New("the last admin who can sign in can't be demoted, disabled or given a new email")
	ErrDisabledEmailChange = errors.New("a disabled user's email can't be changed; enable them first")
)

// UpdateInput changes only the fields that are set.
type UpdateInput struct {
	ID       uuid.UUID
	Email    *string
	Role     *string
	Disabled *bool
}

// Update changes a user's role, enabled state or email. A new email clears their credentials and
// emails a fresh invite before the commit, so a failed send changes nothing (§5).
func (s *Users) Update(ctx context.Context, in UpdateInput) (Account, error) {
	if in.Email == nil && in.Role == nil && in.Disabled == nil {
		return Account{}, ErrNoChanges
	}
	var email string
	if in.Email != nil {
		email = auth.NormalizeEmail(*in.Email)
		if !auth.ValidEmail(email) {
			return Account{}, ErrInvalidEmail
		}
	}
	if in.Role != nil && *in.Role != auth.RoleAdmin && *in.Role != auth.RoleMember {
		return Account{}, ErrInvalidRole
	}
	account, err := s.update(ctx, in, email)
	return account, auth.BusyIfContended(err)
}

func (s *Users) update(ctx context.Context, in UpdateInput, email string) (Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin update user: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, auth.LockTimeout); err != nil {
		return Account{}, err
	}
	q := gen.New(tx)
	// The row lock sign-in and every credential change take, so none of them can race this.
	locked, err := q.LockUser(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUserNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("lock user: %w", err)
	}

	role, disabled := locked.Role, locked.DisabledAt != nil
	if in.Role != nil {
		role = *in.Role
	}
	if in.Disabled != nil {
		disabled = *in.Disabled
	}
	emailChanged := in.Email != nil && email != locked.Email
	if emailChanged && disabled {
		return Account{}, ErrDisabledEmailChange
	}
	// A new email clears the credentials too, so it also takes away an admin who can sign in.
	if locked.Role == auth.RoleAdmin && locked.DisabledAt == nil && locked.SetUp && (role != auth.RoleAdmin || disabled || emailChanged) {
		if err := requireAnotherAdmin(ctx, tx, q); err != nil {
			return Account{}, err
		}
	}

	if err := q.SetRoleAndDisabled(ctx, gen.SetRoleAndDisabledParams{ID: in.ID, Role: role, Disabled: disabled}); err != nil {
		return Account{}, fmt.Errorf("set role and disabled: %w", err)
	}
	if (disabled && locked.DisabledAt == nil) || emailChanged {
		if err := revoke(ctx, q, in.ID); err != nil {
			return Account{}, err
		}
	}
	var invite string
	if emailChanged {
		if err := q.ChangeEmail(ctx, gen.ChangeEmailParams{ID: in.ID, Email: email}); err != nil {
			if isEmailTaken(err) {
				return Account{}, ErrEmailTaken
			}
			return Account{}, fmt.Errorf("change email: %w", err)
		}
		if invite, err = auth.CreateInvite(ctx, tx, in.ID); err != nil {
			return Account{}, err
		}
	}
	row, err := q.GetAccount(ctx, in.ID)
	if err != nil {
		return Account{}, fmt.Errorf("read user: %w", err)
	}
	if invite != "" {
		if err := s.send(ctx, s.inviteMessage(email, invite)); err != nil {
			return Account{}, err
		}
	}
	if err := commit(ctx, tx, "update user"); err != nil {
		return Account{}, err
	}
	return accountFromRow(gen.ListUsersRow(row)), nil
}

func requireAnotherAdmin(ctx context.Context, tx pgx.Tx, q *gen.Queries) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(lastAdminLockKey)); err != nil {
		return fmt.Errorf("lock admins: %w", err)
	}
	// Counted after the lock, so it sees a change that held it before.
	admins, err := q.CountUsableAdmins(ctx)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func revoke(ctx context.Context, q *gen.Queries, id uuid.UUID) error {
	if err := q.DeleteUserSessions(ctx, id); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	if err := q.DeleteUserTokens(ctx, id); err != nil {
		return fmt.Errorf("delete tokens: %w", err)
	}
	return nil
}
