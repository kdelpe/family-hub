package family

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

func TestTreeLeavesOutDatesNotesAndDeletedPeople(t *testing.T) {
	svc, admin := newService(t)
	rosa, err := svc.CreatePerson(t.Context(), admin, PersonInput{
		FirstName: "Rosa", LastName: new("Pierre"), Side: new("first"), Gender: new("female"),
		BirthDate: new("1950-03-14"), BirthDatePrecision: new("day"), BirthPlace: new("Jacmel"),
		Bio: new(strings.Repeat("a", 137) + "\n bcd"),
	})
	require.NoError(t, err)
	ids := addPeople(t, svc, admin, "Jean", "Marie", "Gone")
	married := addLink(t, svc, RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{rosa.ID, ids["Jean"]}, Subtype: new("married"), Notes: new("Admin note")})
	child := addLink(t, svc, parentLink(rosa.ID, ids["Marie"], "biological"))
	addLink(t, svc, parentLink(ids["Gone"], ids["Marie"], "biological"))
	require.NoError(t, svc.DeletePerson(t.Context(), ids["Gone"]))

	tree, err := svc.Tree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []TreePerson{
		{ID: ids["Jean"], FirstName: "Jean"},
		{ID: ids["Marie"], FirstName: "Marie"},
		{ID: rosa.ID, FirstName: "Rosa", LastName: new("Pierre"), Side: new("first"), Gender: new("female"),
			BioExcerpt: new(strings.Repeat("a", 137) + "…")},
	}, tree.Persons)
	assert.ElementsMatch(t, []TreeLink{
		{ID: married.ID, Type: Couple, Subtype: "married", PersonA: married.PersonA, PersonB: married.PersonB},
		{ID: child.ID, Type: ParentChild, Subtype: "biological", PersonA: rosa.ID, PersonB: ids["Marie"]},
	}, tree.Relationships)
}

func TestBioExcerptKeepsShortBios(t *testing.T) {
	svc, admin := newService(t)
	bio := strings.Repeat("é", 140)
	_, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Rosa", Bio: &bio})
	require.NoError(t, err)

	tree, err := svc.Tree(t.Context())

	require.NoError(t, err)
	require.Len(t, tree.Persons, 1)
	assert.Equal(t, &bio, tree.Persons[0].BioExcerpt)
}

func TestAncestorsAndDescendants(t *testing.T) {
	svc, admin := newService(t)
	great, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Great", BirthDate: new("1960-03-14"), BirthDatePrecision: new("day")})
	require.NoError(t, err)
	ids := addPeople(t, svc, admin, "Grand", "Adopter", "Foster", "Parent", "Other", "Child", "Hidden", "Above")
	ids["Great"] = great.ID
	addLink(t, svc, parentLink(ids["Great"], ids["Grand"], "biological"))
	addLink(t, svc, parentLink(ids["Grand"], ids["Parent"], "biological"))
	addLink(t, svc, parentLink(ids["Adopter"], ids["Parent"], "adoptive"))
	addLink(t, svc, parentLink(ids["Foster"], ids["Child"], "foster"))
	addLink(t, svc, parentLink(ids["Parent"], ids["Child"], "biological"))
	// Great is also Other's parent, so Great is met at depths 2 and 3; the closest counts.
	addLink(t, svc, parentLink(ids["Great"], ids["Other"], "biological"))
	addLink(t, svc, parentLink(ids["Other"], ids["Child"], "adoptive"))
	// Above is reached only through Hidden, who is deleted.
	addLink(t, svc, parentLink(ids["Hidden"], ids["Child"], "biological"))
	addLink(t, svc, parentLink(ids["Above"], ids["Hidden"], "biological"))
	require.NoError(t, svc.DeletePerson(t.Context(), ids["Hidden"]))

	relative := func(name string, depth int) Relative {
		return Relative{ID: ids[name], FirstName: name, Depth: depth}
	}
	greatAs := func(birth string, depth int) Relative {
		r := relative("Great", depth)
		r.BirthDate, r.BirthDatePrecision = &birth, new("day")
		return r
	}

	tests := []struct {
		name   string
		walk   func(Viewer, uuid.UUID, int) ([]Relative, error)
		viewer Viewer
		from   string
		depth  int
		want   []Relative
	}{
		{name: "ancestors skip foster parents and stop at deleted people", walk: walker(t, svc.Ancestors), from: "Child", depth: DefaultDepth,
			want: []Relative{relative("Other", 1), relative("Parent", 1), relative("Adopter", 2), relative("Grand", 2), greatAs("--03-14", 2)}},
		{name: "an admin sees a living person's birth year", walk: walker(t, svc.Ancestors), viewer: Viewer{IsAdmin: true}, from: "Child", depth: DefaultDepth,
			want: []Relative{relative("Other", 1), relative("Parent", 1), relative("Adopter", 2), relative("Grand", 2), greatAs("1960-03-14", 2)}},
		{name: "ancestors stop at the depth", walk: walker(t, svc.Ancestors), from: "Child", depth: 1, want: []Relative{relative("Other", 1), relative("Parent", 1)}},
		{name: "descendants skip foster children", walk: walker(t, svc.Descendants), from: "Grand", depth: DefaultDepth,
			want: []Relative{relative("Parent", 1), relative("Child", 2)}},
		{name: "descendants stop at the depth", walk: walker(t, svc.Descendants), from: "Great", depth: 2,
			want: []Relative{relative("Grand", 1), relative("Other", 1), relative("Child", 2), relative("Parent", 2)}},
		{name: "nobody above", walk: walker(t, svc.Ancestors), from: "Great", depth: DefaultDepth, want: []Relative{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.walk(tt.viewer, ids[tt.from], tt.depth)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRelativesRefuseBadDepthAndUnknownPeople(t *testing.T) {
	svc, admin := newService(t)
	ids := addPeople(t, svc, admin, "Rosa", "Gone")
	require.NoError(t, svc.DeletePerson(t.Context(), ids["Gone"]))

	tests := []struct {
		name    string
		id      uuid.UUID
		depth   int
		wantErr error
	}{
		{name: "depth zero", id: ids["Rosa"], depth: 0, wantErr: ErrInvalidFilter},
		{name: "depth too deep", id: ids["Rosa"], depth: maxDepth + 1, wantErr: ErrInvalidFilter},
		{name: "unknown person", id: uuid.New(), depth: 1, wantErr: ErrPersonNotFound},
		{name: "deleted person", id: ids["Gone"], depth: 1, wantErr: ErrPersonNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Ancestors(t.Context(), Viewer{}, tt.id, tt.depth)
			require.ErrorIs(t, err, tt.wantErr)

			_, err = svc.Descendants(t.Context(), Viewer{}, tt.id, tt.depth)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestSummary(t *testing.T) {
	svc, admin := newService(t)

	empty, err := svc.Summary(t.Context())
	require.NoError(t, err)
	assert.Equal(t, Summary{}, empty)

	elder, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Elder", IsDeceased: true,
		BirthDate: new("1900"), BirthDatePrecision: new("circa"), BirthPlace: new("Jacmel")})
	require.NoError(t, err)
	_, err = svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Older but living", BirthDate: new("1890"), BirthDatePrecision: new("year")})
	require.NoError(t, err)
	gone, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: "Gone", IsDeceased: true, BirthDate: new("1880"), BirthDatePrecision: new("year")})
	require.NoError(t, err)
	ids := addPeople(t, svc, admin, "Child", "Grandchild", "Fostered")
	addLink(t, svc, parentLink(elder.ID, ids["Child"], "biological"))
	addLink(t, svc, parentLink(ids["Child"], ids["Grandchild"], "adoptive"))
	addLink(t, svc, parentLink(ids["Grandchild"], ids["Fostered"], "foster"))
	addLink(t, svc, parentLink(gone.ID, elder.ID, "biological"))
	require.NoError(t, svc.DeletePerson(t.Context(), gone.ID))

	sum, err := svc.Summary(t.Context())

	require.NoError(t, err)
	assert.Equal(t, Summary{People: 5, Generations: 3, EarliestRecord: &EarliestRecord{Year: 1900, Circa: true, Place: new("Jacmel")}}, sum)
}

func TestLongestLine(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name  string
		links []gen.FamilyParentLinksRow
		want  int
	}{
		{name: "no links", want: 1},
		{name: "a branch is as long as its longest line", links: []gen.FamilyParentLinksRow{
			{ParentID: a, ChildID: b}, {ParentID: b, ChildID: c}, {ParentID: a, ChildID: d},
		}, want: 3},
		{name: "two parents of one child", links: []gen.FamilyParentLinksRow{{ParentID: a, ChildID: c}, {ParentID: b, ChildID: c}}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, longestLine(tt.links))
		})
	}
}

func TestLongestLineEndsOnALoop(t *testing.T) {
	a, b := uuid.New(), uuid.New()

	got := longestLine([]gen.FamilyParentLinksRow{{ParentID: a, ChildID: b}, {ParentID: b, ChildID: a}})

	// Where the walk starts decides whether the loop counts 2 or 3; it has to end with one of them.
	assert.Contains(t, []int{2, 3}, got)
}

func walker(t *testing.T, walk func(ctx context.Context, viewer Viewer, id uuid.UUID, depth int) ([]Relative, error)) func(Viewer, uuid.UUID, int) ([]Relative, error) {
	return func(viewer Viewer, id uuid.UUID, depth int) ([]Relative, error) {
		return walk(t.Context(), viewer, id, depth)
	}
}
