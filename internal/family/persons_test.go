package family

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
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
		{name: "a NUL in the first name", in: PersonInput{FirstName: "Ro\x00sa"}, wantErrMsg: "the first name contains a character that isn't allowed"},
		{name: "a NUL in the bio", in: PersonInput{FirstName: "Rosa", Bio: new("Tells\x00stories")}, wantErrMsg: "the bio contains a character that isn't allowed"},
		{name: "a last name that isn't UTF-8", in: PersonInput{FirstName: "Rosa", LastName: new("Pi\xffrre")}, wantErrMsg: "the last name contains a character that isn't allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(nil).CreatePerson(t.Context(), uuid.New(), tt.in)

			require.ErrorIs(t, err, ErrInvalidPerson)
			assert.Equal(t, "invalid person: "+tt.wantErrMsg, err.Error())
		})
	}
}

func TestPartialDatesAreComparedAsRanges(t *testing.T) {
	tests := []struct {
		name                  string
		birth, birthPrecision string
		death, deathPrecision string
		wantErr               bool
	}{
		{name: "died the year of a known birthday", birth: "1950-03-14", birthPrecision: "day", death: "1950", deathPrecision: "year"},
		{name: "died the month of a known birthday", birth: "1950-03-14", birthPrecision: "day", death: "1950-03", deathPrecision: "month"},
		{name: "died the day of birth", birth: "1950-03-14", birthPrecision: "day", death: "1950-03-14", deathPrecision: "day"},
		{name: "died early in the birth year", birth: "1950", birthPrecision: "year", death: "1950-01-02", deathPrecision: "day"},
		{name: "died the year of a circa birth", birth: "1950", birthPrecision: "circa", death: "1950-06", deathPrecision: "month"},
		{name: "died the day before birth", birth: "1950-03-14", birthPrecision: "day", death: "1950-03-13", deathPrecision: "day", wantErr: true},
		{name: "died the month before birth", birth: "1950-03-14", birthPrecision: "day", death: "1950-02", deathPrecision: "month", wantErr: true},
		{name: "died the year before birth", birth: "1950-01", birthPrecision: "month", death: "1949", deathPrecision: "circa", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalize(PersonInput{FirstName: "Rosa", IsDeceased: true,
				BirthDate: &tt.birth, BirthDatePrecision: &tt.birthPrecision, DeathDate: &tt.death, DeathDatePrecision: &tt.deathPrecision})

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, "invalid person: the death date is before the birth date")
		})
	}
}

func TestCreateStoresTrimmedNames(t *testing.T) {
	svc, admin := newService(t)

	got, err := svc.CreatePerson(t.Context(), admin, PersonInput{
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

	svc, admin := newService(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosa", BirthDate: &tt.date, BirthDatePrecision: &tt.precision, IsDeceased: tt.deceased})
			require.NoError(t, err)
			viewer := map[string]Viewer{"member": {}, "admin": {IsAdmin: true}, "self": {PersonID: &created.ID}}[tt.viewer]

			got, err := svc.GetPerson(t.Context(), viewer, created.ID)
			require.NoError(t, err)
			listed, err := svc.ListPersons(t.Context(), viewer, ListInput{Query: "Rosa"})
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
	svc, admin := newService(t)
	for _, in := range []PersonInput{
		{FirstName: "Rosa", LastName: new("Pierre"), Side: new("first")},
		{FirstName: "Jean", LastName: new("Pierre"), Side: new("second"), IsDeceased: true},
		{FirstName: "Marie", Side: new("both")},
		{FirstName: "Paul", LastName: new("100%_sure")},
	} {
		_, err := svc.CreatePerson(t.Context(), admin, in)
		require.NoError(t, err)
	}
	gone, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosalie"})
	require.NoError(t, err)
	require.NoError(t, svc.DeletePerson(t.Context(), gone.ID))

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
			got, err := svc.ListPersons(t.Context(), Viewer{}, tt.in)

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
		{name: "a NUL in the search", in: ListInput{Query: "Ro\x00sa"}},
		{name: "a search that isn't UTF-8", in: ListInput{Query: "\xff"}},
		{name: "unknown side", in: ListInput{Side: "north"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(nil).ListPersons(t.Context(), Viewer{}, tt.in)

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

	svc, admin := newService(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, err := svc.CreatePerson(t.Context(), admin, PersonInput{
				FirstName: "Rosa", MiddleName: new("Anne"), LastName: new("Pierre"), Bio: new("Old bio"),
				BirthDate: new("1950-03-14"), BirthDatePrecision: new("day"),
			})
			require.NoError(t, err)

			got, err := svc.UpdatePerson(t.Context(), before.ID, tt.patch)

			stored, getErr := svc.GetPerson(t.Context(), Viewer{IsAdmin: true}, before.ID)
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
	svc, admin := newService(t)
	rosa, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosa"})
	require.NoError(t, err)

	require.NoError(t, svc.DeletePerson(t.Context(), rosa.ID))

	_, err = svc.GetPerson(t.Context(), Viewer{IsAdmin: true}, rosa.ID)
	assert.ErrorIs(t, err, ErrPersonNotFound, "a deleted person is gone from reads")
	_, err = svc.UpdatePerson(t.Context(), rosa.ID, PersonPatch{})
	assert.ErrorIs(t, err, ErrPersonNotFound, "and from edits")
	assert.ErrorIs(t, svc.DeletePerson(t.Context(), rosa.ID), ErrPersonNotFound, "deleting twice")

	restored, err := svc.RestorePerson(t.Context(), rosa.ID)
	require.NoError(t, err)
	assert.Equal(t, rosa, restored)
	_, err = svc.RestorePerson(t.Context(), rosa.ID)
	assert.ErrorIs(t, err, ErrPersonNotFound, "restoring someone not deleted")
}

func TestDeleteGuardsALinkedLogin(t *testing.T) {
	tests := []struct {
		name     string
		disabled bool
		wantErr  error
	}{
		{name: "refused while the login is enabled", wantErr: ErrPersonHasLogin},
		{name: "a disabled login loses the link", disabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			rosa, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosa"})
			require.NoError(t, err)
			_, err = svc.pool.Exec(t.Context(), `UPDATE users SET person_id = $1, disabled_at = CASE WHEN $2 THEN now() END WHERE id = $3`, rosa.ID, tt.disabled, admin)
			require.NoError(t, err)

			err = svc.DeletePerson(t.Context(), rosa.ID)

			var linked *uuid.UUID
			require.NoError(t, svc.pool.QueryRow(t.Context(), `SELECT person_id FROM users WHERE id = $1`, admin).Scan(&linked))
			_, getErr := svc.GetPerson(t.Context(), Viewer{IsAdmin: true}, rosa.ID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.NoError(t, getErr, "still in the tree")
				assert.Equal(t, &rosa.ID, linked, "still linked")
				return
			}
			require.NoError(t, err)
			assert.ErrorIs(t, getErr, ErrPersonNotFound)
			assert.Nil(t, linked)
		})
	}
}

func TestUnknownPersonIsNotFound(t *testing.T) {
	svc, _ := newService(t)
	id := uuid.New()

	_, getErr := svc.GetPerson(t.Context(), Viewer{}, id)
	_, updateErr := svc.UpdatePerson(t.Context(), id, PersonPatch{})
	deleteErr := svc.DeletePerson(t.Context(), id)
	_, restoreErr := svc.RestorePerson(t.Context(), id)

	for _, err := range []error{getErr, updateErr, deleteErr, restoreErr} {
		assert.ErrorIs(t, err, ErrPersonNotFound)
	}
}

func TestPersonWritesAreBusyWhileARowStaysLocked(t *testing.T) {
	const lockPersons = `SELECT 1 FROM persons FOR UPDATE`
	tests := []struct {
		name    string
		lock    string
		deleted bool
		write   func(ctx context.Context, s *Service, id, admin uuid.UUID) error
	}{
		{name: "create, while the creator's user row is locked", lock: `SELECT 1 FROM users FOR UPDATE`,
			write: func(ctx context.Context, s *Service, _, admin uuid.UUID) error {
				_, err := s.CreatePerson(ctx, admin, PersonInput{FirstName: "Jean"})
				return err
			}},
		{name: "update", lock: lockPersons,
			write: func(ctx context.Context, s *Service, id, _ uuid.UUID) error {
				_, err := s.UpdatePerson(ctx, id, PersonPatch{})
				return err
			}},
		{name: "delete", lock: lockPersons,
			write: func(ctx context.Context, s *Service, id, _ uuid.UUID) error { return s.DeletePerson(ctx, id) }},
		{name: "restore", lock: lockPersons, deleted: true,
			write: func(ctx context.Context, s *Service, id, _ uuid.UUID) error {
				_, err := s.RestorePerson(ctx, id)
				return err
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			svc, admin := NewService(pool), insertAdmin(t, pool)
			svc.lockTimeout = 100 * time.Millisecond
			rosa, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosa"})
			require.NoError(t, err)
			if tt.deleted {
				require.NoError(t, svc.DeletePerson(t.Context(), rosa.ID))
			}
			holder, err := pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
			_, err = holder.Exec(t.Context(), tt.lock)
			require.NoError(t, err)

			err = tt.write(t.Context(), svc, rosa.ID, admin)

			assert.ErrorIs(t, err, auth.ErrBusy)
		})
	}
}

// newService returns the service on a fresh schema, and an admin user to create people as.
func newService(t *testing.T) (*Service, uuid.UUID) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	return NewService(pool), insertAdmin(t, pool)
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
