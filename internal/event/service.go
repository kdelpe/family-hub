// Package event holds the family timeline: what happened, when, where, and who was there (architecture §3).
package event

import (
	"bytes"
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
	"github.com/kdelpe/family-hub/internal/family"
)

const (
	maxTitleLength       = 200
	maxLocationLength    = 200
	maxDescriptionLength = 5000
	maxPeople            = 100
)

var (
	ErrEventNotFound = errors.New("event not found")
	ErrInvalidEvent  = errors.New("invalid event")
)

// A month or year is stored as its first day, as for people's dates.
var dateLayout = map[string]string{"day": time.DateOnly, "month": "2006-01", "year": "2006", "circa": "2006"}

type Service struct {
	pool        *pgxpool.Pool
	lockTimeout time.Duration
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, lockTimeout: auth.LockTimeout}
}

// Event is one entry on the timeline. Date is "YYYY-MM-DD", "YYYY-MM" or "YYYY", to match its precision.
type Event struct {
	ID            uuid.UUID
	Title         string
	Date          *string
	DatePrecision *string
	Location      *string
	Description   *string
	People        []Person
	CreatedAt     time.Time
}

// Person is someone who was there: a name only, so an event never shows a date or contact the viewer couldn't see.
type Person struct {
	ID        uuid.UUID
	FirstName string
	LastName  *string
}

// Input is a whole event as an admin enters it; a date and its precision are given together or not at all.
type Input struct {
	Title         string
	Date          *string
	DatePrecision *string
	Location      *string
	Description   *string
	PersonIDs     []uuid.UUID
}

// Patch changes only the fields that are Set; PersonIDs, when set, replaces who was there.
type Patch struct {
	Title         family.Opt[string]
	Date          family.Opt[*string]
	DatePrecision family.Opt[*string]
	Location      family.Opt[*string]
	Description   family.Opt[*string]
	PersonIDs     family.Opt[[]uuid.UUID]
}

func (s *Service) List(ctx context.Context) ([]Event, error) {
	q := gen.New(s.pool)
	rows, err := q.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, toEvent(gen.GetEventRow(row)))
	}
	if err := withPeople(ctx, q, events); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Event, error) {
	return get(ctx, gen.New(s.pool), id)
}

// Create is admin-only; the router checks the role.
func (s *Service) Create(ctx context.Context, createdBy uuid.UUID, in Input) (Event, error) {
	f, err := normalize(in)
	if err != nil {
		return Event{}, err
	}
	var event Event
	// The created_by foreign key waits on the creator's users row, which sign-ins lock FOR UPDATE.
	err = s.write(ctx, "create event", func(q *gen.Queries) error {
		id, err := q.CreateEvent(ctx, gen.CreateEventParams{
			Title: f.Title, EventDate: f.date, EventDatePrecision: f.DatePrecision,
			Location: f.Location, Description: f.Description, CreatedBy: &createdBy,
		})
		if err != nil {
			return fmt.Errorf("create event: %w", err)
		}
		if err := setPeople(ctx, q, id, f.PersonIDs); err != nil {
			return err
		}
		event, err = get(ctx, q, id)
		return err
	})
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

// Update is admin-only; the router checks the role.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (Event, error) {
	var event Event
	err := s.write(ctx, "update event", func(q *gen.Queries) error {
		row, err := q.LockEvent(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEventNotFound
		}
		if err != nil {
			return fmt.Errorf("lock event: %w", err)
		}
		f, err := normalize(patch.apply(Input{
			Title: row.Title, Date: formatDate(row.EventDate, row.EventDatePrecision), DatePrecision: row.EventDatePrecision,
			Location: row.Location, Description: row.Description,
		}))
		if err != nil {
			return err
		}
		err = q.UpdateEvent(ctx, gen.UpdateEventParams{
			ID: id, Title: f.Title, EventDate: f.date, EventDatePrecision: f.DatePrecision,
			Location: f.Location, Description: f.Description,
		})
		if err != nil {
			return fmt.Errorf("update event: %w", err)
		}
		if patch.PersonIDs.Set {
			if err := q.ClearEventPeople(ctx, id); err != nil {
				return fmt.Errorf("clear event people: %w", err)
			}
			if err := setPeople(ctx, q, id, f.PersonIDs); err != nil {
				return err
			}
		}
		event, err = get(ctx, q, id)
		return err
	})
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

// Delete only hides the event, so it can be brought back by hand (§3.1).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.write(ctx, "delete event", func(q *gen.Queries) error {
		n, err := q.SoftDeleteEvent(ctx, id)
		if err != nil {
			return fmt.Errorf("delete event: %w", err)
		}
		if n == 0 {
			return ErrEventNotFound
		}
		return nil
	})
}

// write runs fn in a transaction whose lock waits give up after lockTimeout with ErrBusy.
func (s *Service) write(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
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

func get(ctx context.Context, q *gen.Queries, id uuid.UUID) (Event, error) {
	row, err := q.GetEvent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get event: %w", err)
	}
	events := []Event{toEvent(row)}
	if err := withPeople(ctx, q, events); err != nil {
		return Event{}, err
	}
	return events[0], nil
}

// withPeople fills in who was there, A–Z, with one query for all the events.
func withPeople(ctx context.Context, q *gen.Queries, events []Event) error {
	ids := make([]uuid.UUID, len(events))
	at := make(map[uuid.UUID]int, len(events))
	for i, e := range events {
		ids[i], at[e.ID] = e.ID, i
	}
	rows, err := q.ListEventPeople(ctx, ids)
	if err != nil {
		return fmt.Errorf("list event people: %w", err)
	}
	for _, row := range rows {
		e := &events[at[row.EventID]]
		e.People = append(e.People, Person{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName})
	}
	return nil
}

func setPeople(ctx context.Context, q *gen.Queries, id uuid.UUID, people []uuid.UUID) error {
	if len(people) == 0 {
		return nil
	}
	found, err := q.CountPersonsInTree(ctx, people)
	if err != nil {
		return fmt.Errorf("check event people: %w", err)
	}
	if found != int64(len(people)) {
		return fmt.Errorf("%w: someone tagged isn't in the tree", ErrInvalidEvent)
	}
	if err := q.AddEventPeople(ctx, gen.AddEventPeopleParams{EventID: id, PersonIds: people}); err != nil {
		return fmt.Errorf("add event people: %w", err)
	}
	return nil
}

func toEvent(row gen.GetEventRow) Event {
	return Event{
		ID: row.ID, Title: row.Title, Date: formatDate(row.EventDate, row.EventDatePrecision), DatePrecision: row.EventDatePrecision,
		Location: row.Location, Description: row.Description, People: []Person{}, CreatedAt: row.CreatedAt,
	}
}

func (p Patch) apply(in Input) Input {
	set(&in.Title, p.Title)
	set(&in.Date, p.Date)
	set(&in.DatePrecision, p.DatePrecision)
	set(&in.Location, p.Location)
	set(&in.Description, p.Description)
	set(&in.PersonIDs, p.PersonIDs)
	return in
}

func set[T any](dst *T, o family.Opt[T]) {
	if o.Set {
		*dst = o.Value
	}
}

type fields struct {
	Input
	date *time.Time
}

func normalize(in Input) (fields, error) {
	f := fields{Input: in}
	f.Title = strings.TrimSpace(in.Title)
	if f.Title == "" {
		return fields{}, fmt.Errorf("%w: a title is required", ErrInvalidEvent)
	}
	if err := checkText("title", f.Title, maxTitleLength, false); err != nil {
		return fields{}, err
	}
	for _, field := range []struct {
		name      string
		value     **string
		max       int
		multiline bool
	}{
		{"location", &f.Location, maxLocationLength, false},
		{"description", &f.Description, maxDescriptionLength, true},
	} {
		*field.value = trimmed(*field.value)
		if *field.value == nil {
			continue
		}
		if err := checkText(field.name, **field.value, field.max, field.multiline); err != nil {
			return fields{}, err
		}
	}
	var err error
	if f.date, err = parseDate(f.Date, f.DatePrecision); err != nil {
		return fields{}, err
	}
	if len(f.PersonIDs) > maxPeople {
		return fields{}, fmt.Errorf("%w: tag at most %d people", ErrInvalidEvent, maxPeople)
	}
	// A copy, so the caller's list keeps its order.
	f.PersonIDs = slices.Clone(f.PersonIDs)
	slices.SortFunc(f.PersonIDs, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	f.PersonIDs = slices.Compact(f.PersonIDs)
	return f, nil
}

// checkText refuses what Postgres can't store (NUL, invalid UTF-8) and, in a one-line field, line breaks.
func checkText(name, value string, maxLength int, multiline bool) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || (!multiline && strings.ContainsFunc(value, unicode.IsControl)) {
		return fmt.Errorf("%w: the %s contains a character that isn't allowed", ErrInvalidEvent, name)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return fmt.Errorf("%w: the %s is longer than %d characters", ErrInvalidEvent, name, maxLength)
	}
	return nil
}

// parseDate allows future dates: a reunion can be planned ahead.
func parseDate(value, precision *string) (*time.Time, error) {
	if (value == nil) != (precision == nil) {
		return nil, fmt.Errorf("%w: give the date and its precision together", ErrInvalidEvent)
	}
	if value == nil {
		return nil, nil
	}
	layout, ok := dateLayout[*precision]
	if !ok {
		return nil, fmt.Errorf("%w: the date precision must be day, month, year or circa", ErrInvalidEvent)
	}
	date, err := time.Parse(layout, *value)
	// Year 0 parses in Go, but Postgres has no year 0.
	if err != nil || len(*value) != len(layout) || date.Year() < 1 {
		return nil, fmt.Errorf("%w: the date must look like %s for precision %s", ErrInvalidEvent, layout, *precision)
	}
	return &date, nil
}

func formatDate(date *time.Time, precision *string) *string {
	if date == nil || precision == nil {
		return nil
	}
	s := date.Format(dateLayout[*precision])
	return &s
}

func trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
