// Package family holds the people in the tree and, later, their relationships (architecture §3, §10).
package family

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	maxNameLength  = 100
	maxPlaceLength = 200
	maxBioLength   = 5000
	maxQueryLength = 100

	searchResults = 20
	// The People page lists everyone at once; family scale is hundreds, so this is only a backstop.
	listResults = 5000
)

var (
	ErrPersonNotFound = errors.New("person not found")
	ErrInvalidPerson  = errors.New("invalid person")
	ErrInvalidFilter  = errors.New("invalid filter")
)

var (
	sides      = []string{"first", "second", "both", "married_in"}
	genders    = []string{"male", "female", "other", "unknown"}
	dateLayout = map[string]string{"day": time.DateOnly, "month": "2006-01", "year": "2006", "circa": "2006"}
)

// Viewer is who is asking, which decides whether a living person's birth year shows (§5.2).
type Viewer struct {
	IsAdmin  bool
	PersonID *uuid.UUID
}

// Person is what a viewer may see: BirthDate is masked, "--MM-DD" when the year is hidden.
type Person struct {
	ID                 uuid.UUID
	FirstName          string
	MiddleName         *string
	LastName           *string
	MaidenName         *string
	Side               *string
	BirthPlace         *string
	Gender             *string
	BirthDate          *string
	BirthDatePrecision *string
	DeathDate          *string
	DeathDatePrecision *string
	IsDeceased         bool
	Bio                *string
}

// PersonInput is a whole person as an admin enters it. Dates are "YYYY-MM-DD", "YYYY-MM" or "YYYY" to match
// their precision; a date and its precision are given together or not at all.
type PersonInput struct {
	FirstName          string
	MiddleName         *string
	LastName           *string
	MaidenName         *string
	Side               *string
	BirthPlace         *string
	Gender             *string
	BirthDate          *string
	BirthDatePrecision *string
	DeathDate          *string
	DeathDatePrecision *string
	IsDeceased         bool
	Bio                *string
}

// Opt is a PATCH field: Set false leaves it as it is.
type Opt[T any] struct {
	Set   bool
	Value T
}

type PersonPatch struct {
	FirstName          Opt[string]
	MiddleName         Opt[*string]
	LastName           Opt[*string]
	MaidenName         Opt[*string]
	Side               Opt[*string]
	BirthPlace         Opt[*string]
	Gender             Opt[*string]
	BirthDate          Opt[*string]
	BirthDatePrecision Opt[*string]
	DeathDate          Opt[*string]
	DeathDatePrecision Opt[*string]
	IsDeceased         Opt[bool]
	Bio                Opt[*string]
}

type ListInput struct {
	Query    string
	Side     string
	Deceased *bool
}

type Persons struct {
	pool *pgxpool.Pool
}

func NewPersons(pool *pgxpool.Pool) *Persons {
	return &Persons{pool: pool}
}

// List searches by name when Query is set (best matches first), otherwise lists everyone A–Z.
func (s *Persons) List(ctx context.Context, viewer Viewer, in ListInput) ([]Person, error) {
	query := strings.TrimSpace(in.Query)
	if utf8.RuneCountInString(query) > maxQueryLength {
		return nil, fmt.Errorf("%w: search is longer than %d characters", ErrInvalidFilter, maxQueryLength)
	}
	if in.Side != "" && !slices.Contains(sides, in.Side) {
		return nil, fmt.Errorf("%w: side must be one of %s", ErrInvalidFilter, strings.Join(sides, ", "))
	}
	maxRows := listResults
	if query != "" {
		maxRows = searchResults
	}
	rows, err := gen.New(s.pool).ListPersons(ctx, gen.ListPersonsParams{
		ViewerIsAdmin: viewer.IsAdmin, ViewerPersonID: viewer.PersonID,
		Q: query, QLike: escapeLike(query), Side: in.Side, Deceased: in.Deceased, MaxRows: int32(maxRows),
	})
	if err != nil {
		return nil, fmt.Errorf("list persons: %w", err)
	}
	people := make([]Person, 0, len(rows))
	for _, row := range rows {
		people = append(people, toPerson(gen.GetPersonRow(row)))
	}
	return people, nil
}

func (s *Persons) Get(ctx context.Context, viewer Viewer, id uuid.UUID) (Person, error) {
	row, err := gen.New(s.pool).GetPerson(ctx, gen.GetPersonParams{ViewerIsAdmin: viewer.IsAdmin, ViewerPersonID: viewer.PersonID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrPersonNotFound
	}
	if err != nil {
		return Person{}, fmt.Errorf("get person: %w", err)
	}
	return toPerson(row), nil
}

// Create is admin-only; the router checks the role.
func (s *Persons) Create(ctx context.Context, createdBy uuid.UUID, in PersonInput) (Person, error) {
	f, err := normalize(in)
	if err != nil {
		return Person{}, err
	}
	id, err := gen.New(s.pool).CreatePerson(ctx, gen.CreatePersonParams{
		FirstName: f.FirstName, MiddleName: f.MiddleName, LastName: f.LastName, MaidenName: f.MaidenName, Side: f.Side,
		BirthPlace: f.BirthPlace, Gender: f.Gender, BirthDate: f.birthDate, BirthDatePrecision: f.BirthDatePrecision,
		DeathDate: f.deathDate, DeathDatePrecision: f.DeathDatePrecision, IsDeceased: f.IsDeceased, Bio: f.Bio, CreatedBy: &createdBy,
	})
	if err != nil {
		return Person{}, fmt.Errorf("create person: %w", err)
	}
	return s.Get(ctx, Viewer{IsAdmin: true}, id)
}

// Update is admin-only for now; a member editing their own bio comes with person linking (§5.1).
func (s *Persons) Update(ctx context.Context, id uuid.UUID, patch PersonPatch) (Person, error) {
	err := s.update(ctx, id, patch)
	if err != nil {
		return Person{}, auth.BusyIfContended(err)
	}
	return s.Get(ctx, Viewer{IsAdmin: true}, id)
}

func (s *Persons) update(ctx context.Context, id uuid.UUID, patch PersonPatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update person: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := auth.SetLockTimeout(ctx, tx, auth.LockTimeout); err != nil {
		return err
	}
	q := gen.New(tx)
	row, err := q.LockPerson(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPersonNotFound
	}
	if err != nil {
		return fmt.Errorf("lock person: %w", err)
	}
	f, err := normalize(patch.apply(fromRow(row)))
	if err != nil {
		return err
	}
	err = q.UpdatePerson(ctx, gen.UpdatePersonParams{
		ID: id, FirstName: f.FirstName, MiddleName: f.MiddleName, LastName: f.LastName, MaidenName: f.MaidenName, Side: f.Side,
		BirthPlace: f.BirthPlace, Gender: f.Gender, BirthDate: f.birthDate, BirthDatePrecision: f.BirthDatePrecision,
		DeathDate: f.deathDate, DeathDatePrecision: f.DeathDatePrecision, IsDeceased: f.IsDeceased, Bio: f.Bio,
	})
	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit person: %w", err)
	}
	return nil
}

// Delete only hides the person; their relationships stay, so Restore puts them back where they were (§3.1).
func (s *Persons) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := gen.New(s.pool).SoftDeletePerson(ctx, id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}
	if n == 0 {
		return ErrPersonNotFound
	}
	return nil
}

func (s *Persons) Restore(ctx context.Context, id uuid.UUID) (Person, error) {
	n, err := gen.New(s.pool).RestorePerson(ctx, id)
	if err != nil {
		return Person{}, fmt.Errorf("restore person: %w", err)
	}
	if n == 0 {
		return Person{}, ErrPersonNotFound
	}
	return s.Get(ctx, Viewer{IsAdmin: true}, id)
}

func (p PersonPatch) apply(in PersonInput) PersonInput {
	set(&in.FirstName, p.FirstName)
	set(&in.MiddleName, p.MiddleName)
	set(&in.LastName, p.LastName)
	set(&in.MaidenName, p.MaidenName)
	set(&in.Side, p.Side)
	set(&in.BirthPlace, p.BirthPlace)
	set(&in.Gender, p.Gender)
	set(&in.BirthDate, p.BirthDate)
	set(&in.BirthDatePrecision, p.BirthDatePrecision)
	set(&in.DeathDate, p.DeathDate)
	set(&in.DeathDatePrecision, p.DeathDatePrecision)
	set(&in.IsDeceased, p.IsDeceased)
	set(&in.Bio, p.Bio)
	return in
}

func set[T any](dst *T, o Opt[T]) {
	if o.Set {
		*dst = o.Value
	}
}

type fields struct {
	PersonInput
	birthDate, deathDate *time.Time
}

func normalize(in PersonInput) (fields, error) {
	f := fields{PersonInput: in}
	f.FirstName = strings.TrimSpace(in.FirstName)
	if f.FirstName == "" {
		return fields{}, fmt.Errorf("%w: a first name is required", ErrInvalidPerson)
	}
	for _, field := range []struct {
		name  string
		value **string
		max   int
	}{
		{"middle name", &f.MiddleName, maxNameLength},
		{"last name", &f.LastName, maxNameLength},
		{"maiden name", &f.MaidenName, maxNameLength},
		{"birth place", &f.BirthPlace, maxPlaceLength},
		{"bio", &f.Bio, maxBioLength},
	} {
		*field.value = trimmed(*field.value)
		if *field.value != nil && utf8.RuneCountInString(**field.value) > field.max {
			return fields{}, fmt.Errorf("%w: the %s is longer than %d characters", ErrInvalidPerson, field.name, field.max)
		}
	}
	if utf8.RuneCountInString(f.FirstName) > maxNameLength {
		return fields{}, fmt.Errorf("%w: the first name is longer than %d characters", ErrInvalidPerson, maxNameLength)
	}
	if f.Side != nil && !slices.Contains(sides, *f.Side) {
		return fields{}, fmt.Errorf("%w: side must be one of %s", ErrInvalidPerson, strings.Join(sides, ", "))
	}
	if f.Gender != nil && !slices.Contains(genders, *f.Gender) {
		return fields{}, fmt.Errorf("%w: gender must be one of %s", ErrInvalidPerson, strings.Join(genders, ", "))
	}
	var err error
	if f.birthDate, err = parseDate("birth", f.BirthDate, f.BirthDatePrecision); err != nil {
		return fields{}, err
	}
	if f.deathDate, err = parseDate("death", f.DeathDate, f.DeathDatePrecision); err != nil {
		return fields{}, err
	}
	if f.deathDate != nil && !f.IsDeceased {
		return fields{}, fmt.Errorf("%w: a death date needs is_deceased", ErrInvalidPerson)
	}
	if f.birthDate != nil && f.deathDate != nil && f.deathDate.Before(*f.birthDate) {
		return fields{}, fmt.Errorf("%w: the death date is before the birth date", ErrInvalidPerson)
	}
	return f, nil
}

func parseDate(what string, value, precision *string) (*time.Time, error) {
	if (value == nil) != (precision == nil) {
		return nil, fmt.Errorf("%w: give the %s date and its precision together", ErrInvalidPerson, what)
	}
	if value == nil {
		return nil, nil
	}
	layout, ok := dateLayout[*precision]
	if !ok {
		return nil, fmt.Errorf("%w: the %s date precision must be day, month, year or circa", ErrInvalidPerson, what)
	}
	date, err := time.Parse(layout, *value)
	if err != nil || len(*value) != len(layout) {
		return nil, fmt.Errorf("%w: the %s date must look like %s for precision %s", ErrInvalidPerson, what, layout, *precision)
	}
	if date.After(time.Now()) {
		return nil, fmt.Errorf("%w: the %s date is in the future", ErrInvalidPerson, what)
	}
	return &date, nil
}

func fromRow(row gen.LockPersonRow) PersonInput {
	return PersonInput{
		FirstName: row.FirstName, MiddleName: row.MiddleName, LastName: row.LastName, MaidenName: row.MaidenName,
		Side: row.Side, BirthPlace: row.BirthPlace, Gender: row.Gender,
		BirthDate: formatDate(row.BirthDate, row.BirthDatePrecision), BirthDatePrecision: row.BirthDatePrecision,
		DeathDate: formatDate(row.DeathDate, row.DeathDatePrecision), DeathDatePrecision: row.DeathDatePrecision,
		IsDeceased: row.IsDeceased, Bio: row.Bio,
	}
}

func toPerson(row gen.GetPersonRow) Person {
	p := Person{
		ID: row.ID, FirstName: row.FirstName, MiddleName: row.MiddleName, LastName: row.LastName, MaidenName: row.MaidenName,
		Side: row.Side, BirthPlace: row.BirthPlace, Gender: row.Gender, IsDeceased: row.IsDeceased, Bio: row.Bio,
		DeathDate: formatDate(row.DeathDate, row.DeathDatePrecision), DeathDatePrecision: row.DeathDatePrecision,
	}
	// The precision only goes out with a date the viewer may see.
	if row.BirthDate != "" {
		p.BirthDate, p.BirthDatePrecision = &row.BirthDate, row.BirthDatePrecision
	}
	return p
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

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}
