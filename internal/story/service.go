// Package story holds the family's recorded stories: who told them, what was said, and who is in them (architecture §9.6).
package story

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/family"
)

const (
	maxTitleLength   = 200
	maxPlaceLength   = 200
	maxSummaryLength = 5000
	maxNameLength    = 200
	maxTopicLength   = 50
	maxLineLength    = 5000
	maxTopics        = 20
	maxPeople        = 100
	maxChapters      = 200
	maxLines         = 10000
	// About six hours of speech; it also keeps the search index under Postgres's 1 MB tsvector limit.
	maxTranscriptLength  = 300000
	maxSeconds           = 24 * 60 * 60
	statusDraft          = "draft"
	programLimitExceeded = "54000"
	statusPublished      = "published"
)

var (
	ErrStoryNotFound = errors.New("story not found")
	ErrInvalidStory  = errors.New("invalid story")
	ErrNotAllowed    = errors.New("not allowed")
)

// A month or year is stored as its first day, as for people's dates.
var dateLayout = map[string]string{"day": time.DateOnly, "month": "2006-01", "year": "2006", "circa": "2006"}

var languages = []string{"ht", "fr", "en", "mixed"}

type Service struct {
	pool        *pgxpool.Pool
	lockTimeout time.Duration
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, lockTimeout: auth.LockTimeout}
}

// Story is one recording's details. RecordedOn is "YYYY-MM-DD", "YYYY-MM" or "YYYY", to match its precision.
type Story struct {
	ID                  uuid.UUID
	Title               string
	Status              string
	ConsentConfirmed    bool
	Teller              *Person
	RecordedOn          *string
	RecordedOnPrecision *string
	Place               *string
	Summary             *string
	Language            *string
	Chapters            []Chapter
	Transcript          []Line
	Topics              []string
	People              []Person
	OtherPeople         []string
	Featured            bool
	CreatedBy           *uuid.UUID
	CreatedAt           time.Time
}

// Person is a name only, so a story never shows a date or contact the viewer couldn't see.
type Person struct {
	ID        uuid.UUID
	FirstName string
	LastName  *string
}

// Chapter and Line start At seconds from the beginning of the recording.
type Chapter struct {
	At    int    `json:"at"`
	Title string `json:"title"`
}

type Line struct {
	At   int    `json:"at"`
	Text string `json:"text"`
}

// Input is a new story; it starts as a draft.
type Input struct {
	Title               string
	TellerID            *uuid.UUID
	ConsentConfirmed    bool
	RecordedOn          *string
	RecordedOnPrecision *string
	Place               *string
	Summary             *string
	Language            *string
	Chapters            []Chapter
	Transcript          []Line
	Topics              []string
	PersonIDs           []uuid.UUID
	OtherPeople         []string
}

// Patch changes only the fields that are Set; PersonIDs, when set, replaces who is in the story.
type Patch struct {
	Title               family.Opt[string]
	TellerID            family.Opt[*uuid.UUID]
	Status              family.Opt[string]
	ConsentConfirmed    family.Opt[bool]
	Featured            family.Opt[bool]
	RecordedOn          family.Opt[*string]
	RecordedOnPrecision family.Opt[*string]
	Place               family.Opt[*string]
	Summary             family.Opt[*string]
	Language            family.Opt[*string]
	Chapters            family.Opt[[]Chapter]
	Transcript          family.Opt[[]Line]
	Topics              family.Opt[[]string]
	PersonIDs           family.Opt[[]uuid.UUID]
	OtherPeople         family.Opt[[]string]
}

// Filter narrows a list; Drafts lists drafts instead of published stories.
type Filter struct {
	Drafts   bool
	TellerID *uuid.UUID
	Topic    *string
	Q        *string
}

// List gives published stories, the story of the week first, or drafts: those the viewer added or tells, or everyone's
// for admins.
// Its stories carry no chapters or transcript; Get has them.
func (s *Service) List(ctx context.Context, viewer family.Viewer, f Filter) ([]Story, error) {
	params := gen.ListStoriesParams{Status: statusPublished, TellerID: f.TellerID, Topic: trimmed(f.Topic), Q: trimmed(f.Q)}
	if f.Drafts {
		params.Status = statusDraft
		if !viewer.IsAdmin {
			params.CreatedBy, params.TellerOf = &viewer.UserID, viewer.PersonID
		}
	}
	var stories []Story
	err := s.read(ctx, "list stories", func(q *gen.Queries) error {
		rows, err := q.ListStories(ctx, params)
		if err != nil {
			return fmt.Errorf("list stories: %w", err)
		}
		stories = make([]Story, 0, len(rows))
		for _, row := range rows {
			story, err := toStory(gen.GetStoryRow(row))
			if err != nil {
				return err
			}
			stories = append(stories, story)
		}
		return withPeople(ctx, q, stories)
	})
	if err != nil {
		return nil, err
	}
	return stories, nil
}

// Get answers for a draft only to its creator, its teller and admins; to anyone else it doesn't exist.
func (s *Service) Get(ctx context.Context, viewer family.Viewer, id uuid.UUID) (Story, error) {
	var story Story
	err := s.read(ctx, "get story", func(q *gen.Queries) error {
		var err error
		story, err = get(ctx, q, id)
		var teller *uuid.UUID
		if story.Teller != nil {
			teller = &story.Teller.ID
		}
		if err == nil && !canSee(viewer, story.Status, story.CreatedBy, teller) {
			return ErrStoryNotFound
		}
		return err
	})
	if err != nil {
		return Story{}, err
	}
	return story, nil
}

// Create saves a draft; the router decides who may add stories.
func (s *Service) Create(ctx context.Context, viewer family.Viewer, in Input) (Story, error) {
	f, err := normalize(in)
	if err != nil {
		return Story{}, err
	}
	chapters, transcript, err := encode(f.Chapters, f.Transcript)
	if err != nil {
		return Story{}, err
	}
	var consentAt *time.Time
	if f.ConsentConfirmed {
		consentAt = new(time.Now())
	}
	var story Story
	// The created_by foreign key waits on the creator's users row, which sign-ins lock FOR UPDATE.
	err = s.write(ctx, "create story", func(q *gen.Queries) error {
		if err := lockPeople(ctx, q, f.TellerID, f.PersonIDs); err != nil {
			return err
		}
		id, err := q.CreateStory(ctx, gen.CreateStoryParams{
			Title: f.Title, TellerPersonID: f.TellerID, ConsentConfirmedAt: consentAt, RecordedOn: f.date,
			RecordedOnPrecision: f.RecordedOnPrecision, Place: f.Place, Summary: f.Summary, Language: f.Language,
			Chapters: chapters, Transcript: transcript, Topics: f.Topics, OtherPeople: f.OtherPeople, CreatedBy: &viewer.UserID,
		})
		if err != nil {
			return fmt.Errorf("create story: %w", tooLongToSearch(err))
		}
		if err := addPeople(ctx, q, id, f.PersonIDs); err != nil {
			return err
		}
		story, err = get(ctx, q, id)
		return err
	})
	if err != nil {
		return Story{}, err
	}
	return story, nil
}

// Update is for admins and the story's creator; only admins choose the story of the week.
func (s *Service) Update(ctx context.Context, viewer family.Viewer, id uuid.UUID, patch Patch) (Story, error) {
	return s.update(ctx, viewer, id, patch, func(row gen.LockStoryRow) bool { return canEdit(viewer, row.CreatedBy) })
}

// Hide is the one change a story's teller may make (Patch.OnlyHides), and only they may make it this way, whoever
// added the story.
func (s *Service) Hide(ctx context.Context, viewer family.Viewer, id uuid.UUID, patch Patch) (Story, error) {
	if !patch.OnlyHides() {
		return Story{}, fmt.Errorf("%w: its teller can only hide it", ErrNotAllowed)
	}
	return s.update(ctx, viewer, id, patch, func(row gen.LockStoryRow) bool { return isTeller(viewer, row.LiveTellerID) })
}

func (s *Service) update(ctx context.Context, viewer family.Viewer, id uuid.UUID, patch Patch, may func(gen.LockStoryRow) bool) (Story, error) {
	// One alone would be read against the other's stored value: "1984-11-13" can't be read as a year.
	if patch.RecordedOn.Set != patch.RecordedOnPrecision.Set {
		return Story{}, fmt.Errorf("%w: give the date and its precision together", ErrInvalidStory)
	}
	if patch.Featured.Set && !viewer.IsAdmin {
		return Story{}, fmt.Errorf("%w: only admins choose the story of the week", ErrNotAllowed)
	}
	var story Story
	err := s.write(ctx, "update story", func(q *gen.Queries) error {
		// Before the story's own row: featuring needs the current story of the week's row too, and taking the two
		// in the other order would deadlock with someone featuring that one.
		if patch.Featured.Set && patch.Featured.Value {
			if err := q.LockStoryOfTheWeek(ctx); err != nil {
				return fmt.Errorf("lock story of the week: %w", err)
			}
		}
		row, err := lock(ctx, q, viewer, id, may)
		if err != nil {
			return err
		}
		current, err := toInput(row)
		if err != nil {
			return err
		}
		f, err := normalize(patch.apply(current))
		if err != nil {
			return err
		}
		status := row.Status
		if patch.Status.Set {
			status = patch.Status.Value
		}
		if status != statusDraft && status != statusPublished {
			return fmt.Errorf("%w: the status must be draft or published", ErrInvalidStory)
		}
		// Kept when still confirmed, so it says when the teller first agreed.
		consentAt := row.ConsentConfirmedAt
		// A teller hiding their story withdraws consent with it, so it isn't republished without asking them again,
		// even when they could also edit it.
		hiddenByTeller := patch.OnlyHides() && isTeller(viewer, row.LiveTellerID)
		// A new teller hasn't agreed to anything yet, unless consent is confirmed again in this same change.
		newTeller := patch.TellerID.Set && !equalIDs(patch.TellerID.Value, row.TellerPersonID)
		reconfirmed := patch.ConsentConfirmed.Set && patch.ConsentConfirmed.Value
		switch {
		case !f.ConsentConfirmed || hiddenByTeller || (newTeller && !reconfirmed):
			consentAt = nil
		case newTeller || consentAt == nil:
			consentAt = new(time.Now())
		}
		if status == statusPublished && consentAt == nil {
			return fmt.Errorf("%w: confirm the storyteller agreed to share it before publishing", ErrInvalidStory)
		}
		featuredAt := row.FeaturedAt
		if patch.Featured.Set {
			featuredAt = nil
			if patch.Featured.Value {
				featuredAt = new(time.Now())
			}
		}
		if featuredAt != nil && status != statusPublished {
			if patch.Featured.Set && patch.Featured.Value {
				return fmt.Errorf("%w: only a published story can be the story of the week", ErrInvalidStory)
			}
			featuredAt = nil
		}
		chapters, transcript, err := encode(f.Chapters, f.Transcript)
		if err != nil {
			return err
		}
		// Only what the patch names: a teller since deleted from the tree mustn't block changing the people.
		var teller *uuid.UUID
		var people []uuid.UUID
		if patch.TellerID.Set {
			teller = f.TellerID
		}
		if patch.PersonIDs.Set {
			people = f.PersonIDs
		}
		if err := lockPeople(ctx, q, teller, people); err != nil {
			return err
		}
		if patch.Featured.Set && patch.Featured.Value {
			if err := q.UnfeatureOtherStories(ctx, id); err != nil {
				return fmt.Errorf("unfeature other stories: %w", err)
			}
		}
		err = q.UpdateStory(ctx, gen.UpdateStoryParams{
			ID: id, Title: f.Title, TellerPersonID: f.TellerID, Status: status, ConsentConfirmedAt: consentAt,
			RecordedOn: f.date, RecordedOnPrecision: f.RecordedOnPrecision, Place: f.Place, Summary: f.Summary,
			Language: f.Language, Chapters: chapters, Transcript: transcript, Topics: f.Topics, OtherPeople: f.OtherPeople,
			FeaturedAt: featuredAt,
		})
		if err != nil {
			return fmt.Errorf("update story: %w", tooLongToSearch(err))
		}
		if patch.PersonIDs.Set {
			if err := q.ClearStoryPeople(ctx, id); err != nil {
				return fmt.Errorf("clear story people: %w", err)
			}
			if err := addPeople(ctx, q, id, f.PersonIDs); err != nil {
				return err
			}
		}
		story, err = get(ctx, q, id)
		return err
	})
	if err != nil {
		return Story{}, err
	}
	return story, nil
}

// Delete only hides the story, so it can be brought back by hand (§3.1); for admins and the story's creator.
func (s *Service) Delete(ctx context.Context, viewer family.Viewer, id uuid.UUID) error {
	return s.write(ctx, "delete story", func(q *gen.Queries) error {
		if _, err := lock(ctx, q, viewer, id, func(row gen.LockStoryRow) bool { return canEdit(viewer, row.CreatedBy) }); err != nil {
			return err
		}
		if err := q.SoftDeleteStory(ctx, id); err != nil {
			return fmt.Errorf("delete story: %w", err)
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

// read runs fn's queries on one snapshot, so a story's details and its people come from the same moment.
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

func equalIDs(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// tooLongToSearch turns Postgres refusing a search index over 1 MB into a refusal the editor can act on; the length
// limits keep most stories well under it, but many distinct words in a multibyte script can still pass them.
func tooLongToSearch(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == programLimitExceeded {
		return fmt.Errorf("%w: the transcript has too many different words to search; split it into two stories", ErrInvalidStory)
	}
	return err
}

func canSee(viewer family.Viewer, status string, createdBy, teller *uuid.UUID) bool {
	return status == statusPublished || canEdit(viewer, createdBy) || isTeller(viewer, teller)
}

func isTeller(viewer family.Viewer, teller *uuid.UUID) bool {
	return teller != nil && viewer.PersonID != nil && *teller == *viewer.PersonID
}

// OnlyHides is the one change a story's teller may make: take it back to draft, withdraw consent, or both. Any other
// field set, including one added to Patch later, makes it something else.
func (p Patch) OnlyHides() bool {
	if !p.Status.Set && !p.ConsentConfirmed.Set {
		return false
	}
	if (p.Status.Set && p.Status.Value != statusDraft) || (p.ConsentConfirmed.Set && p.ConsentConfirmed.Value) {
		return false
	}
	v := reflect.ValueOf(p)
	for i := range v.NumField() {
		if name := v.Type().Field(i).Name; name != "Status" && name != "ConsentConfirmed" && v.Field(i).FieldByName("Set").Bool() {
			return false
		}
	}
	return true
}

func canEdit(viewer family.Viewer, createdBy *uuid.UUID) bool {
	return viewer.IsAdmin || (createdBy != nil && *createdBy == viewer.UserID)
}

// lock takes the story for a change by someone who may make it; a draft they can't see doesn't exist for them.
func lock(ctx context.Context, q *gen.Queries, viewer family.Viewer, id uuid.UUID, may func(gen.LockStoryRow) bool) (gen.LockStoryRow, error) {
	row, err := q.LockStory(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.LockStoryRow{}, ErrStoryNotFound
	}
	if err != nil {
		return gen.LockStoryRow{}, fmt.Errorf("lock story: %w", err)
	}
	if !canSee(viewer, row.Status, row.CreatedBy, row.LiveTellerID) {
		return gen.LockStoryRow{}, ErrStoryNotFound
	}
	if !may(row) {
		return gen.LockStoryRow{}, fmt.Errorf("%w: only admins and whoever added it can change a story; its teller can only hide it", ErrNotAllowed)
	}
	return row, nil
}

func get(ctx context.Context, q *gen.Queries, id uuid.UUID) (Story, error) {
	row, err := q.GetStory(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Story{}, ErrStoryNotFound
	}
	if err != nil {
		return Story{}, fmt.Errorf("get story: %w", err)
	}
	story, err := toStory(row)
	if err != nil {
		return Story{}, err
	}
	stories := []Story{story}
	if err := withPeople(ctx, q, stories); err != nil {
		return Story{}, err
	}
	return stories[0], nil
}

// withPeople fills in who is in each story, A–Z, with one query for all of them.
func withPeople(ctx context.Context, q *gen.Queries, stories []Story) error {
	if len(stories) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(stories))
	at := make(map[uuid.UUID]int, len(stories))
	for i, s := range stories {
		ids[i], at[s.ID] = s.ID, i
	}
	rows, err := q.ListStoryPeople(ctx, ids)
	if err != nil {
		return fmt.Errorf("list story people: %w", err)
	}
	for _, row := range rows {
		s := &stories[at[row.StoryID]]
		s.People = append(s.People, Person{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName})
	}
	return nil
}

// lockPeople holds the teller and the people in the story until commit, so none of them is deleted meanwhile.
func lockPeople(ctx context.Context, q *gen.Queries, teller *uuid.UUID, people []uuid.UUID) error {
	ids := people
	if teller != nil && !slices.Contains(people, *teller) {
		ids = append(slices.Clone(people), *teller)
	}
	if len(ids) == 0 {
		return nil
	}
	found, err := q.LockPersonsToTag(ctx, ids)
	if err != nil {
		return fmt.Errorf("lock story people: %w", err)
	}
	if len(found) != len(ids) {
		if teller != nil && !slices.Contains(found, *teller) {
			return fmt.Errorf("%w: the storyteller isn't in the tree", ErrInvalidStory)
		}
		return fmt.Errorf("%w: someone in the story isn't in the tree", ErrInvalidStory)
	}
	return nil
}

func addPeople(ctx context.Context, q *gen.Queries, id uuid.UUID, people []uuid.UUID) error {
	if len(people) == 0 {
		return nil
	}
	if err := q.AddStoryPeople(ctx, gen.AddStoryPeopleParams{StoryID: id, PersonIds: people}); err != nil {
		return fmt.Errorf("add story people: %w", err)
	}
	return nil
}

func toStory(row gen.GetStoryRow) (Story, error) {
	story := Story{
		ID: row.ID, Title: row.Title, Status: row.Status, ConsentConfirmed: row.ConsentConfirmedAt != nil,
		RecordedOn: formatDate(row.RecordedOn, row.RecordedOnPrecision), RecordedOnPrecision: row.RecordedOnPrecision,
		Place: row.Place, Summary: row.Summary, Language: row.Language, Topics: row.Topics, People: []Person{},
		OtherPeople: row.OtherPeople, Featured: row.FeaturedAt != nil, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
	if row.TellerID != nil {
		story.Teller = &Person{ID: *row.TellerID, FirstName: *row.TellerFirstName, LastName: row.TellerLastName}
	}
	if err := decode(row.Chapters, row.Transcript, &story.Chapters, &story.Transcript); err != nil {
		return Story{}, err
	}
	return story, nil
}

func toInput(row gen.LockStoryRow) (Input, error) {
	in := Input{
		Title: row.Title, TellerID: row.TellerPersonID, ConsentConfirmed: row.ConsentConfirmedAt != nil,
		RecordedOn: formatDate(row.RecordedOn, row.RecordedOnPrecision), RecordedOnPrecision: row.RecordedOnPrecision,
		Place: row.Place, Summary: row.Summary, Language: row.Language, Topics: row.Topics, OtherPeople: row.OtherPeople,
	}
	return in, decode(row.Chapters, row.Transcript, &in.Chapters, &in.Transcript)
}

func encode(chapters []Chapter, transcript []Line) ([]byte, []byte, error) {
	c, err := json.Marshal(chapters)
	if err != nil {
		return nil, nil, fmt.Errorf("encode chapters: %w", err)
	}
	t, err := json.Marshal(transcript)
	if err != nil {
		return nil, nil, fmt.Errorf("encode transcript: %w", err)
	}
	return c, t, nil
}

func decode(chapters, transcript []byte, c *[]Chapter, t *[]Line) error {
	if err := json.Unmarshal(chapters, c); err != nil {
		return fmt.Errorf("decode chapters: %w", err)
	}
	if err := json.Unmarshal(transcript, t); err != nil {
		return fmt.Errorf("decode transcript: %w", err)
	}
	return nil
}

func (p Patch) apply(in Input) Input {
	set(&in.Title, p.Title)
	set(&in.TellerID, p.TellerID)
	set(&in.ConsentConfirmed, p.ConsentConfirmed)
	set(&in.RecordedOn, p.RecordedOn)
	set(&in.RecordedOnPrecision, p.RecordedOnPrecision)
	set(&in.Place, p.Place)
	set(&in.Summary, p.Summary)
	set(&in.Language, p.Language)
	set(&in.Chapters, p.Chapters)
	set(&in.Transcript, p.Transcript)
	set(&in.Topics, p.Topics)
	set(&in.PersonIDs, p.PersonIDs)
	set(&in.OtherPeople, p.OtherPeople)
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
		return fields{}, fmt.Errorf("%w: a title is required", ErrInvalidStory)
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
		{"place", &f.Place, maxPlaceLength, false},
		{"summary", &f.Summary, maxSummaryLength, true},
	} {
		*field.value = trimmed(*field.value)
		if *field.value == nil {
			continue
		}
		if err := checkText(field.name, **field.value, field.max, field.multiline); err != nil {
			return fields{}, err
		}
	}
	if f.Language = trimmed(f.Language); f.Language != nil && !slices.Contains(languages, *f.Language) {
		return fields{}, fmt.Errorf("%w: the language must be ht, fr, en or mixed", ErrInvalidStory)
	}
	var err error
	if f.date, err = parseDate(f.RecordedOn, f.RecordedOnPrecision); err != nil {
		return fields{}, err
	}
	if f.Topics, err = names("topic", f.Topics, maxTopics, maxTopicLength); err != nil {
		return fields{}, err
	}
	if f.OtherPeople, err = names("name", f.OtherPeople, maxPeople, maxNameLength); err != nil {
		return fields{}, err
	}
	if f.Chapters, err = chapters(f.Chapters); err != nil {
		return fields{}, err
	}
	if f.Transcript, err = transcript(f.Transcript); err != nil {
		return fields{}, err
	}
	if len(f.PersonIDs) > maxPeople {
		return fields{}, fmt.Errorf("%w: tag at most %d people", ErrInvalidStory, maxPeople)
	}
	// A copy, so the caller's list keeps its order.
	f.PersonIDs = slices.Clone(f.PersonIDs)
	slices.SortFunc(f.PersonIDs, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	f.PersonIDs = slices.Compact(f.PersonIDs)
	return f, nil
}

// names trims a list of one-line labels, drops blanks and repeats (in any case), and keeps the order given.
func names(what string, list []string, maxCount, maxLength int) ([]string, error) {
	// Room for blanks and repeats, without working through a list far too long to keep.
	if len(list) > 2*maxCount {
		return nil, fmt.Errorf("%w: give at most %d %ss", ErrInvalidStory, maxCount, what)
	}
	out := make([]string, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, n := range list {
		n = strings.TrimSpace(n)
		if n == "" || seen[strings.ToLower(n)] {
			continue
		}
		seen[strings.ToLower(n)] = true
		if err := checkText(what, n, maxLength, false); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if len(out) > maxCount {
		return nil, fmt.Errorf("%w: give at most %d %ss", ErrInvalidStory, maxCount, what)
	}
	return out, nil
}

func chapters(list []Chapter) ([]Chapter, error) {
	if len(list) > maxChapters {
		return nil, fmt.Errorf("%w: give at most %d chapters", ErrInvalidStory, maxChapters)
	}
	out := make([]Chapter, len(list))
	for i, c := range list {
		c.Title = strings.TrimSpace(c.Title)
		if c.Title == "" {
			return nil, fmt.Errorf("%w: every chapter needs a title", ErrInvalidStory)
		}
		if err := checkText("chapter title", c.Title, maxTitleLength, false); err != nil {
			return nil, err
		}
		if err := checkSeconds("chapter", c.At); err != nil {
			return nil, err
		}
		out[i] = c
	}
	slices.SortStableFunc(out, func(a, b Chapter) int { return cmp.Compare(a.At, b.At) })
	for i := 1; i < len(out); i++ {
		if out[i].At == out[i-1].At {
			return nil, fmt.Errorf("%w: two chapters start at the same time", ErrInvalidStory)
		}
	}
	return out, nil
}

func transcript(list []Line) ([]Line, error) {
	if len(list) > maxLines {
		return nil, fmt.Errorf("%w: the transcript has more than %d lines", ErrInvalidStory, maxLines)
	}
	out := make([]Line, 0, len(list))
	total := 0
	for _, l := range list {
		l.Text = strings.TrimSpace(l.Text)
		if l.Text == "" {
			continue
		}
		if err := checkText("transcript", l.Text, maxLineLength, true); err != nil {
			return nil, err
		}
		if total += utf8.RuneCountInString(l.Text); total > maxTranscriptLength {
			return nil, fmt.Errorf("%w: the transcript is longer than %d characters in all", ErrInvalidStory, maxTranscriptLength)
		}
		if err := checkSeconds("transcript line", l.At); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	slices.SortStableFunc(out, func(a, b Line) int { return cmp.Compare(a.At, b.At) })
	return out, nil
}

func checkSeconds(what string, at int) error {
	if at < 0 || at > maxSeconds {
		return fmt.Errorf("%w: a %s's time must be between 0 and %d seconds", ErrInvalidStory, what, maxSeconds)
	}
	return nil
}

// checkText refuses what Postgres can't store (NUL, invalid UTF-8) and, in a one-line field, line breaks.
func checkText(name, value string, maxLength int, multiline bool) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || (!multiline && strings.ContainsFunc(value, breaksLine)) {
		return fmt.Errorf("%w: the %s contains a character that isn't allowed", ErrInvalidStory, name)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return fmt.Errorf("%w: the %s is longer than %d characters", ErrInvalidStory, name, maxLength)
	}
	return nil
}

// breaksLine is a control character or Unicode's own line and paragraph separators, which IsControl leaves out.
func breaksLine(r rune) bool {
	return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
}

func parseDate(value, precision *string) (*time.Time, error) {
	if (value == nil) != (precision == nil) {
		return nil, fmt.Errorf("%w: give the date and its precision together", ErrInvalidStory)
	}
	if value == nil {
		return nil, nil
	}
	layout, ok := dateLayout[*precision]
	if !ok {
		return nil, fmt.Errorf("%w: the date precision must be day, month, year or circa", ErrInvalidStory)
	}
	date, err := time.Parse(layout, *value)
	// Year 0 parses in Go, but Postgres has no year 0.
	if err != nil || len(*value) != len(layout) || date.Year() < 1 {
		return nil, fmt.Errorf("%w: the date must look like %s for precision %s", ErrInvalidStory, layout, *precision)
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
