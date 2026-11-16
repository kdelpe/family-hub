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

const (
	inviteSubject     = "You're invited to the family hub"
	inviteFromSubject = "%s invited you to the family hub"
	aboutTheHub       = "the Pluviose-Monestime Family Hub — a private place for our family's tree, photos and stories."
	inviteBody        = `Bonjou%s,

%s

Set up my account:
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
	ErrInvalidSender  = errors.New("from_person_id must be someone in the tree with an active login")
	// ErrEmailNotSent: Invite and Update then save nothing; SendReset keeps the link, so it can just be sent again.
	ErrEmailNotSent = errors.New("email could not be sent")
)

type InviteInput struct {
	Email    string
	Role     string
	PersonID uuid.UUID
	// FromPersonID is who the invite is from; nil means the sending admin.
	FromPersonID *uuid.UUID
	SentBy       uuid.UUID
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

	// No lock timeout: a second invite for the same email or person waits for the first's send, then gets a 409.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin invite: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
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
	msg, err := s.invite(ctx, tx, id, email, in.SentBy, in.FromPersonID)
	if err != nil {
		return auth.User{}, err
	}
	if err := s.send(ctx, msg); err != nil {
		return auth.User{}, err
	}
	if err := commit(ctx, tx, "invite"); err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: id, Email: email, Role: in.Role, PersonID: &in.PersonID}, nil
}

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

type name struct {
	first string
	full  string
}

// invite stores an invite link for userID and writes its email. It's from fromPersonID, or the
// sending admin's own person when that's nil (§5); with neither, it's from nobody.
func (s *Users) invite(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string, sentBy uuid.UUID, fromPersonID *uuid.UUID) (mail.Message, error) {
	q := gen.New(tx)
	adminPerson, err := q.GetUserPersonID(ctx, sentBy)
	if err != nil {
		return mail.Message{}, fmt.Errorf("get sending admin: %w", err)
	}
	from := fromPersonID
	if from == nil {
		from = adminPerson
	}
	fromName, err := senderName(ctx, q, from)
	if err != nil {
		return mail.Message{}, err
	}
	if fromName == nil {
		if fromPersonID != nil {
			return mail.Message{}, ErrInvalidSender
		}
		from = nil
	}
	var approver *name
	if adminPerson != nil && !samePerson(adminPerson, from) {
		if approver, err = senderName(ctx, q, adminPerson); err != nil {
			return mail.Message{}, err
		}
	}
	invitee, err := q.GetUserFirstName(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return mail.Message{}, fmt.Errorf("get invitee: %w", err)
	}
	token, err := auth.CreateInvite(ctx, tx, userID, auth.Sender{FromPersonID: from, SentBy: sentBy})
	if err != nil {
		return mail.Message{}, err
	}
	return inviteMessage(email, auth.SetPasswordLink(s.baseURL, token), invitee, fromName, approver), nil
}

// senderName is nil for nobody, or for someone an invite can't be from.
func senderName(ctx context.Context, q *gen.Queries, id *uuid.UUID) (*name, error) {
	if id == nil {
		return nil, nil
	}
	row, err := q.GetInviteSender(ctx, *id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sender: %w", err)
	}
	return &name{first: row.FirstName, full: auth.FullName(row.FirstName, row.LastName)}, nil
}

// inviteMessage follows design-notes §4.10.
func inviteMessage(email, link, invitee string, from, approver *name) mail.Message {
	subject, intro := inviteSubject, "You're invited to "+aboutTheHub
	if from != nil {
		subject, intro = fmt.Sprintf(inviteFromSubject, from.first), from.full+" invited you to "+aboutTheHub
	}
	if approver != nil {
		intro += "\nApproved by " + approver.full + ", admin."
	}
	greeting := ""
	if invitee != "" {
		greeting = " " + invitee
	}
	days := int(auth.InviteLifetime.Hours() / 24)
	return mail.Message{To: email, Subject: subject, Body: fmt.Sprintf(inviteBody, greeting, intro, link, days)}
}
