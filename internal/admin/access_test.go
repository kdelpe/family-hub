package admin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestNormalizeRequest(t *testing.T) {
	tests := []struct {
		name    string
		in      AccessRequestInput
		want    AccessRequestInput
		wantErr string
	}{
		{name: "trimmed, blank names dropped, blank note none",
			in: AccessRequestInput{FullName: " Darline Monestime ", Email: " Darline@Example.com ", Side: new("second"), Relationship: new("daughter"),
				RelatedNames: []string{" Frantz Monestime ", "  "}, Note: new("  ")},
			want: AccessRequestInput{FullName: "Darline Monestime", Email: "darline@example.com", Side: new("second"), Relationship: new("daughter"),
				RelatedNames: []string{"Frantz Monestime"}}},
		{name: "only a name and email", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com"},
			want: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", RelatedNames: []string{}}},
		{name: "a note over lines", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Note: new("Hi\nthere")},
			want: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", RelatedNames: []string{}, Note: new("Hi\nthere")}},

		{name: "a bad email", in: AccessRequestInput{FullName: "Darline", Email: "darline"}, wantErr: "the email must be"},
		{name: "no name", in: AccessRequestInput{FullName: " ", Email: "darline@example.com"}, wantErr: "the full name must be"},
		{name: "a name over two lines", in: AccessRequestInput{FullName: "Dar\nline", Email: "darline@example.com"}, wantErr: "the full name must be"},
		{name: "a name too long", in: AccessRequestInput{FullName: strings.Repeat("a", maxRequestName+1), Email: "darline@example.com"}, wantErr: "the full name must be"},
		{name: "a side that isn't one", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Side: new("both")}, wantErr: "side must be"},
		{name: "a relationship off the list", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Relationship: new("friend")},
			wantErr: "relationship must be"},
		{name: "five names", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", RelatedNames: []string{"a", "b", "c", "d", "e"}},
			wantErr: "at most 4 names"},
		{name: "a related name with a tab", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", RelatedNames: []string{"a\tb"}},
			wantErr: "each name must be"},
		{name: "a note with a tab and Windows line ends", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Note: new("a\tb\r\nc")},
			want: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", RelatedNames: []string{}, Note: new("a\tb\nc")}},
		{name: "a NUL in the note", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Note: new("a\x00b")},
			wantErr: "the note contains a character"},
		{name: "a note too long", in: AccessRequestInput{FullName: "Darline", Email: "darline@example.com", Note: new(strings.Repeat("a", maxRequestNote+1))},
			wantErr: "the note must be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRequest(tt.in)

			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrInvalidAccessRequest)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubmitStoresOnePendingRequestPerEmail(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	first := AccessRequestInput{FullName: "Darline Monestime", Email: "darline@example.com", RelatedNames: []string{"Frantz Monestime"}}

	require.NoError(t, svc.Submit(t.Context(), first))
	require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Someone Else", Email: "DARLINE@example.com"}))

	requests, err := svc.List(t.Context())
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, "Darline Monestime", requests[0].FullName, "the pending one is left as it was")
	assert.Equal(t, "self", requests[0].Source)
}

func TestRepeatsDontSpendTheAddressBudget(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	admin := insertUser(t, pool, "admin@example.com")
	for range accessRequestsPerEmail + 3 {
		require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Junk", Email: "darline@example.com"}))
	}
	_, err := pool.Exec(t.Context(), `UPDATE access_requests SET status = 'declined', reviewed_by = $1, reviewed_at = now()`, admin)
	require.NoError(t, err)

	require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Darline Monestime", Email: "darline@example.com"}))

	assert.Equal(t, 1, countRows(t, pool, `SELECT count(*) FROM access_requests WHERE status = 'pending' AND full_name = 'Darline Monestime'`))
}

func TestRepeatsThatLoseARaceGiveTheirTokenBack(t *testing.T) {
	// Its own pool: the default is as small as 4 connections on a 4-CPU runner, and every repeat must wait at once.
	cfg := testdb.MigratedPool(t, database.MigrateUp).Config()
	cfg.MaxConns = accessRequestsPerEmail + 3
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	svc := NewAccessRequests(pool)
	admin := insertUser(t, pool, "admin@example.com")
	// A request for the address still being stored: every Submit passes the pending check, then waits on it.
	first, err := pool.Begin(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Rollback(context.Background()) })
	_, err = first.Exec(t.Context(), `INSERT INTO access_requests (full_name, email, source) VALUES ('First', 'darline@example.com', 'self')`)
	require.NoError(t, err)
	var firstPID int
	require.NoError(t, first.QueryRow(t.Context(), `SELECT pg_backend_pid()`).Scan(&firstPID))

	const repeats = accessRequestsPerEmail
	var wg sync.WaitGroup
	for range repeats {
		wg.Go(func() {
			assert.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Repeat", Email: "darline@example.com"}))
		})
	}
	// Only waits on this test's transaction: other packages' tests share the database.
	require.Eventually(t, func() bool {
		var waiting int
		require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))`, firstPID).Scan(&waiting))
		return waiting == repeats
	}, 10*time.Second, 10*time.Millisecond, "every repeat waits on the first")
	require.NoError(t, first.Commit(t.Context()))
	wg.Wait()

	for range accessRequestsPerEmail {
		_, err := pool.Exec(t.Context(), `UPDATE access_requests SET status = 'declined', reviewed_by = $1, reviewed_at = now() WHERE status = 'pending'`, admin)
		require.NoError(t, err)
		require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Darline Monestime", Email: "darline@example.com"}))
	}

	assert.Equal(t, accessRequestsPerEmail, countRows(t, pool, `SELECT count(*) FROM access_requests WHERE full_name = 'Darline Monestime'`),
		"the repeats stored nothing, so the whole budget is left")
}

func TestSubmitStopsAfterFiveAnHourPerEmail(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	admin := insertUser(t, pool, "admin@example.com")

	for range accessRequestsPerEmail + 1 {
		require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Darline", Email: "darline@example.com"}))
		// Declined each time, so the next one isn't just dropped as a repeat.
		_, err := pool.Exec(t.Context(), `UPDATE access_requests SET status = 'declined', reviewed_by = $1, reviewed_at = now() WHERE status = 'pending'`, admin)
		require.NoError(t, err)
	}

	assert.Equal(t, accessRequestsPerEmail, countRows(t, pool, `SELECT count(*) FROM access_requests`))
}

func TestListMatchesNamesAndShowsTheQueue(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	admin := insertUser(t, pool, "admin@example.com")
	frantz := insertNamedPerson(t, pool, "Frantz", "Monestime")
	stanley := insertNamedPerson(t, pool, "Stanley", "Jean-Louis")
	gone := insertNamedPerson(t, pool, "Gone", "Away")
	_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)
	var suggester uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id) VALUES ('stanley@example.com', 'member', $1) RETURNING id`, stanley).Scan(&suggester))
	insert := func(email, status, age string, names []string, suggestedBy *uuid.UUID) {
		t.Helper()
		if names == nil {
			names = []string{}
		}
		source := "self"
		if suggestedBy != nil {
			source = "member_suggestion"
		}
		_, err := pool.Exec(t.Context(), `INSERT INTO access_requests (full_name, email, related_names, source, suggested_by, status, reviewed_by, reviewed_at, created_at)
			VALUES ($1, $1, $2, $3, $4, $5, CASE WHEN $5 <> 'pending' THEN $6::uuid END, CASE WHEN $5 <> 'pending' THEN now() - $7::interval END, now() - $7::interval)`,
			email, names, source, suggestedBy, status, admin, age)
		require.NoError(t, err)
	}
	insert("declined@example.com", "declined", "1 day", nil, nil)
	insert("declined-today@example.com", "declined", "1 minute", []string{"Frantz Monestim"}, nil)
	insert("old@example.com", "declined", "91 days", nil, nil)
	insert("approved@example.com", "approved", "1 day", nil, nil)
	insert("second@example.com", "pending", "1 hour", nil, &suggester)
	insert("first@example.com", "pending", "2 hours", []string{"Frantz Monestim", "Gone Away", "Nobody Here"}, nil)

	requests, err := svc.List(t.Context())

	require.NoError(t, err)
	var emails []string
	for _, r := range requests {
		emails = append(emails, r.Email)
	}
	assert.Equal(t, []string{"first@example.com", "second@example.com", "declined-today@example.com", "declined@example.com"}, emails,
		"pending oldest first, then the latest declines")
	assert.Equal(t, []RelatedName{{Name: "Frantz Monestim", Match: &PersonName{PersonID: frantz, Name: "Frantz Monestime"}}}, requests[2].RelatedNames,
		"a name in two requests matches in both")
	assert.Equal(t, []RelatedName{
		{Name: "Frantz Monestim", Match: &PersonName{PersonID: frantz, Name: "Frantz Monestime"}},
		{Name: "Gone Away"},
		{Name: "Nobody Here"},
	}, requests[0].RelatedNames, "a near spelling matches; deleted people and strangers don't")
	assert.Equal(t, &PersonName{PersonID: stanley, Name: "Stanley Jean-Louis"}, requests[1].SuggestedBy)
}

func TestDeclineAndReopen(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	admin := insertUser(t, pool, "admin@example.com")
	require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Darline", Email: "darline@example.com"}))
	requests, err := svc.List(t.Context())
	require.NoError(t, err)
	id := requests[0].ID

	tests := []struct {
		name    string
		do      func() error
		wantErr error
	}{
		{name: "reopening a pending one", do: func() error { return svc.Reopen(t.Context(), id) }, wantErr: ErrAccessRequestState},
		{name: "declined", do: func() error { return svc.Decline(t.Context(), admin, id) }},
		{name: "declining twice", do: func() error { return svc.Decline(t.Context(), admin, id) }, wantErr: ErrAccessRequestState},
		{name: "an unknown one", do: func() error { return svc.Decline(t.Context(), admin, uuid.New()) }, wantErr: ErrAccessRequestNotFound},
		{name: "reopened", do: func() error { return svc.Reopen(t.Context(), id) }},
		{name: "declined again", do: func() error { return svc.Decline(t.Context(), admin, id) }},
		{name: "a new request meanwhile", do: func() error {
			return svc.Submit(t.Context(), AccessRequestInput{FullName: "Darline again", Email: "darline@example.com"})
		}},
		{name: "reopening while another is pending", do: func() error { return svc.Reopen(t.Context(), id) }, wantErr: ErrAccessRequestState},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.do()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAnInviteApprovesItsRequest(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		unknown    bool
		wantErr    error
		wantStatus string
	}{
		{name: "a pending request is approved", status: "pending", wantStatus: "approved"},
		{name: "a declined one can't be", status: "declined", wantErr: ErrAccessRequestState, wantStatus: "declined"},
		{name: "an unknown one", status: "pending", unknown: true, wantErr: ErrInvalidAccessRequest, wantStatus: "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			admin := insertUser(t, pool, "admin@example.com")
			person := insertPerson(t, pool)
			var request uuid.UUID
			require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO access_requests (full_name, email, source, status, reviewed_by, reviewed_at)
				VALUES ('Darline', 'darline@example.com', 'self', $1, CASE WHEN $1 <> 'pending' THEN $2::uuid END, CASE WHEN $1 <> 'pending' THEN now() END)
				RETURNING id`, tt.status, admin).Scan(&request))
			if tt.unknown {
				request = uuid.New()
			}
			mailer := mailmocks.NewMailer(t)
			if tt.wantErr == nil {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Return(nil).Once()
			}

			user, err := NewUsers(pool, mailer, testBaseURL).Invite(t.Context(), InviteInput{
				Email: "darline@example.com", Role: auth.RoleMember, PersonID: person, SentBy: admin, AccessRequestID: &request,
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, countRows(t, pool, `SELECT count(*) FROM users WHERE email = 'darline@example.com'`), "nothing saved")
			} else {
				require.NoError(t, err)
				var invited *uuid.UUID
				require.NoError(t, pool.QueryRow(t.Context(), `SELECT invited_user_id FROM access_requests WHERE id = $1`, request).Scan(&invited))
				assert.Equal(t, &user.ID, invited)
			}
			var status string
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT status FROM access_requests`).Scan(&status))
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestCleanupDeletesDeclinesAfterNinetyDays(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	admin := insertUser(t, pool, "admin@example.com")
	for _, r := range []struct{ email, status, age string }{
		{"old@example.com", "declined", "91 days"}, {"recent@example.com", "declined", "89 days"}, {"approved@example.com", "approved", "200 days"},
	} {
		_, err := pool.Exec(t.Context(), `INSERT INTO access_requests (full_name, email, source, status, reviewed_by, reviewed_at)
			VALUES ('x', $1, 'self', $2, $3, now() - $4::interval)`, r.email, r.status, admin, r.age)
		require.NoError(t, err)
	}

	n, err := gen.New(pool).DeleteOldDeclinedAccessRequests(t.Context(), 1000)

	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	assert.Zero(t, countRows(t, pool, `SELECT count(*) FROM access_requests WHERE email = 'old@example.com'`))
}

func insertNamedPerson(t *testing.T, pool *pgxpool.Pool, first, last string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES ($1, $2) RETURNING id`, first, last).Scan(&id))
	return id
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), sql).Scan(&n))
	return n
}

func TestSuggest(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	stanley := insertNamedPerson(t, pool, "Stanley", "Jean-Louis")
	var member uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id) VALUES ('stanley@example.com', 'member', $1) RETURNING id`, stanley).Scan(&member))
	kervens := AccessRequestInput{FullName: "Kervens Pluviose", Email: "kervens@example.com", Relationship: new("cousin")}

	require.ErrorIs(t, svc.Suggest(t.Context(), member, nil, kervens), ErrNotLinked, "a login linked to nobody can't suggest")
	gone := insertNamedPerson(t, pool, "Gone", "Away")
	_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)
	require.ErrorIs(t, svc.Suggest(t.Context(), member, &gone, kervens), ErrNotLinked, "nor one linked to someone deleted")
	require.ErrorIs(t, svc.Suggest(t.Context(), member, &stanley, AccessRequestInput{FullName: "Kervens", Email: "nope"}), ErrInvalidAccessRequest)
	require.NoError(t, svc.Suggest(t.Context(), member, &stanley, kervens))
	require.NoError(t, svc.Submit(t.Context(), AccessRequestInput{FullName: "Someone else", Email: "kervens@example.com"}), "a stranger's repeat is dropped")

	require.NoError(t, svc.Suggest(t.Context(), member, &stanley, AccessRequestInput{FullName: "Kervens again", Email: "kervens@example.com"}),
		"a member's repeat is dropped too")

	requests, err := svc.List(t.Context())
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, "Kervens Pluviose", requests[0].FullName)
	assert.Equal(t, "member_suggestion", requests[0].Source)
	assert.Equal(t, &PersonName{PersonID: stanley, Name: "Stanley Jean-Louis"}, requests[0].SuggestedBy)
}

func TestTwentySuggestionsADay(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	svc := NewAccessRequests(pool)
	stanley := insertNamedPerson(t, pool, "Stanley", "Jean-Louis")
	var member uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id) VALUES ('stanley@example.com', 'member', $1) RETURNING id`, stanley).Scan(&member))

	for i := range suggestionsPerDay + 1 {
		require.NoError(t, svc.Suggest(t.Context(), member, &stanley, AccessRequestInput{FullName: "Relative", Email: fmt.Sprintf("relative%d@example.com", i)}))
	}

	assert.Equal(t, suggestionsPerDay, countRows(t, pool, `SELECT count(*) FROM access_requests`), "the 21st is dropped")
}

func TestASuggestedInviteIsFromTheMember(t *testing.T) {
	tests := []struct {
		name              string
		suggesterDisabled bool
		fromAdmin         bool
		wantSubject       string
	}{
		{name: "from the member who suggested them", wantSubject: "Stanley invited you to the family hub"},
		{name: "from the admin when the member can't send one", suggesterDisabled: true, wantSubject: "Maya invited you to the family hub"},
		{name: "from whoever the admin picks", fromAdmin: true, wantSubject: "Maya invited you to the family hub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			maya, stanley := insertNamedPerson(t, pool, "Maya", "Pluviose"), insertNamedPerson(t, pool, "Stanley", "Jean-Louis")
			kervens := insertNamedPerson(t, pool, "Kervens", "Pluviose")
			var admin, member uuid.UUID
			require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id, password_hash, password_changed_at)
				VALUES ('admin@example.com', 'admin', $1, 'hash', now()) RETURNING id`, maya).Scan(&admin))
			require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id, password_hash, password_changed_at, disabled_at)
				VALUES ('stanley@example.com', 'member', $1, 'hash', now(), CASE WHEN $2 THEN now() END) RETURNING id`,
				stanley, tt.suggesterDisabled).Scan(&member))
			require.NoError(t, NewAccessRequests(pool).Suggest(t.Context(), member, &stanley,
				AccessRequestInput{FullName: "Kervens Pluviose", Email: "kervens@example.com"}))
			var request uuid.UUID
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT id FROM access_requests`).Scan(&request))
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, m mail.Message) { sent = m }).Return(nil).Once()
			in := InviteInput{Email: "kervens@example.com", Role: auth.RoleMember, PersonID: kervens, SentBy: admin, AccessRequestID: &request}
			if tt.fromAdmin {
				in.FromPersonID = &maya
			}

			_, err := NewUsers(pool, mailer, testBaseURL).Invite(t.Context(), in)

			require.NoError(t, err)
			assert.Equal(t, tt.wantSubject, sent.Subject)
		})
	}
}
