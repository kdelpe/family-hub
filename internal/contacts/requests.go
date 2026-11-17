package contacts

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/family"
)

// A repeat of an existing request is free, so this counts new people and kinds asked about (§5).
const requestsPerDay = 20

var (
	ErrInvalidRequest  = errors.New("invalid contact request")
	ErrRequestNotFound = errors.New("contact request not found")
	ErrAlreadyDecided  = errors.New("this request has already been answered")
)

// Request is one ask for the viewer's own details, as their My profile lists it.
type Request struct {
	ID        uuid.UUID
	Kind      string
	Status    string
	CreatedAt time.Time
	DecidedAt *time.Time
	From      Requester
}

type Requester struct {
	PersonID uuid.UUID
	Name     string
}

// Decision is what the owner does with a request: from the status it must have, to the one it gets.
type Decision struct{ from, to string }

var (
	Share   = Decision{from: "pending", to: "shared"}
	Decline = Decision{from: "pending", to: "declined"}
	Revoke  = Decision{from: "shared", to: "revoked"}
)

// Ask records that the viewer wants personID's phone, WhatsApp or address, and answers with their status as
// they see it: requested, or shared. Asking again changes nothing.
func (s *Service) Ask(ctx context.Context, viewer family.Viewer, personID uuid.UUID, kind string) (string, error) {
	if !slices.Contains([]string{"phone", "whatsapp", "address"}, kind) {
		return "", fmt.Errorf("%w: kind must be phone, whatsapp or address", ErrInvalidRequest)
	}
	if canSeeAll(viewer, personID) {
		return "", fmt.Errorf("%w: you can already see these details", ErrInvalidRequest)
	}
	var status string
	err := s.tx(ctx, "ask for contact", func(q *gen.Queries) error {
		n, err := q.CountPersonsInTree(ctx, []uuid.UUID{personID})
		if err != nil {
			return fmt.Errorf("check person: %w", err)
		}
		if n == 0 {
			return family.ErrPersonNotFound
		}
		if err := q.LockRequester(ctx, viewer.UserID.String()); err != nil {
			return fmt.Errorf("lock requester: %w", err)
		}
		existing, err := q.GetMyRequestStatus(ctx, gen.GetMyRequestStatusParams{RequesterID: viewer.UserID, PersonID: personID, Kind: kind})
		switch {
		case err == nil:
			status = asTheyAsked(existing)
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("get request: %w", err)
		}
		offered, err := q.OffersKind(ctx, gen.OffersKindParams{PersonID: personID, Kind: kind})
		if err != nil {
			return fmt.Errorf("check offered: %w", err)
		}
		if !offered {
			return fmt.Errorf("%w: they don't offer that on request", ErrInvalidRequest)
		}
		today, err := q.RequestsToday(ctx, viewer.UserID)
		if err != nil {
			return fmt.Errorf("count requests: %w", err)
		}
		if today.N >= requestsPerDay {
			return &auth.RateLimitedError{RetryAfter: time.Until(today.Oldest.Add(24 * time.Hour))}
		}
		if err := q.CreateContactRequest(ctx, gen.CreateContactRequestParams{RequesterID: viewer.UserID, PersonID: personID, Kind: kind}); err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		status = "requested"
		return nil
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

// Requests lists the asks for the viewer's own details: pending first, then what they've shared.
func (s *Service) Requests(ctx context.Context, viewer family.Viewer) ([]Request, error) {
	personID, err := Mine(viewer)
	if err != nil {
		return nil, err
	}
	var requests []Request
	err = s.read(ctx, func(q *gen.Queries) error {
		rows, err := q.ListRequestsFor(ctx, personID)
		if err != nil {
			return fmt.Errorf("list requests: %w", err)
		}
		requests = make([]Request, 0, len(rows))
		for _, r := range rows {
			requests = append(requests, Request{
				ID: r.ID, Kind: r.Kind, Status: r.Status, CreatedAt: r.CreatedAt, DecidedAt: r.DecidedAt,
				From: Requester{PersonID: r.RequesterPersonID, Name: auth.FullName(r.FirstName, r.LastName)},
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return requests, nil
}

// Decide shares, declines or revokes a request for the viewer's own details. The requester is never told
// of a decline or a revoke (§5.2).
func (s *Service) Decide(ctx context.Context, viewer family.Viewer, id uuid.UUID, d Decision) error {
	personID, err := Mine(viewer)
	if err != nil {
		return err
	}
	return s.tx(ctx, "decide contact request", func(q *gen.Queries) error {
		n, err := q.DecideContactRequest(ctx, gen.DecideContactRequestParams{ID: id, PersonID: personID, FromStatus: d.from, ToStatus: d.to})
		if err != nil {
			return fmt.Errorf("decide request: %w", err)
		}
		if n == 1 {
			return nil
		}
		exists, err := q.ContactRequestExists(ctx, gen.ContactRequestExistsParams{ID: id, PersonID: personID})
		if err != nil {
			return fmt.Errorf("check request: %w", err)
		}
		if !exists {
			return ErrRequestNotFound
		}
		return ErrAlreadyDecided
	})
}

func asTheyAsked(status string) string {
	if status == "shared" {
		return "shared"
	}
	return "requested"
}

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
