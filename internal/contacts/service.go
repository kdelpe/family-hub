// Package contacts holds each person's contact methods and who may see them (architecture §5.2).
package contacts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/family"
)

// A backstop against a runaway client; a real profile has a handful.
const maxPerPerson = 50

var (
	ErrContactNotFound = errors.New("contact method not found")
	ErrInvalidContact  = errors.New("invalid contact method")
	ErrNotYours        = errors.New("members can change only their own contact methods")
	ErrNotLinked       = errors.New("your login isn't linked to anyone in the tree yet")
)

// Contact is a full row, for its owner and admins, and for others when it's marked family.
type Contact struct {
	ID         uuid.UUID
	Kind       string
	Platform   *string
	Label      *string
	Value      string
	Visibility string
}

// Listing is what a viewer may see of one person's contact methods. Requestable is each kind (phone,
// whatsapp, address) they could ask for but haven't been shown, without its values.
type Listing struct {
	Contacts    []Contact
	Requestable []Requestable
}

type Requestable struct {
	Kind string
	// MyRequestStatus is none or requested; a declined or revoked request reads requested (§5.2).
	MyRequestStatus string
}

type ContactInput struct {
	Kind       string
	Platform   *string
	Label      *string
	Value      string
	Visibility *string
}

// ContactPatch changes only what's set; a contact's kind and platform never change.
type ContactPatch struct {
	Label      family.Opt[*string]
	Value      family.Opt[string]
	Visibility family.Opt[string]
}

type Service struct {
	pool        *pgxpool.Pool
	lockTimeout time.Duration
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, lockTimeout: auth.LockTimeout}
}

// Mine is the person a viewer's login is linked to, whose contacts /api/me/contacts manages.
func Mine(viewer family.Viewer) (uuid.UUID, error) {
	if viewer.PersonID == nil {
		return uuid.Nil, ErrNotLinked
	}
	return *viewer.PersonID, nil
}

// List gives the owner and admins every row; others get family rows and the kinds they could ask for.
func (s *Service) List(ctx context.Context, viewer family.Viewer, personID uuid.UUID) (Listing, error) {
	var listing Listing
	err := s.read(ctx, "list contacts", func(q *gen.Queries) error {
		n, err := q.CountPersonsInTree(ctx, []uuid.UUID{personID})
		if err != nil {
			return fmt.Errorf("check person: %w", err)
		}
		if n == 0 {
			return family.ErrPersonNotFound
		}
		if canSeeAll(viewer, personID) {
			rows, err := q.ListContacts(ctx, personID)
			if err != nil {
				return fmt.Errorf("list contacts: %w", err)
			}
			listing.Contacts = toContacts(rows)
			return nil
		}
		rows, err := q.ListVisibleContacts(ctx, gen.ListVisibleContactsParams{PersonID: personID, RequesterID: viewer.UserID})
		if err != nil {
			return fmt.Errorf("list visible contacts: %w", err)
		}
		listing.Contacts = toContacts(rows)
		kinds, err := q.RequestableKinds(ctx, gen.RequestableKindsParams{PersonID: personID, RequesterID: viewer.UserID})
		if err != nil {
			return fmt.Errorf("requestable kinds: %w", err)
		}
		for _, k := range kinds {
			listing.Requestable = append(listing.Requestable, Requestable(k))
		}
		return nil
	})
	if err != nil {
		return Listing{}, err
	}
	return listing, nil
}

func (s *Service) Create(ctx context.Context, viewer family.Viewer, personID uuid.UUID, in ContactInput) (Contact, error) {
	// The routes already keep members to their own person; this keeps the rule true for any caller.
	if !canSeeAll(viewer, personID) {
		return Contact{}, ErrNotYours
	}
	c := Contact{Kind: in.Kind, Platform: in.Platform, Label: in.Label, Value: in.Value, Visibility: "hidden"}
	if in.Visibility != nil {
		c.Visibility = *in.Visibility
	}
	if err := c.normalize(); err != nil {
		return Contact{}, err
	}
	var created Contact
	err := s.lockPerson(ctx, "create contact", personID, func(q *gen.Queries) error {
		if err := checkRequestable(ctx, q, personID, c); err != nil {
			return err
		}
		n, err := q.CountContacts(ctx, personID)
		if err != nil {
			return fmt.Errorf("count contacts: %w", err)
		}
		if n >= maxPerPerson {
			return fmt.Errorf("%w: a person can have at most %d contact methods", ErrInvalidContact, maxPerPerson)
		}
		row, err := q.CreateContact(ctx, gen.CreateContactParams{
			PersonID: personID, Kind: c.Kind, Platform: c.Platform, Label: c.Label, Value: c.Value, Visibility: c.Visibility,
		})
		if err != nil {
			return fmt.Errorf("create contact: %w", err)
		}
		created = Contact(row)
		return nil
	})
	if err != nil {
		return Contact{}, err
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, viewer family.Viewer, personID, id uuid.UUID, patch ContactPatch) (Contact, error) {
	if !canSeeAll(viewer, personID) {
		return Contact{}, ErrNotYours
	}
	var updated Contact
	err := s.lockPerson(ctx, "update contact", personID, func(q *gen.Queries) error {
		row, err := q.GetContact(ctx, gen.GetContactParams{ID: id, PersonID: personID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrContactNotFound
		}
		if err != nil {
			return fmt.Errorf("get contact: %w", err)
		}
		c := Contact(row)
		if patch.Label.Set {
			c.Label = patch.Label.Value
		}
		if patch.Value.Set {
			c.Value = patch.Value.Value
		}
		if patch.Visibility.Set {
			c.Visibility = patch.Visibility.Value
		}
		if err := c.normalize(); err != nil {
			return err
		}
		// Only a change asks again: a row already on request stays editable after its person loses their login.
		if row.Visibility != c.Visibility {
			if err := checkRequestable(ctx, q, personID, c); err != nil {
				return err
			}
		}
		saved, err := q.UpdateContact(ctx, gen.UpdateContactParams{ID: id, PersonID: personID, Label: c.Label, Value: c.Value, Visibility: c.Visibility})
		if err != nil {
			return fmt.Errorf("update contact: %w", err)
		}
		updated = Contact(saved)
		return nil
	})
	if err != nil {
		return Contact{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, viewer family.Viewer, personID, id uuid.UUID) error {
	if !canSeeAll(viewer, personID) {
		return ErrNotYours
	}
	return s.lockPerson(ctx, "delete contact", personID, func(q *gen.Queries) error {
		n, err := q.DeleteContact(ctx, gen.DeleteContactParams{ID: id, PersonID: personID})
		if err != nil {
			return fmt.Errorf("delete contact: %w", err)
		}
		if n == 0 {
			return ErrContactNotFound
		}
		return nil
	})
}

func canSeeAll(viewer family.Viewer, personID uuid.UUID) bool {
	return viewer.IsAdmin || (viewer.PersonID != nil && *viewer.PersonID == personID)
}

// checkRequestable refuses on_request for someone without a login: nobody could answer the request (§5.4).
func checkRequestable(ctx context.Context, q *gen.Queries, personID uuid.UUID, c Contact) error {
	if c.Visibility != "on_request" {
		return nil
	}
	has, err := q.PersonHasLogin(ctx, &personID)
	if err != nil {
		return fmt.Errorf("check login: %w", err)
	}
	if !has {
		return fmt.Errorf("%w: on request needs someone with a login to answer; use hidden", ErrInvalidContact)
	}
	return nil
}

// lockPerson runs fn in a transaction holding the person, so a delete can't land mid-write.
func (s *Service) lockPerson(ctx context.Context, what string, personID uuid.UUID, fn func(q *gen.Queries) error) error {
	return s.tx(ctx, what, func(q *gen.Queries) error {
		found, err := q.LockPersonForContacts(ctx, personID)
		if err != nil {
			return fmt.Errorf("lock person: %w", err)
		}
		if !found {
			return family.ErrPersonNotFound
		}
		return fn(q)
	})
}

// read runs fn on one snapshot, so the person check and the rows agree.
func (s *Service) read(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	return nil
}

// tx runs fn in a transaction whose lock waits give up with ErrBusy, so a held row can't pin a connection.
func (s *Service) tx(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, s.lockTimeout); err != nil {
		return err
	}
	if err := fn(gen.New(tx)); err != nil {
		return auth.BusyIfContended(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	return nil
}

func toContacts[R gen.ListContactsRow | gen.ListVisibleContactsRow](rows []R) []Contact {
	contacts := make([]Contact, 0, len(rows))
	for _, r := range rows {
		contacts = append(contacts, Contact(gen.ListContactsRow(r)))
	}
	return contacts
}
