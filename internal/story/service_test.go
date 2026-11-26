package story

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/testdb"
)

func TestCreateRefusesAnInvalidStory(t *testing.T) {
	tests := []struct {
		name       string
		in         Input
		wantErrMsg string
	}{
		{name: "no title", in: Input{Title: "  "}, wantErrMsg: "a title is required"},
		{name: "title too long", in: Input{Title: strings.Repeat("é", maxTitleLength+1)}, wantErrMsg: "the title is longer than 200 characters"},
		{name: "a line break in the title", in: Input{Title: "Brooklyn\n1972"}, wantErrMsg: "the title contains a character that isn't allowed"},
		{name: "a tab in the place", in: Input{Title: "Brooklyn", Place: new("Jac\tmel")}, wantErrMsg: "the place contains a character that isn't allowed"},
		{name: "a NUL in the summary", in: Input{Title: "Brooklyn", Summary: new("A\x00day")}, wantErrMsg: "the summary contains a character that isn't allowed"},
		{name: "unknown language", in: Input{Title: "Brooklyn", Language: new("es")}, wantErrMsg: "the language must be ht, fr, en or mixed"},
		{name: "a date without its precision", in: Input{Title: "Brooklyn", RecordedOn: new("2009")}, wantErrMsg: "give the date and its precision together"},
		{name: "a date that doesn't match its precision", in: Input{Title: "Brooklyn", RecordedOn: new("2009-10"), RecordedOnPrecision: new("year")},
			wantErrMsg: "the date must look like 2006 for precision year"},
		{name: "a line break in a topic", in: Input{Title: "Brooklyn", Topics: []string{"Work\nlife"}}, wantErrMsg: "the topic contains a character that isn't allowed"},
		{name: "too many topics", in: Input{Title: "Brooklyn", Topics: numbered("topic", maxTopics+1)}, wantErrMsg: "give at most 20 topics"},
		{name: "too many other people", in: Input{Title: "Brooklyn", OtherPeople: numbered("Père", maxPeople+1)}, wantErrMsg: "give at most 100 names"},
		{name: "a chapter without a title", in: Input{Title: "Brooklyn", Chapters: []Chapter{{At: 0, Title: " "}}}, wantErrMsg: "every chapter needs a title"},
		{name: "two chapters at once", in: Input{Title: "Brooklyn", Chapters: []Chapter{{At: 60, Title: "A"}, {At: 60, Title: "B"}}},
			wantErrMsg: "two chapters start at the same time"},
		{name: "a chapter before the start", in: Input{Title: "Brooklyn", Chapters: []Chapter{{At: -1, Title: "A"}}},
			wantErrMsg: "a chapter's time must be between 0 and 86400 seconds"},
		{name: "a transcript line past a day", in: Input{Title: "Brooklyn", Transcript: []Line{{At: maxSeconds + 1, Text: "Late"}}},
			wantErrMsg: "a transcript line's time must be between 0 and 86400 seconds"},
		{name: "a transcript line too long", in: Input{Title: "Brooklyn", Transcript: []Line{{Text: strings.Repeat("a", maxLineLength+1)}}},
			wantErrMsg: "the transcript is longer than 5000 characters"},
		{name: "a transcript too long in all", in: Input{Title: "Brooklyn", Transcript: longTranscript()},
			wantErrMsg: "the transcript is longer than 300000 characters in all"},
		{name: "far too many topics", in: Input{Title: "Brooklyn", Topics: make([]string, 1000)}, wantErrMsg: "give at most 20 topics"},
		{name: "too many people", in: Input{Title: "Brooklyn", PersonIDs: make([]uuid.UUID, maxPeople+1)}, wantErrMsg: "tag at most 100 people"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(nil).Create(t.Context(), family.Viewer{UserID: uuid.New()}, tt.in)

			require.ErrorIs(t, err, ErrInvalidStory)
			assert.Equal(t, "invalid story: "+tt.wantErrMsg, err.Error())
		})
	}
}

func TestCreateAndGet(t *testing.T) {
	f := newFixture(t)
	jr, joseph, fritz := f.person(t, "Jean-Robert", "Pluviose"), f.person(t, "Joseph", "Pluviose"), f.person(t, "Fritz", "Pluviose")

	tests := []struct {
		name string
		in   Input
		want Story
	}{
		{
			name: "everything, trimmed and in order",
			in: Input{
				Title: " How we came to Brooklyn ", TellerID: &jr, ConsentConfirmed: true, RecordedOn: new("2026-10"),
				RecordedOnPrecision: new("month"), Place: new(" Brooklyn "), Summary: new("Leaving Jacmel.\nThe first winter."),
				Language:   new("mixed"),
				Chapters:   []Chapter{{At: 1050, Title: " Landing at JFK "}, {At: 0, Title: "Growing up in Jacmel"}},
				Transcript: []Line{{At: 1050, Text: "The cold hit me."}, {At: 0, Text: " My father had a shop. "}, {At: 5, Text: " "}},
				Topics:     []string{" Moving ", "Work", "Moving", ""}, PersonIDs: []uuid.UUID{joseph, fritz, joseph},
				OtherPeople: []string{" Père Antoine ", "Père Antoine"},
			},
			want: Story{
				Title: "How we came to Brooklyn", Status: "draft", ConsentConfirmed: true,
				Teller:     &Person{ID: jr, FirstName: "Jean-Robert", LastName: new("Pluviose")},
				RecordedOn: new("2026-10"), RecordedOnPrecision: new("month"), Place: new("Brooklyn"),
				Summary: new("Leaving Jacmel.\nThe first winter."), Language: new("mixed"),
				Chapters:    []Chapter{{At: 0, Title: "Growing up in Jacmel"}, {At: 1050, Title: "Landing at JFK"}},
				Transcript:  []Line{{At: 0, Text: "My father had a shop."}, {At: 1050, Text: "The cold hit me."}},
				Topics:      []string{"Moving", "Work"},
				People:      []Person{{ID: fritz, FirstName: "Fritz", LastName: new("Pluviose")}, {ID: joseph, FirstName: "Joseph", LastName: new("Pluviose")}},
				OtherPeople: []string{"Père Antoine"},
			},
		},
		{
			name: "only a title",
			in:   Input{Title: "Christmas Eve, 1988", Place: new(" ")},
			want: Story{Title: "Christmas Eve, 1988", Status: "draft", Chapters: []Chapter{}, Transcript: []Line{}, Topics: []string{},
				People: []Person{}, OtherPeople: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := f.svc.Create(t.Context(), f.member, tt.in)
			require.NoError(t, err)

			tt.want.ID, tt.want.CreatedBy, tt.want.CreatedAt = created.ID, &f.member.UserID, created.CreatedAt
			assert.Equal(t, tt.want, created)
			got, err := f.svc.Get(t.Context(), f.member, created.ID)
			require.NoError(t, err)
			assert.Equal(t, created, got)
		})
	}
}

func TestCreateRefusesPeopleNotInTheTree(t *testing.T) {
	f := newFixture(t)
	gone := f.person(t, "Gone", "Pluviose")
	_, err := f.pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, gone)
	require.NoError(t, err)

	tests := []struct {
		name string
		in   Input
		want string
	}{
		{name: "unknown teller", in: Input{Title: "A", TellerID: new(uuid.New())}, want: "invalid story: the storyteller isn't in the tree"},
		{name: "deleted teller", in: Input{Title: "A", TellerID: &gone}, want: "invalid story: the storyteller isn't in the tree"},
		{name: "unknown person", in: Input{Title: "A", PersonIDs: []uuid.UUID{uuid.New()}}, want: "invalid story: someone in the story isn't in the tree"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.svc.Create(t.Context(), f.member, tt.in)

			require.ErrorIs(t, err, ErrInvalidStory)
			assert.EqualError(t, err, tt.want)
		})
	}
}

func TestDraftsAreSeenOnlyByTheirCreatorAndAdmins(t *testing.T) {
	f := newFixture(t)
	draft, err := f.svc.Create(t.Context(), f.member, Input{Title: "Grandma's kitchen"})
	require.NoError(t, err)
	_, err = f.svc.Create(t.Context(), f.admin, Input{Title: "The band bus"})
	require.NoError(t, err)

	tests := []struct {
		name       string
		viewer     family.Viewer
		wantSee    bool
		wantDrafts []string
	}{
		{name: "the creator", viewer: f.member, wantSee: true, wantDrafts: []string{"Grandma's kitchen"}},
		{name: "an admin", viewer: f.admin, wantSee: true, wantDrafts: []string{"The band bus", "Grandma's kitchen"}},
		{name: "another member", viewer: f.other, wantSee: false, wantDrafts: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.svc.Get(t.Context(), tt.viewer, draft.ID)
			if tt.wantSee {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrStoryNotFound)
				_, err = f.svc.Update(t.Context(), tt.viewer, draft.ID, Patch{Title: family.Opt[string]{Set: true, Value: "Mine"}})
				require.ErrorIs(t, err, ErrStoryNotFound)
				require.ErrorIs(t, f.svc.Delete(t.Context(), tt.viewer, draft.ID), ErrStoryNotFound)
			}
			drafts, err := f.svc.List(t.Context(), tt.viewer, Filter{Drafts: true})
			require.NoError(t, err)
			assert.Equal(t, tt.wantDrafts, titles(drafts))
			published, err := f.svc.List(t.Context(), tt.viewer, Filter{})
			require.NoError(t, err)
			assert.Empty(t, published)
		})
	}
}

func TestPublishingNeedsConsent(t *testing.T) {
	f := newFixture(t)
	published := family.Opt[string]{Set: true, Value: "published"}

	tests := []struct {
		name       string
		in         Input
		patch      Patch
		wantErrMsg string
		wantStatus string
	}{
		{name: "no consent", in: Input{Title: "A"}, patch: Patch{Status: published},
			wantErrMsg: "invalid story: confirm the storyteller agreed to share it before publishing"},
		{name: "consent given with the publish", in: Input{Title: "A"},
			patch: Patch{Status: published, ConsentConfirmed: family.Opt[bool]{Set: true, Value: true}}, wantStatus: "published"},
		{name: "consent given at the start", in: Input{Title: "A", ConsentConfirmed: true}, patch: Patch{Status: published}, wantStatus: "published"},
		{name: "consent taken back while publishing", in: Input{Title: "A", ConsentConfirmed: true},
			patch:      Patch{Status: published, ConsentConfirmed: family.Opt[bool]{Set: true}},
			wantErrMsg: "invalid story: confirm the storyteller agreed to share it before publishing"},
		{name: "an unknown status", in: Input{Title: "A", ConsentConfirmed: true}, patch: Patch{Status: family.Opt[string]{Set: true, Value: "live"}},
			wantErrMsg: "invalid story: the status must be draft or published"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := f.svc.Create(t.Context(), f.member, tt.in)
			require.NoError(t, err)

			got, err := f.svc.Update(t.Context(), f.member, created.ID, tt.patch)

			if tt.wantErrMsg != "" {
				require.ErrorIs(t, err, ErrInvalidStory)
				assert.EqualError(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, got.Status)
		})
	}

	t.Run("consent can't be taken back from a published story", func(t *testing.T) {
		story := f.publish(t, f.member, Input{Title: "B"})
		_, err := f.svc.Update(t.Context(), f.member, story.ID, Patch{ConsentConfirmed: family.Opt[bool]{Set: true}})
		require.ErrorIs(t, err, ErrInvalidStory)
	})
}

func TestStoryOfTheWeek(t *testing.T) {
	f := newFixture(t)
	featured := family.Opt[bool]{Set: true, Value: true}
	first := f.publish(t, f.member, Input{Title: "First"})
	second := f.publish(t, f.member, Input{Title: "Second"})
	draft, err := f.svc.Create(t.Context(), f.member, Input{Title: "Draft"})
	require.NoError(t, err)

	_, err = f.svc.Update(t.Context(), f.member, first.ID, Patch{Featured: featured})
	require.ErrorIs(t, err, ErrNotAllowed)
	_, err = f.svc.Update(t.Context(), f.admin, draft.ID, Patch{Featured: featured})
	require.EqualError(t, err, "invalid story: only a published story can be the story of the week")

	got, err := f.svc.Update(t.Context(), f.admin, first.ID, Patch{Featured: featured})
	require.NoError(t, err)
	assert.True(t, got.Featured)
	assert.Equal(t, []string{"First", "Second"}, f.listTitles(t, Filter{}))

	_, err = f.svc.Update(t.Context(), f.admin, second.ID, Patch{Featured: featured})
	require.NoError(t, err)
	assert.Equal(t, []string{"Second", "First"}, f.listTitles(t, Filter{}))
	got, err = f.svc.Get(t.Context(), f.admin, first.ID)
	require.NoError(t, err)
	assert.False(t, got.Featured, "featuring a story takes it from the one before")

	got, err = f.svc.Update(t.Context(), f.member, second.ID, Patch{Status: family.Opt[string]{Set: true, Value: "draft"}})
	require.NoError(t, err)
	assert.False(t, got.Featured, "a story taken back to draft stops being the story of the week")
}

func TestListFilters(t *testing.T) {
	f := newFixture(t)
	jr, mc := f.person(t, "Jean-Robert", "Pluviose"), f.person(t, "Marie-Claire", "Pluviose")
	f.publish(t, f.member, Input{Title: "How we came to Brooklyn", TellerID: &jr, Topics: []string{"Moving"},
		Transcript: []Line{{At: 1620, Text: "I saw snow for the first time.\nI ran out in my socks."}}})
	f.publish(t, f.member, Input{Title: "Joseph and the yellow cab", TellerID: &mc, Topics: []string{"Work"},
		Summary: new("Thirty years behind the wheel.")})
	f.publish(t, f.member, Input{Title: "Our wedding", TellerID: &mc, Topics: []string{"Weddings"}})

	tests := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{name: "everything, latest first", filter: Filter{}, want: []string{"Our wedding", "Joseph and the yellow cab", "How we came to Brooklyn"}},
		{name: "by teller", filter: Filter{TellerID: &mc}, want: []string{"Our wedding", "Joseph and the yellow cab"}},
		{name: "by topic, in any case", filter: Filter{Topic: new("work")}, want: []string{"Joseph and the yellow cab"}},
		{name: "a word after a line break in the transcript", filter: Filter{Q: new("ran")}, want: []string{"How we came to Brooklyn"}},
		{name: "a word from the transcript", filter: Filter{Q: new("snow")}, want: []string{"How we came to Brooklyn"}},
		{name: "a word from the summary", filter: Filter{Q: new(" wheel ")}, want: []string{"Joseph and the yellow cab"}},
		{name: "punctuation alone is no search", filter: Filter{Q: new("?")}, want: []string{"Our wedding", "Joseph and the yellow cab", "How we came to Brooklyn"}},
		{name: "teller and topic together", filter: Filter{TellerID: &jr, Topic: new("Work")}, want: []string{}},
		{name: "blank search is no search", filter: Filter{Q: new("  "), Topic: new(" ")}, want: []string{"Our wedding", "Joseph and the yellow cab", "How we came to Brooklyn"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, f.listTitles(t, tt.filter))
		})
	}
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	jr, joseph := f.person(t, "Jean-Robert", "Pluviose"), f.person(t, "Joseph", "Pluviose")
	created, err := f.svc.Create(t.Context(), f.member, Input{Title: "Brooklyn", TellerID: &jr, PersonIDs: []uuid.UUID{joseph},
		RecordedOn: new("2026"), RecordedOnPrecision: new("year"), Topics: []string{"Moving"}})
	require.NoError(t, err)

	got, err := f.svc.Update(t.Context(), f.member, created.ID, Patch{
		Title:      family.Opt[string]{Set: true, Value: "How we came to Brooklyn"},
		TellerID:   family.Opt[*uuid.UUID]{Set: true},
		PersonIDs:  family.Opt[[]uuid.UUID]{Set: true, Value: []uuid.UUID{jr}},
		Chapters:   family.Opt[[]Chapter]{Set: true, Value: []Chapter{{At: 0, Title: "Jacmel"}}},
		Transcript: family.Opt[[]Line]{Set: true, Value: []Line{{At: 3, Text: "Nou prale."}}},
	})
	require.NoError(t, err)

	assert.Equal(t, "How we came to Brooklyn", got.Title)
	assert.Nil(t, got.Teller)
	assert.Equal(t, []Person{{ID: jr, FirstName: "Jean-Robert", LastName: new("Pluviose")}}, got.People)
	assert.Equal(t, []Chapter{{At: 0, Title: "Jacmel"}}, got.Chapters)
	assert.Equal(t, []Line{{At: 3, Text: "Nou prale."}}, got.Transcript)
	assert.Equal(t, new("2026"), got.RecordedOn, "fields not in the patch are kept")
	assert.Equal(t, []string{"Moving"}, got.Topics)

	_, err = f.svc.Update(t.Context(), f.member, created.ID, Patch{RecordedOn: family.Opt[*string]{Set: true, Value: new("1972")}})
	require.EqualError(t, err, "invalid story: give the date and its precision together")
}

func TestOnlyAdminsAndTheCreatorChangeAPublishedStory(t *testing.T) {
	f := newFixture(t)
	story := f.publish(t, f.member, Input{Title: "Brooklyn"})
	rename := Patch{Title: family.Opt[string]{Set: true, Value: "Renamed"}}

	tests := []struct {
		name    string
		viewer  family.Viewer
		wantErr error
	}{
		{name: "another member", viewer: f.other, wantErr: ErrNotAllowed},
		{name: "the creator", viewer: f.member},
		{name: "an admin", viewer: f.admin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.svc.Get(t.Context(), tt.viewer, story.ID)
			require.NoError(t, err, "a published story is seen by everyone")
			assert.Equal(t, "published", got.Status)
			_, err = f.svc.Update(t.Context(), tt.viewer, story.ID, rename)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
	require.ErrorIs(t, f.svc.Delete(t.Context(), f.other, story.ID), ErrNotAllowed)
}

func TestListLeavesOutChaptersAndTranscript(t *testing.T) {
	f := newFixture(t)
	f.publish(t, f.member, Input{Title: "Brooklyn", Chapters: []Chapter{{At: 0, Title: "Jacmel"}}, Transcript: []Line{{At: 0, Text: "Nou prale."}}})

	stories, err := f.svc.List(t.Context(), f.member, Filter{})

	require.NoError(t, err)
	require.Len(t, stories, 1)
	assert.Empty(t, stories[0].Chapters)
	assert.Empty(t, stories[0].Transcript)
}

func TestTopicsAreNamedOnceInAnyCase(t *testing.T) {
	f := newFixture(t)

	story, err := f.svc.Create(t.Context(), f.member, Input{Title: "Brooklyn", Topics: []string{"Work", "work", "WORK "}})

	require.NoError(t, err)
	assert.Equal(t, []string{"Work"}, story.Topics)
}

func TestADeletedTellerDropsOut(t *testing.T) {
	f := newFixture(t)
	jr, joseph := f.person(t, "Jean-Robert", "Pluviose"), f.person(t, "Joseph", "Pluviose")
	story := f.publish(t, f.member, Input{Title: "Brooklyn", TellerID: &jr, PersonIDs: []uuid.UUID{jr}})
	_, err := f.pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, jr)
	require.NoError(t, err)

	got, err := f.svc.Get(t.Context(), f.member, story.ID)

	require.NoError(t, err)
	assert.Nil(t, got.Teller)
	assert.Empty(t, got.People)
	got, err = f.svc.Update(t.Context(), f.member, story.ID, Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true, Value: []uuid.UUID{joseph}}})
	require.NoError(t, err, "the people can still change")
	assert.Equal(t, []Person{{ID: joseph, FirstName: "Joseph", LastName: new("Pluviose")}}, got.People)
}

func TestOneStoryOfTheWeekWhenTwoAreFeaturedAtOnce(t *testing.T) {
	f := newFixture(t)
	featured := family.Opt[bool]{Set: true, Value: true}
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		ids[i] = f.publish(t, f.member, Input{Title: "Story"}).ID
	}

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Go(func() {
			_, err := f.svc.Update(t.Context(), f.admin, id, Patch{Featured: featured})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	var n int
	require.NoError(t, f.pool.QueryRow(t.Context(), `SELECT count(*) FROM stories WHERE featured_at IS NOT NULL`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	story := f.publish(t, f.member, Input{Title: "Brooklyn"})

	require.NoError(t, f.svc.Delete(t.Context(), f.member, story.ID))

	_, err := f.svc.Get(t.Context(), f.member, story.ID)
	require.ErrorIs(t, err, ErrStoryNotFound)
	assert.Empty(t, f.listTitles(t, Filter{}))
	_, err = f.svc.Update(t.Context(), f.admin, story.ID, Patch{})
	require.ErrorIs(t, err, ErrStoryNotFound)
	require.ErrorIs(t, f.svc.Delete(t.Context(), f.admin, story.ID), ErrStoryNotFound)
}

type fixture struct {
	svc                  *Service
	pool                 *pgxpool.Pool
	admin, member, other family.Viewer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testdb.MigratedPool(t, database.MigrateUp)
	user := func(email, role string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role) VALUES ($1, $2) RETURNING id`, email, role).Scan(&id))
		return id
	}
	return fixture{
		svc: NewService(pool), pool: pool,
		admin:  family.Viewer{UserID: user("admin@example.com", "admin"), IsAdmin: true},
		member: family.Viewer{UserID: user("maya@example.com", "member")},
		other:  family.Viewer{UserID: user("leo@example.com", "member")},
	}
}

func (f fixture) person(t *testing.T, first, last string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES ($1, $2) RETURNING id`, first, last).Scan(&id))
	return id
}

func (f fixture) publish(t *testing.T, viewer family.Viewer, in Input) Story {
	t.Helper()
	in.ConsentConfirmed = true
	created, err := f.svc.Create(t.Context(), viewer, in)
	require.NoError(t, err)
	story, err := f.svc.Update(t.Context(), viewer, created.ID, Patch{Status: family.Opt[string]{Set: true, Value: "published"}})
	require.NoError(t, err)
	return story
}

func (f fixture) listTitles(t *testing.T, filter Filter) []string {
	t.Helper()
	stories, err := f.svc.List(t.Context(), f.member, filter)
	require.NoError(t, err)
	return titles(stories)
}

func titles(stories []Story) []string {
	out := make([]string, len(stories))
	for i, s := range stories {
		out[i] = s.Title
	}
	return out
}

func numbered(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + " " + strings.Repeat("i", i+1)
	}
	return out
}

func longTranscript() []Line {
	lines := make([]Line, maxTranscriptLength/maxLineLength+1)
	for i := range lines {
		lines[i] = Line{At: i, Text: strings.Repeat("a", maxLineLength)}
	}
	return lines
}

func TestTheTellerMayHideTheirStory(t *testing.T) {
	f := newFixture(t)
	grandma := f.person(t, "Marie-Claire", "Pluviose")
	teller := family.Viewer{UserID: f.other.UserID, PersonID: &grandma}
	toDraft := family.Opt[string]{Set: true, Value: "draft"}
	noConsent := family.Opt[bool]{Set: true}

	tests := []struct {
		name       string
		patch      Patch
		wantErr    error
		wantStatus string
	}{
		{name: "back to draft and consent withdrawn", patch: Patch{Status: toDraft, ConsentConfirmed: noConsent}, wantStatus: "draft"},
		{name: "back to draft", patch: Patch{Status: toDraft}, wantStatus: "draft"},
		{name: "consent alone, while it's published", patch: Patch{ConsentConfirmed: noConsent}, wantErr: ErrInvalidStory},
		{name: "a new title", patch: Patch{Title: family.Opt[string]{Set: true, Value: "Mine"}}, wantErr: ErrNotAllowed},
		{name: "hiding and a new title together", patch: Patch{Status: toDraft, Title: family.Opt[string]{Set: true, Value: "Mine"}}, wantErr: ErrNotAllowed},
		{name: "publishing again", patch: Patch{Status: family.Opt[string]{Set: true, Value: "published"}}, wantErr: ErrNotAllowed},
		{name: "an empty change", patch: Patch{}, wantErr: ErrNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			story := f.publish(t, f.member, Input{Title: "Our wedding", TellerID: &grandma})

			got, err := f.svc.Update(t.Context(), teller, story.ID, tt.patch)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.False(t, got.ConsentConfirmed, "hiding withdraws consent, so it isn't republished without asking again")
			_, err = f.svc.Update(t.Context(), f.member, story.ID, Patch{Status: family.Opt[string]{Set: true, Value: "published"}})
			require.ErrorIs(t, err, ErrInvalidStory)
			_, err = f.svc.Get(t.Context(), teller, story.ID)
			require.NoError(t, err, "the teller still sees their hidden story")
			_, err = f.svc.Get(t.Context(), f.admin, story.ID)
			require.NoError(t, err)
			_, err = f.svc.Get(t.Context(), family.Viewer{UserID: uuid.New()}, story.ID)
			require.ErrorIs(t, err, ErrStoryNotFound, "the rest of the family no longer sees it")
		})
	}

	t.Run("their drafts list has the stories they tell, while they're in the tree", func(t *testing.T) {
		fresh := newFixture(t)
		aunt := fresh.person(t, "Nadège", "Pluviose")
		teller := family.Viewer{UserID: fresh.other.UserID, PersonID: &aunt}
		_, err := fresh.svc.Create(t.Context(), fresh.member, Input{Title: "Thirty years of third grade", TellerID: &aunt})
		require.NoError(t, err)
		_, err = fresh.svc.Create(t.Context(), fresh.member, Input{Title: "Someone else's"})
		require.NoError(t, err)

		drafts, err := fresh.svc.List(t.Context(), teller, Filter{Drafts: true})
		require.NoError(t, err)
		assert.Equal(t, []string{"Thirty years of third grade"}, titles(drafts))

		_, err = fresh.pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, aunt)
		require.NoError(t, err)
		drafts, err = fresh.svc.List(t.Context(), teller, Filter{Drafts: true})
		require.NoError(t, err)
		assert.Empty(t, drafts)
	})

	t.Run("they can't delete it", func(t *testing.T) {
		story := f.publish(t, f.member, Input{Title: "Our wedding", TellerID: &grandma})
		require.ErrorIs(t, f.svc.Delete(t.Context(), teller, story.ID), ErrNotAllowed)
	})
}

func TestANewTellerMustConsentAgain(t *testing.T) {
	f := newFixture(t)
	jr, nad := f.person(t, "Jean-Robert", "Pluviose"), f.person(t, "Nadège", "Pluviose")
	consent := family.Opt[bool]{Set: true, Value: true}

	tests := []struct {
		name        string
		published   bool
		patch       Patch
		wantErr     error
		wantConsent bool
	}{
		{name: "a published story, new teller", published: true, patch: Patch{TellerID: family.Opt[*uuid.UUID]{Set: true, Value: &nad}}, wantErr: ErrInvalidStory},
		{name: "a published story, new teller who agreed", published: true,
			patch: Patch{TellerID: family.Opt[*uuid.UUID]{Set: true, Value: &nad}, ConsentConfirmed: consent}, wantConsent: true},
		{name: "a consented draft, new teller", patch: Patch{TellerID: family.Opt[*uuid.UUID]{Set: true, Value: &nad}}},
		{name: "a consented draft, no teller any more", patch: Patch{TellerID: family.Opt[*uuid.UUID]{Set: true}}},
		{name: "the same teller sent again", patch: Patch{TellerID: family.Opt[*uuid.UUID]{Set: true, Value: &jr}}, wantConsent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := Input{Title: "Brooklyn", TellerID: &jr, ConsentConfirmed: true}
			story := f.publish(t, f.member, in)
			if !tt.published {
				var err error
				story, err = f.svc.Update(t.Context(), f.member, story.ID, Patch{Status: family.Opt[string]{Set: true, Value: "draft"}})
				require.NoError(t, err)
			}

			got, err := f.svc.Update(t.Context(), f.member, story.ID, tt.patch)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantConsent, got.ConsentConfirmed)
		})
	}
}

func TestATellerWhoCanEditStillWithdrawsConsentByHiding(t *testing.T) {
	f := newFixture(t)
	grandma := f.person(t, "Marie-Claire", "Pluviose")

	tests := []struct {
		name   string
		viewer family.Viewer
	}{
		{name: "the teller is an admin", viewer: family.Viewer{UserID: f.admin.UserID, IsAdmin: true, PersonID: &grandma}},
		{name: "the teller added it", viewer: family.Viewer{UserID: f.member.UserID, PersonID: &grandma}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			story := f.publish(t, f.member, Input{Title: "Our wedding", TellerID: &grandma})

			got, err := f.svc.Update(t.Context(), tt.viewer, story.ID, Patch{Status: family.Opt[string]{Set: true, Value: "draft"}})

			require.NoError(t, err)
			assert.False(t, got.ConsentConfirmed)
		})
	}
}

func TestATranscriptTooVariedToSearchIsRefused(t *testing.T) {
	f := newFixture(t)
	// Distinct three-character words in a multibyte script: within the length limits, over Postgres's 1 MB tsvector.
	var lines []Line
	var b strings.Builder
	for i := 0; i < maxTranscriptLength/4; i++ {
		b.WriteRune(rune(0x4E00 + i/400))
		b.WriteRune(rune(0x4E00 + (i/20)%20))
		b.WriteRune(rune(0x4E00 + i%20))
		b.WriteRune(' ')
		if (i+1)%(maxLineLength/4) == 0 {
			lines = append(lines, Line{At: len(lines), Text: b.String()})
			b.Reset()
		}
	}
	lines = append(lines, Line{At: len(lines), Text: b.String()})

	_, err := f.svc.Create(t.Context(), f.member, Input{Title: "Long", Transcript: lines})

	require.ErrorIs(t, err, ErrInvalidStory)
	assert.EqualError(t, err, "create story: invalid story: the transcript has too many different words to search; split it into two stories")
}
