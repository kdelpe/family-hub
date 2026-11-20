package family

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	// birthdayWindow is how many days ahead Coming up looks, from today (§5.3).
	birthdayWindow = 14
	newcomerWindow = 30 * 24 * time.Hour
	maxNewcomers   = 20
)

// Home is what the home page's cards show (design-notes §4.1).
type Home struct {
	Birthdays []Birthday
	Newcomers []Newcomer
}

// Birthday is a coming birthday: its date this year (or next) and how many days away, never the birth year.
type Birthday struct {
	ID        uuid.UUID
	FirstName string
	LastName  *string
	On        time.Time
	InDays    int
}

// Newcomer is someone added to the tree recently.
type Newcomer struct {
	ID        uuid.UUID
	FirstName string
	LastName  *string
	AddedAt   time.Time
}

// Home reads the birthdays coming up and the people added lately, other than the viewer. Today is now's date
// in loc, the family's timezone (APP_TIMEZONE), so everyone sees the same list.
func (s *Service) Home(ctx context.Context, viewer Viewer, now time.Time, loc *time.Location) (Home, error) {
	var home Home
	err := s.read(ctx, "home", func(q *gen.Queries) error {
		days := birthdayDays(now.In(loc))
		monthDays := make([]string, 0, len(days))
		for md := range days {
			monthDays = append(monthDays, md)
		}
		born, err := q.BirthdaysOn(ctx, monthDays)
		if err != nil {
			return fmt.Errorf("birthdays on: %w", err)
		}
		for _, b := range born {
			on := days[b.MonthDay]
			home.Birthdays = append(home.Birthdays, Birthday{ID: b.ID, FirstName: b.FirstName, LastName: b.LastName, On: on.date, InDays: on.in})
		}
		slices.SortFunc(home.Birthdays, func(a, b Birthday) int {
			return cmp.Or(cmp.Compare(a.InDays, b.InDays), cmp.Compare(a.FirstName, b.FirstName), cmp.Compare(deref(a.LastName), deref(b.LastName)))
		})
		added, err := q.AddedSince(ctx, gen.AddedSinceParams{Since: now.Add(-newcomerWindow), ViewerPersonID: viewer.PersonID, MaxPeople: maxNewcomers})
		if err != nil {
			return fmt.Errorf("added since: %w", err)
		}
		for _, a := range added {
			home.Newcomers = append(home.Newcomers, Newcomer{ID: a.ID, FirstName: a.FirstName, LastName: a.LastName, AddedAt: a.CreatedAt})
		}
		return nil
	})
	if err != nil {
		return Home{}, err
	}
	return home, nil
}

type birthdayDay struct {
	date time.Time
	in   int
}

// birthdayDays maps each birthday ("MM-DD") that falls from today through birthdayWindow days out to that
// date. In a common year, February 28 also stands for February 29 birthdays (§5.3).
func birthdayDays(today time.Time) map[string]birthdayDay {
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	days := make(map[string]birthdayDay, birthdayWindow+2)
	for i := range birthdayWindow + 1 {
		d := start.AddDate(0, 0, i)
		days[d.Format("01-02")] = birthdayDay{date: d, in: i}
		if d.Month() == time.February && d.Day() == 28 && !isLeap(d.Year()) {
			days["02-29"] = birthdayDay{date: d, in: i}
		}
	}
	return days
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
