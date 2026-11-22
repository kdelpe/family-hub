package event

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestCreateRefusesAnInvalidEvent(t *testing.T) {
	tests := []struct {
		name       string
		in         Input
		wantErrMsg string
	}{
		{name: "no title", in: Input{Title: "  "}, wantErrMsg: "a title is required"},
		{name: "title too long", in: Input{Title: strings.Repeat("é", maxTitleLength+1)}, wantErrMsg: "the title is longer than 200 characters"},
		{name: "a line break in the title", in: Input{Title: "Wedding\nday"}, wantErrMsg: "the title contains a character that isn't allowed"},
		{name: "a tab in the location", in: Input{Title: "Wedding", Location: new("Jac\tmel")}, wantErrMsg: "the location contains a character that isn't allowed"},
		{name: "a NUL in the description", in: Input{Title: "Wedding", Description: new("A\x00day")}, wantErrMsg: "the description contains a character that isn't allowed"},
		{name: "description too long", in: Input{Title: "Wedding", Description: new(strings.Repeat("a", maxDescriptionLength+1))},
			wantErrMsg: "the description is longer than 5000 characters"},
		{name: "a date without its precision", in: Input{Title: "Wedding", Date: new("1962")}, wantErrMsg: "give the date and its precision together"},
		{name: "a precision without its date", in: Input{Title: "Wedding", DatePrecision: new("year")}, wantErrMsg: "give the date and its precision together"},
		{name: "unknown precision", in: Input{Title: "Wedding", Date: new("1962"), DatePrecision: new("decade")},
			wantErrMsg: "the date precision must be day, month, year or circa"},
		{name: "a date that doesn't match its precision", in: Input{Title: "Wedding", Date: new("1962-06"), DatePrecision: new("year")},
			wantErrMsg: "the date must look like 2006 for precision year"},
		{name: "not a date", in: Input{Title: "Wedding", Date: new("1962-02-30"), DatePrecision: new("day")},
			wantErrMsg: "the date must look like 2006-01-02 for precision day"},
		{name: "year zero", in: Input{Title: "Wedding", Date: new("0000"), DatePrecision: new("circa")},
			wantErrMsg: "the date must look like 2006 for precision circa"},
		{name: "too many people", in: Input{Title: "Wedding", PersonIDs: make([]uuid.UUID, maxPeople+1)}, wantErrMsg: "tag at most 100 people"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(nil).Create(t.Context(), uuid.New(), tt.in)

			require.ErrorIs(t, err, ErrInvalidEvent)
			assert.Equal(t, "invalid event: "+tt.wantErrMsg, err.Error())
		})
	}
}

func TestCreateAndGet(t *testing.T) {
	svc, pool, admin := newService(t)
	rosa, ana := insertPerson(t, pool, "Rosa", "Pierre"), insertPerson(t, pool, "Ana", "Pluviose")
	next := time.Now().AddDate(1, 0, 0).Format("2006")

	tests := []struct {
		name string
		in   Input
		want Event
	}{
		{
			name: "everything, trimmed, people A–Z and named once",
			in: Input{Title: " Wedding in Jacmel ", Date: new("1962-06-02"), DatePrecision: new("day"), Location: new(" Jacmel "),
				Description: new("Dancing\nall night."), PersonIDs: []uuid.UUID{rosa, ana, rosa}},
			want: Event{Title: "Wedding in Jacmel", Date: new("1962-06-02"), DatePrecision: new("day"), Location: new("Jacmel"),
				Description: new("Dancing\nall night."), People: []Person{{ID: ana, FirstName: "Ana", LastName: new("Pluviose")}, {ID: rosa, FirstName: "Rosa", LastName: new("Pierre")}}},
		},
		{
			name: "only a title; blank text is no text",
			in:   Input{Title: "Moved to Brooklyn", Location: new("  ")},
			want: Event{Title: "Moved to Brooklyn", People: []Person{}},
		},
		{
			name: "a reunion planned for next year",
			in:   Input{Title: "Reunion", Date: &next, DatePrecision: new("year")},
			want: Event{Title: "Reunion", Date: &next, DatePrecision: new("year"), People: []Person{}},
		},
		{
			name: "about a year",
			in:   Input{Title: "First house", Date: new("1975"), DatePrecision: new("circa")},
			want: Event{Title: "First house", Date: new("1975"), DatePrecision: new("circa"), People: []Person{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.Create(t.Context(), admin, tt.in)
			require.NoError(t, err)
			got, err := svc.Get(t.Context(), created.ID)
			require.NoError(t, err)

			assert.Equal(t, created, got)
			tt.want.ID, tt.want.CreatedAt = got.ID, got.CreatedAt
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateRefusesSomeoneNotInTheTree(t *testing.T) {
	svc, pool, admin := newService(t)
	gone := insertPerson(t, pool, "Gone", "Pierre")
	_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)

	for _, id := range []uuid.UUID{uuid.New(), gone} {
		_, err := svc.Create(t.Context(), admin, Input{Title: "Wedding", PersonIDs: []uuid.UUID{id}})

		require.ErrorIs(t, err, ErrInvalidEvent)
		assert.EqualError(t, err, "invalid event: someone tagged isn't in the tree")
	}
	events, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestListNewestFirstUndatedLast(t *testing.T) {
	svc, _, admin := newService(t)
	for _, in := range []Input{
		{Title: "Undated, older"},
		{Title: "1962", Date: new("1962"), DatePrecision: new("year")},
		{Title: "Undated, newer"},
		{Title: "June 1975", Date: new("1975-06"), DatePrecision: new("month")},
		{Title: "1980", Date: new("1980-01-05"), DatePrecision: new("day")},
	} {
		_, err := svc.Create(t.Context(), admin, in)
		require.NoError(t, err)
	}

	events, err := svc.List(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"1980", "June 1975", "1962", "Undated, newer", "Undated, older"}, titles(events))
}

func TestPeopleDeletedFromTheTreeDropOut(t *testing.T) {
	svc, pool, admin := newService(t)
	rosa, ana := insertPerson(t, pool, "Rosa", "Pierre"), insertPerson(t, pool, "Ana", "Pluviose")
	created, err := svc.Create(t.Context(), admin, Input{Title: "Wedding", PersonIDs: []uuid.UUID{rosa, ana}})
	require.NoError(t, err)

	_, err = pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, rosa)
	require.NoError(t, err)

	got, err := svc.Get(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, []Person{{ID: ana, FirstName: "Ana", LastName: new("Pluviose")}}, got.People)
}

func TestUpdate(t *testing.T) {
	svc, pool, admin := newService(t)
	rosa, ana := insertPerson(t, pool, "Rosa", "Pierre"), insertPerson(t, pool, "Ana", "Pluviose")

	tests := []struct {
		name  string
		patch Patch
		check func(t *testing.T, got Event)
	}{
		{
			name:  "only the title changes",
			patch: Patch{Title: family.Opt[string]{Set: true, Value: "Rosa's wedding"}},
			check: func(t *testing.T, got Event) {
				assert.Equal(t, "Rosa's wedding", got.Title)
				assert.Equal(t, new("1962-06-02"), got.Date)
				assert.Equal(t, new("Jacmel"), got.Location)
				assert.Len(t, got.People, 1)
			},
		},
		{
			name: "a vaguer date",
			patch: Patch{Date: family.Opt[*string]{Set: true, Value: new("1962")},
				DatePrecision: family.Opt[*string]{Set: true, Value: new("circa")}},
			check: func(t *testing.T, got Event) {
				assert.Equal(t, new("1962"), got.Date)
				assert.Equal(t, new("circa"), got.DatePrecision)
			},
		},
		{
			name:  "who was there is replaced",
			patch: Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true, Value: []uuid.UUID{ana}}},
			check: func(t *testing.T, got Event) {
				assert.Equal(t, []Person{{ID: ana, FirstName: "Ana", LastName: new("Pluviose")}}, got.People)
			},
		},
		{
			name:  "nobody tagged any more",
			patch: Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true}},
			check: func(t *testing.T, got Event) { assert.Empty(t, got.People) },
		},
		{
			name:  "the place cleared",
			patch: Patch{Location: family.Opt[*string]{Set: true}},
			check: func(t *testing.T, got Event) { assert.Nil(t, got.Location) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.Create(t.Context(), admin, Input{Title: "Wedding", Date: new("1962-06-02"), DatePrecision: new("day"),
				Location: new("Jacmel"), PersonIDs: []uuid.UUID{rosa}})
			require.NoError(t, err)

			got, err := svc.Update(t.Context(), created.ID, tt.patch)

			require.NoError(t, err)
			tt.check(t, got)
			again, err := svc.Get(t.Context(), created.ID)
			require.NoError(t, err)
			assert.Equal(t, got, again)
		})
	}
}

func TestUpdateRefusesAndKeepsTheEvent(t *testing.T) {
	svc, pool, admin := newService(t)
	rosa := insertPerson(t, pool, "Rosa", "Pierre")
	created, err := svc.Create(t.Context(), admin, Input{Title: "Wedding", PersonIDs: []uuid.UUID{rosa}})
	require.NoError(t, err)

	tests := []struct {
		name  string
		patch Patch
		want  string
	}{
		{name: "a blank title", patch: Patch{Title: family.Opt[string]{Set: true, Value: " "}}, want: "invalid event: a title is required"},
		{name: "a date without a precision", patch: Patch{Date: family.Opt[*string]{Set: true, Value: new("1962")}},
			want: "invalid event: give the date and its precision together"},
		{name: "someone not in the tree", patch: Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true, Value: []uuid.UUID{uuid.New()}}},
			want: "invalid event: someone tagged isn't in the tree"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(t.Context(), created.ID, tt.patch)

			require.ErrorIs(t, err, ErrInvalidEvent)
			assert.EqualError(t, err, tt.want)
			got, err := svc.Get(t.Context(), created.ID)
			require.NoError(t, err)
			assert.Equal(t, created, got)
		})
	}
}

func TestDelete(t *testing.T) {
	svc, _, admin := newService(t)
	created, err := svc.Create(t.Context(), admin, Input{Title: "Wedding"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(t.Context(), created.ID))

	_, err = svc.Get(t.Context(), created.ID)
	require.ErrorIs(t, err, ErrEventNotFound)
	events, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, events)
	_, err = svc.Update(t.Context(), created.ID, Patch{Title: family.Opt[string]{Set: true, Value: "Back"}})
	require.ErrorIs(t, err, ErrEventNotFound)
	require.ErrorIs(t, svc.Delete(t.Context(), created.ID), ErrEventNotFound)
}

func TestUnknownEvent(t *testing.T) {
	svc, _, _ := newService(t)
	id := uuid.New()

	_, err := svc.Get(t.Context(), id)
	require.ErrorIs(t, err, ErrEventNotFound)
	_, err = svc.Update(t.Context(), id, Patch{})
	require.ErrorIs(t, err, ErrEventNotFound)
	require.ErrorIs(t, svc.Delete(t.Context(), id), ErrEventNotFound)
}

func newService(t *testing.T) (*Service, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	var admin uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role) VALUES ('admin@example.com', 'admin') RETURNING id`).Scan(&admin))
	return NewService(pool), pool, admin
}

func insertPerson(t *testing.T, pool *pgxpool.Pool, first, last string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES ($1, $2) RETURNING id`, first, last).Scan(&id))
	return id
}

func titles(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Title
	}
	return out
}
