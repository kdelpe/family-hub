package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/mail"
)

const (
	inviteSubject = "You're invited to the family hub"
	inviteBody    = `Bonjou,

You're invited to the family hub, a private place for our family's tree, photos and stories.

Set up your account:
%s

This link works for %d days. If it has expired, ask an admin for a new one.
Admins can see all the information you enter.
`
)

var (
	ErrInvalidRole = errors.New("role must be admin or member")
	// ErrEmailNotSent: Invite and Update then save nothing; SendReset keeps the link, so it can just be sent again.
	ErrEmailNotSent = errors.New("email could not be sent")
)

type InviteInput struct {
	Email string
	Role  string
}

// Invite creates a user without a password and emails them a link to set one. The email goes
// out before the commit, so a failed send leaves no user behind.
func (s *Users) Invite(ctx context.Context, in InviteInput) (auth.User, error) {
	email := auth.NormalizeEmail(in.Email)
	if !auth.ValidEmail(email) {
		return auth.User{}, ErrInvalidEmail
	}
	if in.Role != auth.RoleAdmin && in.Role != auth.RoleMember {
		return auth.User{}, ErrInvalidRole
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin invite: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	id, err := gen.New(tx).CreateInvitedUser(ctx, gen.CreateInvitedUserParams{Email: email, Role: in.Role})
	if isEmailTaken(err) {
		return auth.User{}, ErrEmailTaken
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	token, err := auth.CreateInvite(ctx, tx, id)
	if err != nil {
		return auth.User{}, err
	}
	if err := s.send(ctx, s.inviteMessage(email, token)); err != nil {
		return auth.User{}, err
	}
	if err := commit(ctx, tx, "invite"); err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: id, Email: email, Role: in.Role}, nil
}

func (s *Users) inviteMessage(email, token string) mail.Message {
	link := auth.SetPasswordLink(s.baseURL, token)
	days := int(auth.InviteLifetime.Hours() / 24)
	return mail.Message{To: email, Subject: inviteSubject, Body: fmt.Sprintf(inviteBody, link, days)}
}
