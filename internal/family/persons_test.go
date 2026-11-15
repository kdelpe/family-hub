package family

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestCreateRefusesAnInvalidPerson(t *testing.T) {
	tests := []struct {
		name       string
		in         PersonInput
		wantErrMsg string
	}{
		{name: "no first name", in: PersonInput{FirstName: "   "}, wantErrMsg: "a first name is required"},
		{name: "first name too long", in: PersonInput{FirstName: strings.Repeat("é", maxNameLength+1)}, wantErrMsg: "the first name is longer than 100 characters"},
		{name: "last name too long", in: PersonInput{FirstName: "Rosa", LastName: new(strings.Repeat("a", maxNameLength+1))}, wantErrMsg: "the last name is longer than 100 characters"},
		{name: "bio too long", in: PersonInput{FirstName: "Rosa", Bio: new(strings.Repeat("a", maxBioLength+1))}, wantErrMsg: "the bio is longer than 5000 characters"},
		{name: "unknown side", in: PersonInput{FirstName: "Rosa", Side: new("north")}, wantErrMsg: "side must be one of first, second, both, married_in"},
		{name: "unknown gender", in: PersonInput{FirstName: "Rosa", Gender: new("x")}, wantErrMsg: "gender must be one of male, female, other, unknown"},
		{name: "a date without its precision", in: PersonInput{FirstName: "Rosa", BirthDate: new("1950")}, wantErrMsg: "give the birth date and its precision together"},
		{name: "a precision without its date", in: PersonInput{FirstName: "Rosa", DeathDatePrecision: new("year"), IsDeceased: true}, wantErrMsg: "give the death date and its precision together"},
		{name: "unknown precision", in: PersonInput{FirstName: "Rosa", BirthDate: new("1950"), BirthDatePrecision: new("decade")},
			wantErrMsg: "the birth date precision must be day, month, year or circa"},
		{name: "a date that doesn't match its precision", in: PersonInput{FirstName: "Rosa", BirthDate: new("1950-03"), BirthDatePrecision: new("year")},
			wantErrMsg: "the birth date must look like 2006 for precision year"},
		{name: "not a date", in: PersonInput{FirstName: "Rosa", BirthDate: new("1950-02-30"), BirthDatePrecision: new("day")},
			wantErrMsg: "the birth date must look like 2006-01-02 for precision day"},
		{name: "a future date", in: PersonInput{FirstName: "Rosa", BirthDate: new("2999"), BirthDatePrecision: new("year")}, wantErrMsg: "the birth date is in the future"},
		{name: "a death date for someone living", in: PersonInput{FirstName: "Rosa", DeathDate: new("2001"), DeathDatePrecision: new("year")},
			wantErrMsg: "a death date needs is_deceased"},
		{name: "died before being born", in: PersonInput{FirstName: "Rosa", BirthDate: new("1950"), BirthDatePrecision: new("year"),
			DeathDate: new("1949"), DeathDatePrecision: new("year"), IsDeceased: true}, wantErrMsg: "the death date is before the birth date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPersons(nil).Create(t.Context(), uuid.New(), tt.in)

			require.ErrorIs(t, err, ErrInvalidPerson)
			assert.Equal(t, "invalid person: "+tt.wantErrMsg, err.Error())
		})
	}
}

func TestCreateStoresTrimmedNames(t *testing.T) {
	persons, admin := newPersons(t)

	got, err := persons.Create(t.Context(), admin, PersonInput{
		FirstName: "  Rosa ", LastName: new("   "), MaidenName: new(" Pierre "), Bio: new(""),
		BirthDate: new("1950-03-14"), BirthDatePrecision: new("day"),
		DeathDate: new("2001-06"), DeathDatePrecision: new("month"), IsDeceased: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "Rosa", got.FirstName)
	assert.Nil(t, got.LastName, "a blank surname is stored as NULL")
	assert.Equal(t, new("Pierre"), got.MaidenName)
	assert.Nil(t, got.Bio)
	assert.Equal(t, new("1950-03-14"), got.BirthDate)
	assert.Equal(t, new("2001-06"), got.DeathDate)
	assert.Equal(t, new("month"), got.DeathDatePrecision)
}

func TestBirthYearIsHiddenForLivingPeople(t *testing.T) {
	tests := []struct {
		name          string
		date          string
		precision     string
		deceased      bool
		viewer        string
		wantDate      *string
		wantPrecision *string
	}{
		{name: "a member sees a living person's day and month", date: "1950-03-14", precision: "day", viewer: "member", wantDate: new("--03-14"), wantPrecision: new("day")},
		{name: "a member sees a living person's month", date: "1950-03", precision: "month", viewer: "member", wantDate: new("--03"), wantPrecision: new("month")},
		{name: "a member sees nothing of a living person's year", date: "1950", precision: "year", viewer: "member"},
		{name: "a member sees nothing of a living person's circa year", date: "1950", precision: "circa", viewer: "member"},
		{name: "a member sees a deceased person's full date", date: "1950-03-14", precision: "day", deceased: true, viewer: "member", wantDate: new("1950-03-14"), wantPrecision: new("day")},
		{name: "a member sees a deceased person's circa year", date: "1950", precision: "circa", deceased: true, viewer: "member", wantDate: new("1950"), wantPrecision: new("circa")},
		{name: "an admin sees a living person's full date", date: "1950-03-14", precision: "day", viewer: "admin", wantDate: new("1950-03-14"), wantPrecision: new("day")},
		{name: "an admin sees a living person's year", date: "1950", precision: "year", viewer: "admin", wantDate: new("1950"), wantPrecision: new("year")},
		{name: "people see their own year", date: "1950-03", precision: "month", viewer: "self", wantDate: new("1950-03"), wantPrecision: new("month")},
	}

	persons, admin := newPersons(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := persons.Create(t.Context(), admin, PersonInput{FirstName: "Rosa", BirthDate: &tt.date, BirthDatePrecision: &tt.precision, IsDeceased: tt.deceased})
			require.NoError(t, err)
			viewer := map[string]Viewer{"member": {}, "admin": {IsAdmin: true}, "self": {PersonID: &created.ID}}[tt.viewer]

			got, err := persons.Get(t.Context(), viewer, created.ID)
			require.NoError(t, err)
			listed, err := persons.List(t.Context(), viewer, ListInput{Query: "Rosa"})
			require.NoError(t, err)

			assert.Equal(t, tt.wantDate, got.BirthDate)
			assert.Equal(t, tt.wantPrecision, got.BirthDatePrecision)
			i := index(listed, created.ID)
			require.NotEqual(t, -1, i)
			assert.Equal(t, got, listed[i], "search masks the same way")
		})
	}
}

func TestListSearchesAndFilters(t *testing.T) {
	persons, admin := newPersons(t)
	for _, in := range []PersonInput{
		{FirstName: "Rosa", LastName: new("Pierre"), Side: new("first")},
		{FirstName: "Jean", LastName: new("Pierre"), Side: new("second"), IsDeceased: true},
		{FirstName: "Marie", Side: new("both")},
		{FirstName: "Paul", LastName: new("100%_sure")},
	} {
		_, err := persons.Create(t.Context(), admin, in)
		require.NoError(t, err)
	}
	gone, err := persons.Create(t.Context(), admin, PersonInput{FirstName: "Rosalie"})
	require.NoError(t, err)
	require.NoError(t, persons.Delete(t.Context(), gone.ID))

	tests := []struct {
		name string
		in   ListInput
		want []string
	}{
		{name: "everyone A–Z, without the deleted", want: []string{"Jean", "Marie", "Paul", "Rosa"}},
		{name: "part of a name", in: ListInput{Query: "  ros "}, want: []string{"Rosa"}},
		{name: "a surname", in: ListInput{Query: "pierre"}, want: []string{"Jean", "Rosa"}},
		{name: "someone without a surname", in: ListInput{Query: "Marie"}, want: []string{"Marie"}},
		{name: "a misspelling", in: ListInput{Query: "Rosa Piere"}, want: []string{"Rosa"}},
		{name: "wildcards are plain characters", in: ListInput{Query: "%_"}, want: []string{"Paul"}},
		{name: "a side", in: ListInput{Side: "second"}, want: []string{"Jean"}},
		{name: "the living", in: ListInput{Deceased: new(false)}, want: []string{"Marie", "Paul", "Rosa"}},
		{name: "the departed", in: ListInput{Deceased: new(true)}, want: []string{"Jean"}},
		{name: "nobody matches", in: ListInput{Query: "xyz"}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := persons.List(t.Context(), Viewer{}, tt.in)

			require.NoError(t, err)
			names := []string{}
			for _, p := range got {
				names = append(names, p.FirstName)
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

func TestListRefusesABadFilter(t *testing.T) {
	tests := []struct {
		name string
		in   ListInput
	}{
		{name: "search too long", in: ListInput{Query: strings.Repeat("a", maxQueryLength+1)}},
		{name: "unknown side", in: ListInput{Side: "north"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPersons(nil).List(t.Context(), Viewer{}, tt.in)

			assert.ErrorIs(t, err, ErrInvalidFilter)
		})
	}
}

func TestUpdateChangesOnlyTheFieldsSent(t *testing.T) {
	tests := []struct {
		name    string
		patch   PersonPatch
		want    func(p *Person)
		wantErr error
	}{
		{name: "a bio", patch: PersonPatch{Bio: Opt[*string]{Set: true, Value: new(" Tells the best stories. ")}},
			want: func(p *Person) { p.Bio = new("Tells the best stories.") }},
		{name: "clearing a middle name", patch: PersonPatch{MiddleName: Opt[*string]{Set: true}},
			want: func(p *Person) { p.MiddleName = nil }},
		{name: "a death, with its date", patch: PersonPatch{IsDeceased: Opt[bool]{Set: true, Value: true},
			DeathDate: Opt[*string]{Set: true, Value: new("2020")}, DeathDatePrecision: Opt[*string]{Set: true, Value: new("year")}},
			want: func(p *Person) { p.IsDeceased, p.DeathDate, p.DeathDatePrecision = true, new("2020"), new("year") }},
		{name: "nothing sent", want: func(*Person) {}},
		{name: "a blank first name", patch: PersonPatch{FirstName: Opt[string]{Set: true, Value: " "}}, wantErr: ErrInvalidPerson},
		{name: "a birth date without its precision", patch: PersonPatch{BirthDatePrecision: Opt[*string]{Set: true}}, wantErr: ErrInvalidPerson},
	}

	persons, admin := newPersons(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, err := persons.Create(t.Context(), admin, PersonInput{
				FirstName: "Rosa", MiddleName: new("Anne"), LastName: new("Pierre"), Bio: new("Old bio"),
				BirthDate: new("1950-03-14"), BirthDatePrecision: new("day"),
			})
			require.NoError(t, err)

			got, err := persons.Update(t.Context(), before.ID, tt.patch)

			stored, getErr := persons.Get(t.Context(), Viewer{IsAdmin: true}, before.ID)
			require.NoError(t, getErr)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, stored, "nothing changed")
				return
			}
			require.NoError(t, err)
			want := before
			tt.want(&want)
			assert.Equal(t, want, got)
			assert.Equal(t, want, stored)
		})
	}
}

func TestDeleteAndRestore(t *testing.T) {
	persons, admin := newPersons(t)
	rosa, err := persons.Create(t.Context(), admin, PersonInput{FirstName: "Rosa"})
	require.NoError(t, err)

	require.NoError(t, persons.Delete(t.Context(), rosa.ID))

	_, err = persons.Get(t.Context(), Viewer{IsAdmin: true}, rosa.ID)
	assert.ErrorIs(t, err, ErrPersonNotFound, "a deleted person is gone from reads")
	_, err = persons.Update(t.Context(), rosa.ID, PersonPatch{})
	assert.ErrorIs(t, err, ErrPersonNotFound, "and from edits")
	assert.ErrorIs(t, persons.Delete(t.Context(), rosa.ID), ErrPersonNotFound, "deleting twice")

	restored, err := persons.Restore(t.Context(), rosa.ID)
	require.NoError(t, err)
	assert.Equal(t, rosa, restored)
	_, err = persons.Restore(t.Context(), rosa.ID)
	assert.ErrorIs(t, err, ErrPersonNotFound, "restoring someone not deleted")
}

func TestUnknownPersonIsNotFound(t *testing.T) {
	persons, _ := newPersons(t)
	id := uuid.New()

	_, getErr := persons.Get(t.Context(), Viewer{}, id)
	_, updateErr := persons.Update(t.Context(), id, PersonPatch{})
	deleteErr := persons.Delete(t.Context(), id)
	_, restoreErr := persons.Restore(t.Context(), id)

	for _, err := range []error{getErr, updateErr, deleteErr, restoreErr} {
		assert.ErrorIs(t, err, ErrPersonNotFound)
	}
}

// newPersons returns the service on a fresh schema, and an admin user to create people as.
func newPersons(t *testing.T) (*Persons, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	return NewPersons(pool), insertAdmin(t, pool)
}

func insertAdmin(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role) VALUES ('admin@example.com', 'admin') RETURNING id`).Scan(&id))
	return id
}

func index(people []Person, id uuid.UUID) int {
	for i, p := range people {
		if p.ID == id {
			return i
		}
	}
	return -1
}
