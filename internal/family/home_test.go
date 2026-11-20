package family

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestBirthdayDays(t *testing.T) {
	tests := []struct {
		name     string
		today    string
		monthDay string
		wantOn   string
		wantIn   int
		wantNone bool
	}{
		{name: "today", today: "2026-06-06", monthDay: "06-06", wantOn: "2026-06-06", wantIn: 0},
		{name: "two weeks out", today: "2026-06-06", monthDay: "06-20", wantOn: "2026-06-20", wantIn: 14},
		{name: "past the window", today: "2026-06-06", monthDay: "06-21", wantNone: true},
		{name: "yesterday", today: "2026-06-06", monthDay: "06-05", wantNone: true},
		{name: "across the new year", today: "2026-12-28", monthDay: "01-05", wantOn: "2027-01-05", wantIn: 8},
		{name: "February 29 on February 28 in a common year", today: "2026-02-20", monthDay: "02-29", wantOn: "2026-02-28", wantIn: 8},
		{name: "February 29 in a leap year", today: "2028-02-20", monthDay: "02-29", wantOn: "2028-02-29", wantIn: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			on, ok := birthdayDays(day(tt.today))[tt.monthDay]

			if tt.wantNone {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, day(tt.wantOn), on.date)
			assert.Equal(t, tt.wantIn, on.in)
		})
	}
}

func TestHome(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc, admin := NewService(pool), insertAdmin(t, pool)
	newYork, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	// 9pm on November 13 in New York is already November 14 in UTC.
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, newYork)
	person := func(first string, in PersonInput) uuid.UUID {
		in.FirstName = first
		p, err := svc.CreatePerson(t.Context(), admin, in)
		require.NoError(t, err)
		return p.ID
	}
	born := func(date, precision string) PersonInput {
		return PersonInput{BirthDate: new(date), BirthDatePrecision: new(precision)}
	}
	nadege := person("Nadège", born("1970-11-19", "day"))
	today := person("Today", born("1990-11-13", "day"))
	person("Tomorrow in UTC only", born("1990-11-28", "day"))
	inTwoWeeks := person("Two weeks", born("1985-11-27", "day"))
	person("Too late", born("1985-11-28", "day"))
	person("Passed", born("1985-11-12", "day"))
	person("Month only", born("1985-11", "month"))
	person("Deceased", PersonInput{BirthDate: new("1920-11-15"), BirthDatePrecision: new("day"), IsDeceased: true})
	deleted := person("Deleted", born("1980-11-16", "day"))
	require.NoError(t, svc.DeletePerson(t.Context(), deleted))
	_, err = pool.Exec(t.Context(), `UPDATE persons SET created_at = $1 WHERE first_name <> 'Two weeks'`, now.Add(-31*24*time.Hour))
	require.NoError(t, err)
	recent := person("Recent", PersonInput{})
	viewer := person("Viewer, also new", PersonInput{})
	for id, ago := range map[uuid.UUID]time.Duration{recent: time.Hour, viewer: 2 * time.Hour, inTwoWeeks: 29 * 24 * time.Hour} {
		_, err = pool.Exec(t.Context(), `UPDATE persons SET created_at = $1 WHERE id = $2`, now.Add(-ago), id)
		require.NoError(t, err)
	}

	home, err := svc.Home(t.Context(), Viewer{PersonID: &viewer}, now, newYork)

	require.NoError(t, err)
	assert.Equal(t, []Birthday{
		{ID: today, FirstName: "Today", On: day("2026-11-13"), InDays: 0},
		{ID: nadege, FirstName: "Nadège", On: day("2026-11-19"), InDays: 6},
		{ID: inTwoWeeks, FirstName: "Two weeks", On: day("2026-11-27"), InDays: 14},
	}, home.Birthdays, "living, full dates only, from the family's today through two weeks out")
	require.Len(t, home.Newcomers, 2)
	assert.Equal(t, []string{"Recent", "Two weeks"}, []string{home.Newcomers[0].FirstName, home.Newcomers[1].FirstName},
		"added in the last 30 days, newest first, other than the viewer")
}

func TestHomeShowsAtMostFiveNewcomers(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc, admin := NewService(pool), insertAdmin(t, pool)
	addPeople(t, svc, admin, "A", "B", "C", "D", "E", "F")

	home, err := svc.Home(t.Context(), Viewer{}, time.Now(), time.UTC)

	require.NoError(t, err)
	assert.Len(t, home.Newcomers, maxNewcomers)
	assert.Equal(t, "F", home.Newcomers[0].FirstName)
}
