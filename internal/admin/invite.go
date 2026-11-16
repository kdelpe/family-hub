package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

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
	ErrNoPerson    = errors.New("person_id is required: every invited login is someone in the tree")
	// ErrPersonNotFound is about a person_id in the body, so it's a 400, not a 404.
	ErrPersonNotFound = errors.New("no such person in the tree")
	ErrPersonTaken    = errors.New("that person already has a login")
	// ErrEmailNotSent: Invite and Update then save nothing; SendReset keeps the link, so it can just be sent again.
	ErrEmailNotSent = errors.New("email could not be sent")
)

type InviteInput struct {
	Email    string
	Role     string
	PersonID uuid.UUID
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
	if in.PersonID == uuid.Nil {
		return auth.User{}, ErrNoPerson
	}
	user, err := s.invite(ctx, email, in)
	return user, auth.BusyIfContended(err)
}

func (s *Users) invite(ctx context.Context, email string, in InviteInput) (auth.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin invite: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, auth.LockTimeout); err != nil {
		return auth.User{}, err
	}
	q := gen.New(tx)
	if err := lockPersonToLink(ctx, q, in.PersonID); err != nil {
		return auth.User{}, err
	}
	id, err := q.CreateInvitedUser(ctx, gen.CreateInvitedUserParams{Email: email, Role: in.Role, PersonID: &in.PersonID})
	switch {
	case isEmailTaken(err):
		return auth.User{}, ErrEmailTaken
	case isPersonTaken(err):
		return auth.User{}, ErrPersonTaken
	case err != nil:
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
	return auth.User{ID: id, Email: email, Role: in.Role, PersonID: &in.PersonID}, nil
}

// lockPersonToLink holds the person until the commit, so they can't be deleted while a login is linked to them.
func lockPersonToLink(ctx context.Context, q *gen.Queries, id uuid.UUID) error {
	found, err := q.LockPersonToLink(ctx, id)
	if err != nil {
		return fmt.Errorf("lock person: %w", err)
	}
	if !found {
		return ErrPersonNotFound
	}
	return nil
}

func (s *Users) inviteMessage(email, token string) mail.Message {
	link := auth.SetPasswordLink(s.baseURL, token)
	days := int(auth.InviteLifetime.Hours() / 24)
	return mail.Message{To: email, Subject: inviteSubject, Body: fmt.Sprintf(inviteBody, link, days)}
}
