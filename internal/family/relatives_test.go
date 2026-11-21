package family

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddRelative(t *testing.T) {
	bothParents := [][3]string{{"Rosa", "Marie", "biological"}, {"Jean", "Marie", "biological"}}

	tests := []struct {
		name      string
		links     [][3]string
		deleted   string
		from      string
		in        func(ids map[string]uuid.UUID) RelativeInput
		wantName  string
		wantLinks []string
	}{
		{name: "a new parent, biological unless told otherwise", from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, Person: &PersonInput{FirstName: "Paul"}}
			},
			wantName: "Paul", wantLinks: []string{"Paul > Marie, biological"}},
		{name: "an adoptive parent already in the tree", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"], Subtype: new("adoptive")}
			},
			wantName: "Rosa", wantLinks: []string{"Rosa > Marie, adoptive"}},
		{name: "a child, with their other parent", from: "Rosa",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationChild, Person: &PersonInput{FirstName: "Paul"}, OtherParentID: ids["Jean"]}
			},
			wantName: "Paul", wantLinks: []string{"Jean > Paul, biological", "Rosa > Paul, biological"}},
		{name: "a foster child", from: "Rosa",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationChild, Person: &PersonInput{FirstName: "Paul"}, Subtype: new("foster")}
			},
			wantName: "Paul", wantLinks: []string{"Rosa > Paul, foster"}},
		{name: "a former wife", from: "Jean",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationPartner, PersonID: ids["Rosa"], Subtype: new("married"), Ended: true}
			},
			wantName: "Rosa", wantLinks: []string{"Jean & Rosa, married, ended"}},
		{name: "a full sibling, through both parents", links: bothParents, from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: &PersonInput{FirstName: "Paul"}}
			},
			wantName: "Paul", wantLinks: []string{"Jean > Paul, biological", "Rosa > Paul, biological"}},
		{name: "a half sibling, through the parent they share", links: bothParents, from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: &PersonInput{FirstName: "Paul"}, SharedParentID: ids["Rosa"]}
			},
			wantName: "Paul", wantLinks: []string{"Rosa > Paul, biological"}},
		{name: "a sibling, but not through a foster parent", links: [][3]string{{"Rosa", "Marie", "adoptive"}, {"Jean", "Marie", "foster"}}, from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: &PersonInput{FirstName: "Paul"}}
			},
			wantName: "Paul", wantLinks: []string{"Rosa > Paul, biological"}},
		{name: "a sibling, but not through a deleted parent", links: bothParents, deleted: "Jean", from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: &PersonInput{FirstName: "Paul"}}
			},
			wantName: "Paul", wantLinks: []string{"Rosa > Paul, biological"}},
		{name: "a full sibling who already shares one parent", links: append(bothParents, [3]string{"Rosa", "Lucie", "biological"}), from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, PersonID: ids["Lucie"]}
			},
			wantName: "Lucie", wantLinks: []string{"Jean > Lucie, biological"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := addPeople(t, svc, admin, "Rosa", "Jean", "Marie", "Lucie")
			for _, l := range tt.links {
				addLink(t, svc, parentLink(ids[l[0]], ids[l[1]], l[2]))
			}
			if tt.deleted != "" {
				require.NoError(t, svc.DeletePerson(t.Context(), ids[tt.deleted]))
			}
			in := tt.in(ids)

			got, err := svc.AddRelative(t.Context(), admin, ids[tt.from], in)

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got.Person.FirstName)
			ids[got.Person.FirstName] = got.Person.ID
			assert.Equal(t, tt.wantLinks, describeLinks(ids, got.Relationships))
			for _, rel := range got.Relationships {
				assert.Equal(t, rel, storedRelationship(t, svc, rel.ID))
			}
			if in.Person != nil {
				var createdBy uuid.UUID
				require.NoError(t, svc.pool.QueryRow(t.Context(), `SELECT created_by FROM persons WHERE id = $1`, got.Person.ID).Scan(&createdBy))
				assert.Equal(t, admin, createdBy)
			}
		})
	}
}

func TestAddRelativeSavesNothingWhenRefused(t *testing.T) {
	bothParents := [][3]string{{"Rosa", "Marie", "biological"}, {"Jean", "Marie", "biological"}}
	paul := &PersonInput{FirstName: "Paul"}

	tests := []struct {
		name    string
		links   [][3]string
		deleted string
		from    string
		in      func(ids map[string]uuid.UUID) RelativeInput
		wantErr string
	}{
		{name: "an unknown relation", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: "cousin", PersonID: ids["Rosa"]}
			},
			wantErr: "invalid relationship: relation must be one of parent, child, partner, sibling"},
		{name: "someone in the tree and someone new", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"], Person: paul}
			},
			wantErr: "invalid relationship: give either person_id or a new person"},
		{name: "nobody", from: "Marie", in: func(map[string]uuid.UUID) RelativeInput { return RelativeInput{Relation: RelationParent} },
			wantErr: "invalid relationship: give either person_id or a new person"},
		{name: "themselves", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationPartner, PersonID: ids["Marie"], Subtype: new("married")}
			},
			wantErr: "invalid relationship: someone can't be their own relative"},
		{name: "an ended parent link", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"], Ended: true}
			},
			wantErr: "invalid relationship: only a partner can be ended"},
		{name: "another parent for a parent", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"], OtherParentID: ids["Jean"]}
			},
			wantErr: "invalid relationship: other_parent_id is only for a child"},
		{name: "the person as their child's other parent", from: "Rosa",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationChild, Person: paul, OtherParentID: ids["Rosa"]}
			},
			wantErr: "invalid relationship: the other parent must be someone else"},
		{name: "a shared parent for a partner", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationPartner, PersonID: ids["Jean"], Subtype: new("married"), SharedParentID: ids["Rosa"]}
			},
			wantErr: "invalid relationship: shared_parent_id is only for a sibling"},
		{name: "someone new without a first name", from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, Person: &PersonInput{FirstName: " "}}
			},
			wantErr: "invalid person: a first name is required"},
		{name: "a new partner without a subtype", from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationPartner, Person: paul}
			},
			wantErr: "invalid relationship: a couple needs a subtype: married or partner"},
		{name: "a sibling of someone with no parent yet", from: "Marie",
			in: func(map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: paul}
			},
			wantErr: "invalid relationship: siblings are linked through their parents; add a parent first"},
		{name: "a half sibling through someone who isn't their parent", links: [][3]string{{"Rosa", "Marie", "biological"}}, from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, Person: paul, SharedParentID: ids["Jean"]}
			},
			wantErr: "invalid relationship: the shared parent must be one of their parents"},
		{name: "someone who is already their sibling", links: append(bothParents, [3]string{"Rosa", "Lucie", "biological"}, [3]string{"Jean", "Lucie", "adoptive"}), from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, PersonID: ids["Lucie"]}
			},
			wantErr: "relationship conflict: they're already siblings"},
		{name: "a sibling through a parent who only fosters them", links: append(bothParents, [3]string{"Rosa", "Lucie", "foster"}), from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationSibling, PersonID: ids["Lucie"], SharedParentID: ids["Rosa"]}
			},
			wantErr: "relationship conflict: they're already linked this way; edit that link instead"},
		{name: "a new third biological parent", links: bothParents, from: "Marie",
			in:      func(map[string]uuid.UUID) RelativeInput { return RelativeInput{Relation: RelationParent, Person: paul} },
			wantErr: "relationship conflict: the child already has two biological parents"},
		{name: "a child whose other parent would be a third, so the first link goes too", links: [][3]string{{"Jean", "Marie", "biological"}}, from: "Rosa",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationChild, PersonID: ids["Marie"], OtherParentID: ids["Lucie"]}
			},
			wantErr: "relationship conflict: the child already has two biological parents"},
		{name: "a parent who is already their child", links: [][3]string{{"Marie", "Rosa", "biological"}}, from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"]}
			},
			wantErr: "relationship conflict: the child is already an ancestor of the parent"},
		{name: "a deleted relative", deleted: "Rosa", from: "Marie",
			in: func(ids map[string]uuid.UUID) RelativeInput {
				return RelativeInput{Relation: RelationParent, PersonID: ids["Rosa"]}
			},
			wantErr: "invalid relationship: both people must be in the tree"},
		{name: "from someone deleted", deleted: "Marie", from: "Marie",
			in:      func(map[string]uuid.UUID) RelativeInput { return RelativeInput{Relation: RelationParent, Person: paul} },
			wantErr: "person not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := addPeople(t, svc, admin, "Rosa", "Jean", "Marie", "Lucie")
			for _, l := range tt.links {
				addLink(t, svc, parentLink(ids[l[0]], ids[l[1]], l[2]))
			}
			if tt.deleted != "" {
				require.NoError(t, svc.DeletePerson(t.Context(), ids[tt.deleted]))
			}
			before := storedCounts(t, svc)

			_, err := svc.AddRelative(t.Context(), admin, ids[tt.from], tt.in(ids))

			assert.EqualError(t, err, tt.wantErr)
			assert.Equal(t, before, storedCounts(t, svc), "nothing saved")
		})
	}
}

// describeLinks reads links by name: "Rosa > Marie, biological" for a parent link, "Jean & Rosa, married" for a couple.
func describeLinks(ids map[string]uuid.UUID, rels []Relationship) []string {
	names := map[uuid.UUID]string{}
	for name, id := range ids {
		names[id] = name
	}
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		if r.Type == ParentChild {
			out = append(out, fmt.Sprintf("%s > %s, %s", names[r.PersonA], names[r.PersonB], r.Subtype))
			continue
		}
		pair := []string{names[r.PersonA], names[r.PersonB]}
		slices.Sort(pair)
		link := fmt.Sprintf("%s & %s, %s", pair[0], pair[1], r.Subtype)
		if r.Ended {
			link += ", ended"
		}
		out = append(out, link)
	}
	slices.Sort(out)
	return out
}

func storedCounts(t *testing.T, svc *Service) [2]int {
	t.Helper()
	var counts [2]int
	require.NoError(t, svc.pool.QueryRow(t.Context(),
		`SELECT (SELECT count(*) FROM persons), (SELECT count(*) FROM relationships)`).Scan(&counts[0], &counts[1]))
	return counts
}
