package admin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/ratelimit"
)

const (
	// Every request counts, not just failures; over the limit the form still answers the same (§5).
	accessRequestsPerEmail       = 5
	accessRequestsPerEmailWindow = time.Hour

	// Counts stored suggestions; a repeat for an email already pending is free.
	suggestionsPerDay = 20

	maxRequestName  = 200
	maxRelatedNames = 4
	maxRequestNote  = 2000
)

// The form's list (design-notes §3.2); the server keeps to it so the queue reads the same for every request.
var relationships = []string{
	"daughter", "son", "grandchild", "sister", "brother", "niece or nephew", "cousin",
	"wife, husband or partner", "mother or father", "other relative",
}

var (
	ErrInvalidAccessRequest  = errors.New("invalid access request")
	ErrAccessRequestNotFound = errors.New("access request not found")
	// ErrAccessRequestState is a decline or approval of a request that isn't pending, or a reopen of one not declined.
	ErrAccessRequestState = errors.New("access request can't change that way")
	ErrNotLinked          = errors.New("your login isn't linked to anyone in the tree yet")
)

// AccessRequestInput is the public form: someone asking for an invitation.
type AccessRequestInput struct {
	FullName     string
	Email        string
	Side         *string
	Relationship *string
	RelatedNames []string
	Note         *string
}

// AccessRequest is one entry in the admins' Requests queue.
type AccessRequest struct {
	ID           uuid.UUID
	FullName     string
	Email        string
	Side         *string
	Relationship *string
	RelatedNames []RelatedName
	Note         *string
	Source       string
	SuggestedBy  *PersonName
	Status       string
	ReviewedAt   *time.Time
	CreatedAt    time.Time
}

// RelatedName is a name as typed, with the person in the tree it most likely means, if any.
type RelatedName struct {
	Name  string
	Match *PersonName
}

type PersonName struct {
	PersonID uuid.UUID
	Name     string
}

type AccessRequests struct {
	pool     *pgxpool.Pool
	perEmail *ratelimit.Limiter
}

func NewAccessRequests(pool *pgxpool.Pool) *AccessRequests {
	return &AccessRequests{pool: pool, perEmail: ratelimit.New(accessRequestsPerEmail, accessRequestsPerEmailWindow)}
}

// Submit stores a request for an invitation. Whatever happens after the input checks, the caller answers the
// same way: a repeat while one is pending, or an address over its limit, stores nothing. The handler drops
// bots (the honeypot) before this.
func (s *AccessRequests) Submit(ctx context.Context, in AccessRequestInput) error {
	r, err := normalizeRequest(in)
	if err != nil {
		return err
	}
	q := gen.New(s.pool)
	// A repeat is dropped without spending the address's budget, so a stranger's repeats can't use it up.
	pending, err := q.PendingAccessRequestExists(ctx, r.Email)
	if err != nil {
		return fmt.Errorf("check pending request: %w", err)
	}
	if pending {
		return nil
	}
	// Trimmed from a body of up to 1 MiB: a copy, so the limiter key doesn't keep the whole body alive.
	key := strings.Clone(r.Email)
	if ok, _ := s.perEmail.Take(key); !ok {
		return nil
	}
	n, err := q.CreateAccessRequest(ctx, gen.CreateAccessRequestParams{
		FullName: r.FullName, Email: r.Email, Side: r.Side, Relationship: r.Relationship,
		RelatedNames: r.RelatedNames, Note: r.Note, Source: "self",
	})
	// Only a stored request spends the budget: a concurrent repeat that lost the race gets its token back.
	if err != nil || n == 0 {
		s.perEmail.Refund(key)
	}
	if err != nil {
		return fmt.Errorf("create access request: %w", err)
	}
	return nil
}

// List is the Requests queue: pending first, then those declined in the last 90 days, which can be reopened.
func (s *AccessRequests) List(ctx context.Context) ([]AccessRequest, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin list access requests: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := gen.New(tx)
	rows, err := q.ListAccessRequests(ctx)
	if err != nil {
		return nil, fmt.Errorf("list access requests: %w", err)
	}
	matches, err := matchNames(ctx, q, rows)
	if err != nil {
		return nil, err
	}
	requests := make([]AccessRequest, 0, len(rows))
	for _, row := range rows {
		names := make([]RelatedName, len(row.RelatedNames))
		for i, n := range row.RelatedNames {
			names[i] = RelatedName{Name: n, Match: matches[n]}
		}
		r := AccessRequest{
			ID: row.ID, FullName: row.FullName, Email: row.Email, Side: row.Side, Relationship: row.Relationship,
			RelatedNames: names, Note: row.Note, Source: row.Source, Status: row.Status, ReviewedAt: row.ReviewedAt, CreatedAt: row.CreatedAt,
		}
		if row.SuggesterPersonID != nil && row.SuggesterFirstName != nil {
			r.SuggestedBy = &PersonName{PersonID: *row.SuggesterPersonID, Name: auth.FullName(*row.SuggesterFirstName, row.SuggesterLastName)}
		}
		requests = append(requests, r)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit list access requests: %w", err)
	}
	return requests, nil
}

// Suggest is a member suggesting a relative: an access request from them, answered like the public form.
// Past the input checks it stores nothing for a repeat while one is pending or past 20 a day (§5), and the
// caller answers the same either way.
func (s *AccessRequests) Suggest(ctx context.Context, userID uuid.UUID, personID *uuid.UUID, in AccessRequestInput) error {
	// The queue names who suggested someone by their place in the tree.
	if personID == nil {
		return ErrNotLinked
	}
	r, err := normalizeRequest(in)
	if err != nil {
		return err
	}
	pending, err := gen.New(s.pool).PendingAccessRequestExists(ctx, r.Email)
	if err != nil {
		return fmt.Errorf("check pending request: %w", err)
	}
	if pending {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin suggestion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := gen.New(tx)
	inTree, err := q.CountPersonsInTree(ctx, []uuid.UUID{*personID})
	if err != nil {
		return fmt.Errorf("check suggester: %w", err)
	}
	if inTree == 0 {
		return ErrNotLinked
	}
	if err := q.LockSuggester(ctx, userID); err != nil {
		return fmt.Errorf("lock suggester: %w", err)
	}
	n, err := q.SuggestionsToday(ctx, &userID)
	if err != nil {
		return fmt.Errorf("count suggestions: %w", err)
	}
	if n >= suggestionsPerDay {
		return nil
	}
	if _, err := q.CreateAccessRequest(ctx, gen.CreateAccessRequestParams{
		FullName: r.FullName, Email: r.Email, Side: r.Side, Relationship: r.Relationship,
		RelatedNames: r.RelatedNames, Note: r.Note, Source: "member_suggestion", SuggestedBy: &userID,
	}); err != nil {
		return fmt.Errorf("create suggestion: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit suggestion: %w", err)
	}
	return nil
}

// Decline quietly sets a pending request aside; nobody is told (§5).
func (s *AccessRequests) Decline(ctx context.Context, adminID, id uuid.UUID) error {
	q := gen.New(s.pool)
	n, err := q.DeclineAccessRequest(ctx, gen.DeclineAccessRequestParams{ID: id, ReviewedBy: &adminID})
	if err != nil {
		return fmt.Errorf("decline access request: %w", err)
	}
	return changed(ctx, q, id, n, "only a pending request can be declined")
}

// Reopen undoes a decline. It's refused while another request for that email is pending.
func (s *AccessRequests) Reopen(ctx context.Context, id uuid.UUID) error {
	q := gen.New(s.pool)
	n, err := q.ReopenAccessRequest(ctx, id)
	if isUniqueViolation(err, "uq_access_requests_pending_email") {
		return fmt.Errorf("%w: another request for that email is pending; decline or approve that one instead", ErrAccessRequestState)
	}
	if err != nil {
		return fmt.Errorf("reopen access request: %w", err)
	}
	return changed(ctx, q, id, n, "only a declined request can be reopened")
}

func changed(ctx context.Context, q *gen.Queries, id uuid.UUID, n int64, refused string) error {
	if n == 1 {
		return nil
	}
	exists, err := q.AccessRequestExists(ctx, id)
	if err != nil {
		return fmt.Errorf("check access request: %w", err)
	}
	if !exists {
		return ErrAccessRequestNotFound
	}
	return fmt.Errorf("%w: %s", ErrAccessRequestState, refused)
}

// lockRequest holds the pending request an invite answers until the invite commits, and gives the suggesting
// member's person, if any.
func lockRequest(ctx context.Context, q *gen.Queries, id uuid.UUID) (*uuid.UUID, error) {
	suggester, err := q.LockPendingAccessRequest(ctx, id)
	if err == nil {
		return suggester, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lock access request: %w", err)
	}
	exists, err := q.AccessRequestExists(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check access request: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("%w: no such access request", ErrInvalidAccessRequest)
	}
	return nil, fmt.Errorf("%w: only a pending request can be approved", ErrAccessRequestState)
}

// matchNames finds the closest person to every distinct name in the queue, in one query.
func matchNames(ctx context.Context, q *gen.Queries, rows []gen.ListAccessRequestsRow) (map[string]*PersonName, error) {
	var typed []string
	for _, row := range rows {
		for _, n := range row.RelatedNames {
			if !slices.Contains(typed, n) {
				typed = append(typed, n)
			}
		}
	}
	matches := map[string]*PersonName{}
	if len(typed) == 0 {
		return matches, nil
	}
	found, err := q.MatchRelatedNames(ctx, typed)
	if err != nil {
		return nil, fmt.Errorf("match related names: %w", err)
	}
	for _, m := range found {
		matches[typed[m.Idx]] = &PersonName{PersonID: m.PersonID, Name: auth.FullName(m.FirstName, m.LastName)}
	}
	return matches, nil
}

func normalizeRequest(in AccessRequestInput) (AccessRequestInput, error) {
	r := AccessRequestInput{Email: auth.NormalizeEmail(in.Email), Side: in.Side, Relationship: in.Relationship}
	if !auth.ValidEmail(r.Email) {
		return r, invalidRequest("the email must be an address like name@example.com")
	}
	var ok bool
	if r.FullName, ok = formLine(in.FullName, maxRequestName); !ok || r.FullName == "" {
		return r, invalidRequest(fmt.Sprintf("the full name must be one line of 1 to %d characters", maxRequestName))
	}
	if r.Side != nil && !slices.Contains([]string{"first", "second", "married_in"}, *r.Side) {
		return r, invalidRequest("side must be first, second or married_in")
	}
	if r.Relationship != nil && !slices.Contains(relationships, *r.Relationship) {
		return r, invalidRequest("relationship must be one from the form's list")
	}
	for _, n := range in.RelatedNames {
		name, ok := formLine(n, maxRequestName)
		if !ok {
			return r, invalidRequest(fmt.Sprintf("each name must be one line of at most %d characters", maxRequestName))
		}
		if name != "" {
			r.RelatedNames = append(r.RelatedNames, name)
		}
	}
	if len(r.RelatedNames) > maxRelatedNames {
		return r, invalidRequest(fmt.Sprintf("at most %d names", maxRelatedNames))
	}
	if r.RelatedNames == nil {
		r.RelatedNames = []string{}
	}
	if in.Note != nil {
		note := strings.TrimSpace(strings.ReplaceAll(*in.Note, "\r\n", "\n"))
		if !utf8.ValidString(note) || strings.ContainsFunc(note, func(c rune) bool { return unicode.IsControl(c) && c != '\n' && c != '\t' }) {
			return r, invalidRequest("the note contains a character that isn't allowed")
		}
		if utf8.RuneCountInString(note) > maxRequestNote {
			return r, invalidRequest(fmt.Sprintf("the note must be at most %d characters", maxRequestNote))
		}
		if note != "" {
			r.Note = &note
		}
	}
	return r, nil
}

// formLine trims s and reports whether it's valid text of at most limit characters on one line.
func formLine(s string, limit int) (string, bool) {
	s = strings.TrimSpace(s)
	return s, utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl) && utf8.RuneCountInString(s) <= limit
}

func invalidRequest(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidAccessRequest, msg)
}
