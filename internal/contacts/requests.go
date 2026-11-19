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
	ErrWrongState      = errors.New("contact request can't change that way")
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
type Decision struct{ from, to, refused string }

var (
	Share   = Decision{from: "pending", to: "shared", refused: "only a pending request can be shared"}
	Decline = Decision{from: "pending", to: "declined", refused: "only a pending request can be declined"}
	Revoke  = Decision{from: "shared", to: "revoked", refused: "only a shared request can be taken back"}
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
	// The owner sees who asked by their place in the tree; a login linked to nobody couldn't be shown.
	if _, err := Mine(viewer); err != nil {
		return "", err
	}
	var status string
	// The person and their login are held, so a delete or disable can't land between the checks and the insert.
	err := s.lockPerson(ctx, "ask for contact", personID, func(q *gen.Queries) error {
		answerable, err := q.PersonHasLogin(ctx, &personID)
		if err != nil {
			return fmt.Errorf("check login: %w", err)
		}
		offered, err := q.OffersKind(ctx, gen.OffersKindParams{PersonID: personID, Kind: kind})
		if err != nil {
			return fmt.Errorf("check offered: %w", err)
		}
		if !answerable || !offered {
			return fmt.Errorf("%w: they don't offer that on request", ErrInvalidRequest)
		}
		if err := q.LockRequester(ctx, viewer.UserID); err != nil {
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
		today, err := q.RequestsToday(ctx, viewer.UserID)
		if err != nil {
			return fmt.Errorf("count requests: %w", err)
		}
		if today.N >= requestsPerDay {
			return &auth.RateLimitedError{RetryAfter: time.Duration(today.RetryAfterSeconds * float64(time.Second))}
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
	err = s.read(ctx, "list contact requests", func(q *gen.Queries) error {
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
		return fmt.Errorf("%w: %s", ErrWrongState, d.refused)
	})
}

func asTheyAsked(status string) string {
	if status == "shared" {
		return "shared"
	}
	return "requested"
}
