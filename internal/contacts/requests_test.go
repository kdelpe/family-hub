package contacts

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
)

// member is someone in the tree with a login, seen as they'd ask.
func member(t *testing.T, pool *pgxpool.Pool, first, last string) family.Viewer {
	t.Helper()
	var person uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES ($1, $2) RETURNING id`, first, last).Scan(&person))
	return family.Viewer{UserID: linkLogin(t, pool, person, person.String()+"@example.com"), PersonID: &person}
}

func TestAskShareRevoke(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	mobile, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	require.NoError(t, err)
	home, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0001", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "address", Value: "Jacmel", Visibility: new("on_request")})
	require.NoError(t, err)
	seen := func() Listing {
		t.Helper()
		l, err := svc.List(t.Context(), darline, *maya.PersonID)
		require.NoError(t, err)
		return l
	}
	decide := func(d Decision) {
		t.Helper()
		requests, err := svc.Requests(t.Context(), maya)
		require.NoError(t, err)
		require.Len(t, requests, 1)
		require.NoError(t, svc.Decide(t.Context(), maya, requests[0].ID, d))
	}

	status, err := svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	assert.Equal(t, "requested", status)
	assert.Equal(t, Listing{Contacts: []Contact{}, Requestable: []Requestable{{"address", "none"}, {"phone", "requested"}}}, seen())
	requests, err := svc.Requests(t.Context(), maya)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, Requester{PersonID: *darline.PersonID, Name: "Darline Monestime"}, requests[0].From)
	assert.Equal(t, "pending", requests[0].Status)

	decide(Share)
	assert.Equal(t, Listing{Contacts: []Contact{mobile, home}, Requestable: []Requestable{{"address", "none"}}}, seen(), "every phone, and only phones")
	status, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	assert.Equal(t, "shared", status, "asking again changes nothing")

	decide(Revoke)
	assert.Equal(t, Listing{Contacts: []Contact{}, Requestable: []Requestable{{"address", "none"}, {"phone", "requested"}}}, seen(), "a revoke reads as requested")
	requests, err = svc.Requests(t.Context(), maya)
	require.NoError(t, err)
	assert.Empty(t, requests, "a revoked request leaves the list")
	status, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	assert.Equal(t, "requested", status, "and asking again doesn't reopen it")
}

func TestDecideRules(t *testing.T) {
	svc, pool := newService(t)
	maya, darline, rosa := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime"), member(t, pool, "Rosa", "Pierre")
	_, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "address", Value: "Jacmel", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "address")
	require.NoError(t, err)
	requests, err := svc.Requests(t.Context(), maya)
	require.NoError(t, err)
	id := requests[0].ID

	tests := []struct {
		name    string
		viewer  family.Viewer
		id      uuid.UUID
		d       Decision
		wantErr error
	}{
		{name: "revoking what wasn't shared", viewer: maya, id: id, d: Revoke, wantErr: ErrWrongState},
		{name: "someone else's request", viewer: rosa, id: id, d: Share, wantErr: ErrRequestNotFound},
		{name: "an unknown request", viewer: maya, id: uuid.New(), d: Share, wantErr: ErrRequestNotFound},
		{name: "a login linked to nobody", viewer: family.Viewer{UserID: darline.UserID}, id: id, d: Share, wantErr: ErrNotLinked},
		{name: "declined", viewer: maya, id: id, d: Decline},
		{name: "declining twice", viewer: maya, id: id, d: Decline, wantErr: ErrWrongState},
		{name: "sharing once declined", viewer: maya, id: id, d: Share, wantErr: ErrWrongState},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Decide(t.Context(), tt.viewer, tt.id, tt.d)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}

	listing, err := svc.List(t.Context(), darline, *maya.PersonID)
	require.NoError(t, err)
	assert.Equal(t, []Requestable{{"address", "requested"}}, listing.Requestable, "a decline stays silent")
}

func TestAskRules(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	_, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "address", Value: "Jacmel"})
	require.NoError(t, err)
	gone := insertPerson(t, pool, "Gone")
	_, err = pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)

	tests := []struct {
		name    string
		viewer  family.Viewer
		person  uuid.UUID
		kind    string
		wantErr error
	}{
		{name: "an email", viewer: darline, person: *maya.PersonID, kind: "email", wantErr: ErrInvalidRequest},
		{name: "a hidden kind", viewer: darline, person: *maya.PersonID, kind: "address", wantErr: ErrInvalidRequest},
		{name: "a kind they don't have", viewer: darline, person: *maya.PersonID, kind: "whatsapp", wantErr: ErrInvalidRequest},
		{name: "their own", viewer: maya, person: *maya.PersonID, kind: "phone", wantErr: ErrInvalidRequest},
		{name: "an admin, who sees everything", viewer: family.Viewer{UserID: darline.UserID, IsAdmin: true}, person: *maya.PersonID, kind: "phone", wantErr: ErrInvalidRequest},
		{name: "someone deleted", viewer: darline, person: gone, kind: "phone", wantErr: family.ErrPersonNotFound},
		{name: "a login linked to nobody", viewer: family.Viewer{UserID: darline.UserID}, person: *maya.PersonID, kind: "phone", wantErr: ErrNotLinked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Ask(t.Context(), tt.viewer, tt.person, tt.kind)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
	var n int
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM contact_requests`).Scan(&n))
	assert.Zero(t, n, "nothing stored")
}

func TestTwentyNewRequestsADay(t *testing.T) {
	svc, pool := newService(t)
	darline := member(t, pool, "Darline", "Monestime")
	var first uuid.UUID
	for i := range requestsPerDay + 1 {
		relative := member(t, pool, "Relative", string(rune('A'+i)))
		_, err := svc.Create(t.Context(), relative, *relative.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
		require.NoError(t, err)
		if i == 0 {
			first = *relative.PersonID
		}
		_, err = svc.Ask(t.Context(), darline, *relative.PersonID, "phone")
		if i < requestsPerDay {
			require.NoError(t, err)
			continue
		}
		var limited *auth.RateLimitedError
		require.ErrorAs(t, err, &limited)
		assert.InDelta(t, (24 * time.Hour).Seconds(), limited.RetryAfter.Seconds(), 60, "until the oldest is a day old")
	}

	status, err := svc.Ask(t.Context(), darline, first, "phone")

	require.NoError(t, err, "a repeat is free")
	assert.Equal(t, "requested", status)
}

func TestAShareEndsWhenEitherSideCantSeeIt(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, pool *pgxpool.Pool, maya, darline family.Viewer)
	}{
		{name: "the requester is unlinked", change: func(t *testing.T, pool *pgxpool.Pool, _, darline family.Viewer) {
			_, err := pool.Exec(t.Context(), `UPDATE users SET person_id = NULL WHERE id = $1`, darline.UserID)
			require.NoError(t, err)
		}},
		{name: "the requester's person is deleted", change: func(t *testing.T, pool *pgxpool.Pool, _, darline family.Viewer) {
			_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, *darline.PersonID)
			require.NoError(t, err)
		}},
		{name: "the owner's login is disabled", change: func(t *testing.T, pool *pgxpool.Pool, maya, _ family.Viewer) {
			_, err := pool.Exec(t.Context(), `UPDATE users SET disabled_at = now() WHERE id = $1`, maya.UserID)
			require.NoError(t, err)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, pool := newService(t)
			maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
			_, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
			require.NoError(t, err)
			_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
			require.NoError(t, err)
			requests, err := svc.Requests(t.Context(), maya)
			require.NoError(t, err)
			require.NoError(t, svc.Decide(t.Context(), maya, requests[0].ID, Share))

			tt.change(t, pool, maya, darline)

			listing, err := svc.List(t.Context(), family.Viewer{UserID: darline.UserID}, *maya.PersonID)
			require.NoError(t, err)
			assert.Empty(t, listing.Contacts, "the phone is no longer shown")
		})
	}
}

func TestAskingAgainForWhatsNoLongerOffered(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	phone, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	_, err = svc.Update(t.Context(), maya, *maya.PersonID, phone.ID, ContactPatch{Visibility: family.Opt[string]{Set: true, Value: "hidden"}})
	require.NoError(t, err)

	_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")

	assert.ErrorIs(t, err, ErrInvalidRequest)
}

func TestRequestsFromSomeoneWhoCantSignInLeaveTheList(t *testing.T) {
	svc, pool := newService(t)
	maya, darline := member(t, pool, "Maya", "Pluviose"), member(t, pool, "Darline", "Monestime")
	_, err := svc.Create(t.Context(), maya, *maya.PersonID, ContactInput{Kind: "phone", Value: "+1 555 010 0000", Visibility: new("on_request")})
	require.NoError(t, err)
	_, err = svc.Ask(t.Context(), darline, *maya.PersonID, "phone")
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), `UPDATE users SET disabled_at = now() WHERE id = $1`, darline.UserID)
	require.NoError(t, err)

	requests, err := svc.Requests(t.Context(), maya)

	require.NoError(t, err)
	assert.Empty(t, requests)
}
