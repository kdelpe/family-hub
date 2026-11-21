package contacts

import (
	"context"
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

var admin = family.Viewer{IsAdmin: true}

func TestWhoSeesWhat(t *testing.T) {
	svc, pool := newService(t)
	maya, rosa := insertPerson(t, pool, "Maya"), insertPerson(t, pool, "Rosa")
	linkLogin(t, pool, maya, "maya@example.com")
	add := func(person uuid.UUID, in ContactInput) Contact {
		t.Helper()
		c, err := svc.Create(t.Context(), admin, person, in)
		require.NoError(t, err)
		return c
	}
	email := add(maya, ContactInput{Kind: "email", Value: "maya@example.com", Visibility: new("family")})
	hidden := add(maya, ContactInput{Kind: "email", Value: "work@example.com"})
	phone := add(maya, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	second := add(maya, ContactInput{Kind: "phone", Value: "+1 555 010 0001", Visibility: new("on_request")})
	whatsapp := add(maya, ContactInput{Kind: "social", Platform: new("whatsapp"), Value: "+1 555 010 0002", Visibility: new("on_request")})
	rosaEmail := add(rosa, ContactInput{Kind: "email", Value: "rosa@example.com", Visibility: new("family")})

	tests := []struct {
		name   string
		viewer family.Viewer
		person uuid.UUID
		want   Listing
	}{
		{name: "the owner sees every row", viewer: family.Viewer{PersonID: &maya}, person: maya,
			want: Listing{Contacts: []Contact{email, hidden, phone, second, whatsapp}, Full: true}},
		{name: "an admin sees every row", viewer: admin, person: maya,
			want: Listing{Contacts: []Contact{email, hidden, phone, second, whatsapp}, Full: true}},
		{name: "family sees family rows and what they could ask for, once per kind", viewer: family.Viewer{PersonID: &rosa}, person: maya,
			want: Listing{Contacts: []Contact{email}, Requestable: []Requestable{{"phone", "none"}, {"whatsapp", "none"}}}},
		{name: "an unlinked member too", viewer: family.Viewer{}, person: maya,
			want: Listing{Contacts: []Contact{email}, Requestable: []Requestable{{"phone", "none"}, {"whatsapp", "none"}}}},
		{name: "someone else's rows", viewer: family.Viewer{PersonID: &maya}, person: rosa,
			want: Listing{Contacts: []Contact{rosaEmail}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.List(t.Context(), tt.viewer, tt.person)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAPersonWhoCantAnswerOffersNothingOnRequest(t *testing.T) {
	tests := []struct {
		name     string
		takeAway string
	}{
		{name: "an unlinked login", takeAway: `UPDATE users SET person_id = NULL WHERE id = $1`},
		{name: "a disabled login", takeAway: `UPDATE users SET disabled_at = now() WHERE id = $1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool := newService(t)
			maya := insertPerson(t, pool, "Maya")
			login := linkLogin(t, pool, maya, "maya@example.com")
			address, err := svc.Create(t.Context(), admin, maya, ContactInput{Kind: "address", Value: "Jacmel", Visibility: new("on_request")})
			require.NoError(t, err)
			_, err = pool.Exec(t.Context(), tt.takeAway, login)
			require.NoError(t, err)

			got, err := svc.List(t.Context(), family.Viewer{}, maya)

			require.NoError(t, err)
			assert.Equal(t, Listing{Contacts: []Contact{}}, got, "their rows stay with admins")
			_, err = svc.Create(t.Context(), admin, maya, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
			require.ErrorIs(t, err, ErrInvalidContact, "and none can be added")
			edited, err := svc.Update(t.Context(), admin, maya, address.ID, ContactPatch{Value: family.Opt[string]{Set: true, Value: "Jacmel, Haiti"}})
			require.NoError(t, err, "an existing one can still be fixed")
			assert.Equal(t, "Jacmel, Haiti", edited.Value)
		})
	}
}

func TestOnRequestWaitsForALoginBeingTakenAway(t *testing.T) {
	takeAways := map[string]string{
		"disabled": `UPDATE users SET disabled_at = now() WHERE id = $1`,
		"unlinked": `UPDATE users SET person_id = NULL WHERE id = $1`,
	}
	writes := map[string]func(ctx context.Context, svc *Service, maya uuid.UUID) error{
		"a new row": func(ctx context.Context, svc *Service, maya uuid.UUID) error {
			_, err := svc.Create(ctx, admin, maya, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
			return err
		},
		"a hidden row made requestable": func(ctx context.Context, svc *Service, maya uuid.UUID) error {
			rows, err := svc.List(ctx, admin, maya)
			if err != nil {
				return err
			}
			_, err = svc.Update(ctx, admin, maya, rows.Contacts[0].ID, ContactPatch{Visibility: family.Opt[string]{Set: true, Value: "on_request"}})
			return err
		},
	}

	for takeName, takeAway := range takeAways {
		for writeName, write := range writes {
			t.Run(writeName+", login "+takeName, func(t *testing.T) {
				svc, pool := newService(t)
				maya := insertPerson(t, pool, "Maya")
				login := linkLogin(t, pool, maya, "maya@example.com")
				_, err := svc.Create(t.Context(), admin, maya, ContactInput{Kind: "address", Value: "Jacmel"})
				require.NoError(t, err)
				change, err := pool.Begin(t.Context())
				require.NoError(t, err)
				t.Cleanup(func() { _ = change.Rollback(context.Background()) })
				_, err = change.Exec(t.Context(), takeAway, login)
				require.NoError(t, err)

				done := make(chan error, 1)
				go func() { done <- write(t.Context(), svc, maya) }()
				require.Eventually(t, func() bool {
					var waiting int
					require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM pg_locks WHERE NOT granted`).Scan(&waiting))
					return waiting > 0
				}, 5*time.Second, 10*time.Millisecond, "the write waits on the login's row")
				require.NoError(t, change.Commit(t.Context()))

				require.ErrorIs(t, <-done, ErrInvalidContact)
				var requestable int
				require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM contact_methods WHERE visibility = 'on_request'`).Scan(&requestable))
				assert.Zero(t, requestable, "nothing left on request")
			})
		}
	}
}

func TestWrites(t *testing.T) {
	svc, pool := newService(t)
	maya, rosa, gone := insertPerson(t, pool, "Maya"), insertPerson(t, pool, "Rosa"), insertPerson(t, pool, "Gone")
	_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)
	me := family.Viewer{PersonID: &maya}
	phone := ContactInput{Kind: "phone", Value: "+1 555 010 0000"}

	created, err := svc.Create(t.Context(), me, maya, phone)
	require.NoError(t, err)
	assert.Equal(t, "hidden", created.Visibility, "hidden unless chosen")

	_, err = svc.Update(t.Context(), me, maya, created.ID, ContactPatch{
		Label: family.Opt[*string]{Set: true, Value: new("Mobile")}, Visibility: family.Opt[string]{Set: true, Value: "on_request"},
	})
	require.ErrorIs(t, err, ErrInvalidContact, "Maya has no login yet, so nobody could answer")
	linkLogin(t, pool, maya, "maya@example.com")
	updated, err := svc.Update(t.Context(), me, maya, created.ID, ContactPatch{
		Label: family.Opt[*string]{Set: true, Value: new("Mobile")}, Visibility: family.Opt[string]{Set: true, Value: "on_request"},
	})
	require.NoError(t, err)
	assert.Equal(t, Contact{ID: created.ID, Kind: "phone", Label: new("Mobile"), Value: "+1 555 010 0000", Visibility: "on_request"}, updated)

	tests := []struct {
		name    string
		write   func() error
		wantErr error
	}{
		{name: "a member adding to someone else", wantErr: ErrNotYours,
			write: func() error { _, err := svc.Create(t.Context(), me, rosa, phone); return err }},
		{name: "a member editing someone else's", wantErr: ErrNotYours,
			write: func() error { _, err := svc.Update(t.Context(), me, rosa, created.ID, ContactPatch{}); return err }},
		{name: "a member deleting someone else's", wantErr: ErrNotYours,
			write: func() error { return svc.Delete(t.Context(), me, rosa, created.ID) }},
		{name: "a row under the wrong person", wantErr: ErrContactNotFound,
			write: func() error { _, err := svc.Update(t.Context(), admin, rosa, created.ID, ContactPatch{}); return err }},
		{name: "deleting under the wrong person", wantErr: ErrContactNotFound,
			write: func() error { return svc.Delete(t.Context(), admin, rosa, created.ID) }},
		{name: "a deleted person", wantErr: family.ErrPersonNotFound,
			write: func() error { _, err := svc.Create(t.Context(), admin, gone, phone); return err }},
		{name: "an invalid edit", wantErr: ErrInvalidContact,
			write: func() error {
				_, err := svc.Update(t.Context(), me, maya, created.ID, ContactPatch{Visibility: family.Opt[string]{Set: true, Value: "family"}})
				return err
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.write(), tt.wantErr)
		})
	}

	got, err := svc.List(t.Context(), me, maya)
	require.NoError(t, err)
	assert.Equal(t, []Contact{updated}, got.Contacts, "the refused writes changed nothing")
	require.NoError(t, svc.Delete(t.Context(), me, maya, created.ID))
	assert.ErrorIs(t, svc.Delete(t.Context(), me, maya, created.ID), ErrContactNotFound, "deleting twice")
}

func TestAPersonHasAtMostFiftyContacts(t *testing.T) {
	svc, pool := newService(t)
	maya := insertPerson(t, pool, "Maya")
	for range maxPerPerson {
		_, err := svc.Create(t.Context(), admin, maya, ContactInput{Kind: "other", Value: "a"})
		require.NoError(t, err)
	}

	_, err := svc.Create(t.Context(), admin, maya, ContactInput{Kind: "other", Value: "a"})

	assert.ErrorIs(t, err, ErrInvalidContact)
}

func TestReadsNeedSomeoneInTheTree(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.List(t.Context(), admin, uuid.New())

	assert.ErrorIs(t, err, family.ErrPersonNotFound)
}

func TestMine(t *testing.T) {
	maya := uuid.New()

	got, err := Mine(family.Viewer{PersonID: &maya})
	require.NoError(t, err)
	assert.Equal(t, maya, got)

	_, err = Mine(family.Viewer{IsAdmin: true})
	assert.ErrorIs(t, err, ErrNotLinked, "even an admin needs a linked person for /me")
}

func newService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	return NewService(pool), pool
}

func insertPerson(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name) VALUES ($1) RETURNING id`, name).Scan(&id))
	return id
}

func linkLogin(t *testing.T, pool *pgxpool.Pool, person uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id) VALUES ($1, 'member', $2) RETURNING id`, email, person).Scan(&id))
	return id
}

func TestDirectory(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	rosa, gone, deleted := insertPerson(t, pool, "Rosa"), insertPerson(t, pool, "Gone"), insertPerson(t, pool, "Deleted")
	quiet, unanswerable := insertPerson(t, pool, "Quiet"), insertPerson(t, pool, "Unanswerable")
	add := func(person uuid.UUID, in ContactInput) Contact {
		t.Helper()
		c, err := svc.Create(t.Context(), admin, person, in)
		require.NoError(t, err)
		return c
	}
	mayaEmail := add(*maya.PersonID, ContactInput{Kind: "email", Value: "maya@example.com", Visibility: new("family")})
	mayaPhone := add(*maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	rosaEmail := add(rosa, ContactInput{Kind: "email", Value: "rosa@example.com", Visibility: new("family")})
	add(gone, ContactInput{Kind: "email", Value: "gone@example.com", Visibility: new("family")})
	add(deleted, ContactInput{Kind: "email", Value: "deleted@example.com", Visibility: new("family")})
	add(quiet, ContactInput{Kind: "email", Value: "quiet@example.com"})
	unanswerableLogin := linkLogin(t, pool, unanswerable, "unanswerable@example.com")
	add(unanswerable, ContactInput{Kind: "phone", Value: "+1 555 010 0001", Visibility: new("on_request")})
	_, err := pool.Exec(t.Context(), `UPDATE users SET disabled_at = now() WHERE id = $1`, unanswerableLogin)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE persons SET is_deceased = true WHERE id = $1`, gone)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, deleted)
	require.NoError(t, err)
	pluviose := new("Pluviose")
	nothing := Listing{Contacts: []Contact{}}
	darlineEntry := DirectoryEntry{ID: *darline.PersonID, FirstName: "Darline", LastName: new("Monestime"), HasAccount: true, Listing: nothing}
	mayaAsFamily := DirectoryEntry{ID: *maya.PersonID, FirstName: "Maya", LastName: pluviose, HasAccount: true,
		Listing: Listing{Contacts: []Contact{mayaEmail}, Requestable: []Requestable{{"phone", "none"}}}}
	rosaEntry := DirectoryEntry{ID: rosa, FirstName: "Rosa", Listing: Listing{Contacts: []Contact{rosaEmail}}}
	quietEntry := DirectoryEntry{ID: quiet, FirstName: "Quiet", Listing: nothing}
	// A disabled login doesn't count as an account.
	unanswerableEntry := DirectoryEntry{ID: unanswerable, FirstName: "Unanswerable", Listing: nothing}

	tests := []struct {
		name   string
		viewer family.Viewer
		query  string
		want   []DirectoryEntry
	}{
		{name: "every living person, A–Z, as this viewer sees them", viewer: darline,
			want: []DirectoryEntry{{ID: *darline.PersonID, FirstName: "Darline", LastName: new("Monestime"), HasAccount: true, Listing: Listing{Contacts: []Contact{}, Full: true}},
				mayaAsFamily, quietEntry, rosaEntry, unanswerableEntry}},
		{name: "their own entry in full", viewer: maya,
			want: []DirectoryEntry{darlineEntry, {ID: *maya.PersonID, FirstName: "Maya", LastName: pluviose, HasAccount: true,
				Listing: Listing{Contacts: []Contact{mayaEmail, mayaPhone}, Full: true}}, quietEntry, rosaEntry, unanswerableEntry}},
		{name: "searched by name", viewer: darline, query: "  pluvi ", want: []DirectoryEntry{mayaAsFamily}},
		{name: "a typo still finds them", viewer: darline, query: "Mya Pluviose", want: []DirectoryEntry{mayaAsFamily}},
		{name: "never by contact value", viewer: darline, query: "rosa@example.com", want: []DirectoryEntry{}},
		{name: "wildcards match themselves", viewer: darline, query: "%", want: []DirectoryEntry{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.Directory(t.Context(), tt.viewer, tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDirectoryShowsWhatWasShared(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	phone, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	requests, err := svc.Requests(t.Context(), maya)
	require.NoError(t, err)
	require.NoError(t, svc.Decide(t.Context(), maya, requests[0].ID, Share))

	got, err := svc.Directory(t.Context(), darline, "Maya")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, Listing{Contacts: []Contact{phone}}, got[0].Listing)
}

func TestDirectoryPutsTheBestMatchFirst(t *testing.T) {
	svc, pool := newService(t)
	for _, name := range []string{"Rosa", "Rosalind"} {
		_, err := svc.Create(t.Context(), admin, insertPerson(t, pool, name), ContactInput{Kind: "email", Value: name + "@example.com", Visibility: new("family")})
		require.NoError(t, err)
	}

	got, err := svc.Directory(t.Context(), admin, "Rosalind")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, []string{"Rosalind", "Rosa"}, []string{got[0].FirstName, got[1].FirstName})
}

func TestDirectorySearchRules(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Directory(t.Context(), admin, strings.Repeat("a", 101))

	require.ErrorIs(t, err, family.ErrInvalidFilter)
}

func TestEditingKeepsALinkSavedUnderTheOlderRule(t *testing.T) {
	svc, pool := newService(t)
	maya := insertPerson(t, pool, "Maya")
	me := family.Viewer{PersonID: &maya}
	var id uuid.UUID
	// A post link passed the looser check that came before profile links (§5.2).
	err := pool.QueryRow(t.Context(), `INSERT INTO contact_methods (person_id, kind, platform, value, visibility)
		VALUES ($1, 'social', 'instagram', 'https://www.instagram.com/p/XYZ/', 'hidden') RETURNING id`, maya).Scan(&id)
	require.NoError(t, err)

	tests := []struct {
		name    string
		patch   ContactPatch
		wantErr error
	}{
		{name: "a label-only edit", patch: ContactPatch{Label: family.Opt[*string]{Set: true, Value: new("Old account")}}},
		{name: "the same value sent back with a space", patch: ContactPatch{Value: family.Opt[string]{Set: true, Value: "https://www.instagram.com/p/XYZ/ "}}},
		{name: "showing it to the family is held to the rule", patch: ContactPatch{Visibility: family.Opt[string]{Set: true, Value: "family"}},
			wantErr: ErrInvalidContact},
		{name: "a new value is held to the rule", patch: ContactPatch{Value: family.Opt[string]{Set: true, Value: "https://www.instagram.com/p/ABC/"}},
			wantErr: ErrInvalidContact},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(t.Context(), me, maya, id, tt.patch)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
