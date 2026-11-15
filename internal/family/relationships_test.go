package family

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
)

func TestCreateRelationshipRefusesInvalidInput(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name       string
		in         RelationshipInput
		wantErrMsg string
	}{
		{name: "unknown type", in: RelationshipInput{Type: "sibling", ParentID: a, ChildID: b}, wantErrMsg: "type must be parent_child or couple"},
		{name: "a parent link with person_ids", in: RelationshipInput{Type: ParentChild, PersonIDs: []uuid.UUID{a, b}},
			wantErrMsg: "a parent link takes parent_id and child_id, not person_ids"},
		{name: "a parent link without a child", in: RelationshipInput{Type: ParentChild, ParentID: a}, wantErrMsg: "a parent link needs parent_id and child_id"},
		{name: "one's own parent", in: RelationshipInput{Type: ParentChild, ParentID: a, ChildID: a}, wantErrMsg: "someone can't be their own parent"},
		{name: "an ended parent link", in: RelationshipInput{Type: ParentChild, ParentID: a, ChildID: b, Ended: true}, wantErrMsg: "only a couple can be ended"},
		{name: "a parent link with a couple's subtype", in: RelationshipInput{Type: ParentChild, ParentID: a, ChildID: b, Subtype: new("married")},
			wantErrMsg: "subtype must be one of biological, adoptive, foster"},
		{name: "a couple with parent_id", in: RelationshipInput{Type: Couple, ParentID: a, PersonIDs: []uuid.UUID{a, b}, Subtype: new("married")},
			wantErrMsg: "a couple takes person_ids, not parent_id and child_id"},
		{name: "a couple of one", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{a}, Subtype: new("married")},
			wantErrMsg: "a couple needs person_ids with two different people"},
		{name: "a couple of three", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{a, b, c}, Subtype: new("married")},
			wantErrMsg: "a couple needs person_ids with two different people"},
		{name: "a couple of the same person", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{a, a}, Subtype: new("married")},
			wantErrMsg: "a couple needs person_ids with two different people"},
		{name: "a couple without a subtype", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{a, b}},
			wantErrMsg: "a couple needs a subtype: married or partner"},
		{name: "a couple with a parent link's subtype", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{a, b}, Subtype: new("biological")},
			wantErrMsg: "subtype must be one of married, partner"},
		{name: "notes too long", in: RelationshipInput{Type: ParentChild, ParentID: a, ChildID: b, Notes: new(strings.Repeat("é", maxNotesLength+1))},
			wantErrMsg: "the notes are longer than 1000 characters"},
		{name: "a NUL in the notes", in: RelationshipInput{Type: ParentChild, ParentID: a, ChildID: b, Notes: new("Raised\x00by her aunt")},
			wantErrMsg: "the notes contain a character that isn't allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(nil).CreateRelationship(t.Context(), tt.in)

			require.ErrorIs(t, err, ErrInvalidRelationship)
			assert.Equal(t, "invalid relationship: "+tt.wantErrMsg, err.Error())
		})
	}
}

func TestCreateRelationship(t *testing.T) {
	svc, admin := newService(t)
	ids := addPeople(t, svc, admin, "Rosa", "Jean", "Marie")
	lower, higher := ids["Rosa"], ids["Jean"]
	if bytes.Compare(lower[:], higher[:]) > 0 {
		lower, higher = higher, lower
	}

	tests := []struct {
		name string
		in   RelationshipInput
		want Relationship
	}{
		{name: "a parent link is biological unless told otherwise",
			in:   RelationshipInput{Type: ParentChild, ParentID: ids["Rosa"], ChildID: ids["Marie"], Notes: new("  Born in Jacmel  ")},
			want: Relationship{Type: ParentChild, Subtype: "biological", PersonA: ids["Rosa"], PersonB: ids["Marie"], Notes: new("Born in Jacmel")}},
		{name: "an adoptive parent", in: RelationshipInput{Type: ParentChild, ParentID: ids["Jean"], ChildID: ids["Marie"], Subtype: new("adoptive")},
			want: Relationship{Type: ParentChild, Subtype: "adoptive", PersonA: ids["Jean"], PersonB: ids["Marie"]}},
		{name: "a couple is stored lower id first, whatever the order given",
			in:   RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{higher, lower}, Subtype: new("married"), Ended: true},
			want: Relationship{Type: Couple, Subtype: "married", Ended: true, PersonA: lower, PersonB: higher}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CreateRelationship(t.Context(), tt.in)

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, got.ID)
			tt.want.ID = got.ID
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want, storedRelationship(t, svc, got.ID))
		})
	}
}

func TestCreateRelationshipNeedsBothPeopleInTheTree(t *testing.T) {
	svc, admin := newService(t)
	ids := addPeople(t, svc, admin, "Rosa", "Jean")
	require.NoError(t, svc.DeletePerson(t.Context(), ids["Jean"]))

	tests := []struct {
		name string
		in   RelationshipInput
	}{
		{name: "an unknown parent", in: parentLink(uuid.New(), ids["Rosa"], "biological")},
		{name: "a deleted child", in: parentLink(ids["Rosa"], ids["Jean"], "biological")},
		{name: "a deleted partner", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{ids["Rosa"], ids["Jean"]}, Subtype: new("married")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateRelationship(t.Context(), tt.in)

			assert.EqualError(t, err, "invalid relationship: both people must be in the tree")
		})
	}
}

func TestDuplicateLinksAreRefused(t *testing.T) {
	svc, admin := newService(t)
	ids := addPeople(t, svc, admin, "Rosa", "Jean", "Marie")
	addLink(t, svc, parentLink(ids["Rosa"], ids["Marie"], "biological"))
	addLink(t, svc, RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{ids["Rosa"], ids["Jean"]}, Subtype: new("married")})

	tests := []struct {
		name string
		in   RelationshipInput
	}{
		{name: "the same parent link, with another subtype", in: parentLink(ids["Rosa"], ids["Marie"], "adoptive")},
		{name: "the same couple, the other way round", in: RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{ids["Jean"], ids["Rosa"]}, Subtype: new("partner")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateRelationship(t.Context(), tt.in)

			require.ErrorIs(t, err, ErrRelationshipConflict)
			assert.EqualError(t, err, "relationship conflict: they're already linked this way; edit that link instead")
		})
	}
}

func TestAncestryLoopsAreRefused(t *testing.T) {
	deepChain := make([][3]string, 0, 11)
	for i := range 11 {
		deepChain = append(deepChain, [3]string{fmt.Sprint("G", i), fmt.Sprint("G", i+1), "biological"})
	}

	tests := []struct {
		name          string
		links         [][3]string
		deleted       string
		parent, child string
		subtype       string
		wantLoop      bool
	}{
		{name: "a child as their parent's parent", links: [][3]string{{"A", "B", "biological"}}, parent: "B", child: "A", wantLoop: true},
		{name: "a grandchild as a grandparent", links: [][3]string{{"A", "B", "biological"}, {"B", "C", "biological"}}, parent: "C", child: "A", wantLoop: true},
		{name: "a chain deeper than any display bound", links: deepChain, parent: "G11", child: "G0", wantLoop: true},
		{name: "a loop closed through a foster link", links: [][3]string{{"A", "B", "foster"}, {"B", "C", "adoptive"}}, parent: "C", child: "A", subtype: "foster", wantLoop: true},
		{name: "a loop through a deleted person", links: [][3]string{{"A", "B", "biological"}, {"B", "C", "biological"}}, deleted: "B", parent: "C", child: "A", wantLoop: true},
		{name: "a second child", links: [][3]string{{"A", "B", "biological"}}, parent: "A", child: "C"},
		{name: "a grandparent adopting a grandchild", links: [][3]string{{"A", "B", "biological"}, {"B", "C", "biological"}}, parent: "A", child: "C", subtype: "adoptive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := map[string]uuid.UUID{}
			for _, l := range tt.links {
				for _, name := range l[:2] {
					if _, ok := ids[name]; !ok {
						ids[name] = addPeople(t, svc, admin, name)[name]
					}
				}
				addLink(t, svc, parentLink(ids[l[0]], ids[l[1]], l[2]))
			}
			if _, ok := ids[tt.child]; !ok {
				ids[tt.child] = addPeople(t, svc, admin, tt.child)[tt.child]
			}
			if tt.deleted != "" {
				require.NoError(t, svc.DeletePerson(t.Context(), ids[tt.deleted]))
			}
			subtype := tt.subtype
			if subtype == "" {
				subtype = "biological"
			}

			_, err := svc.CreateRelationship(t.Context(), parentLink(ids[tt.parent], ids[tt.child], subtype))

			if !tt.wantLoop {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, "relationship conflict: the child is already an ancestor of the parent")
		})
	}
}

func TestAtMostTwoBiologicalParents(t *testing.T) {
	tests := []struct {
		name         string
		thirdSubtype string
		deleted      string
		wantErr      bool
	}{
		{name: "a third biological parent", thirdSubtype: "biological", wantErr: true},
		{name: "a deleted biological parent still counts", thirdSubtype: "biological", deleted: "Jean", wantErr: true},
		{name: "a third parent by adoption", thirdSubtype: "adoptive"},
		{name: "a foster parent", thirdSubtype: "foster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := addPeople(t, svc, admin, "Rosa", "Jean", "Paul", "Marie")
			addLink(t, svc, parentLink(ids["Rosa"], ids["Marie"], "biological"))
			addLink(t, svc, parentLink(ids["Jean"], ids["Marie"], "biological"))
			if tt.deleted != "" {
				require.NoError(t, svc.DeletePerson(t.Context(), ids[tt.deleted]))
			}

			_, err := svc.CreateRelationship(t.Context(), parentLink(ids["Paul"], ids["Marie"], tt.thirdSubtype))

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, "relationship conflict: the child already has two biological parents")
		})
	}
}

func TestUpdateRelationship(t *testing.T) {
	tests := []struct {
		name       string
		couple     bool
		patch      RelationshipPatch
		want       func(r *Relationship)
		wantErrMsg string
	}{
		{name: "a couple that ended", couple: true, patch: RelationshipPatch{Ended: Opt[bool]{Set: true, Value: true}},
			want: func(r *Relationship) { r.Ended = true }},
		{name: "partners who married", couple: true, patch: RelationshipPatch{Subtype: Opt[string]{Set: true, Value: "married"}},
			want: func(r *Relationship) { r.Subtype = "married" }},
		{name: "notes, trimmed", patch: RelationshipPatch{Notes: Opt[*string]{Set: true, Value: new(" Raised by her aunt ")}},
			want: func(r *Relationship) { r.Notes = new("Raised by her aunt") }},
		{name: "notes cleared", patch: RelationshipPatch{Notes: Opt[*string]{Set: true}}, want: func(r *Relationship) { r.Notes = nil }},
		{name: "a parent who turned out to be a foster parent", patch: RelationshipPatch{Subtype: Opt[string]{Set: true, Value: "foster"}},
			want: func(r *Relationship) { r.Subtype = "foster" }},
		{name: "nothing sent", want: func(*Relationship) {}},
		{name: "an ended parent link", patch: RelationshipPatch{Ended: Opt[bool]{Set: true, Value: true}},
			wantErrMsg: "invalid relationship: only a couple can be ended"},
		{name: "a couple's subtype on a parent link", patch: RelationshipPatch{Subtype: Opt[string]{Set: true, Value: "married"}},
			wantErrMsg: "invalid relationship: subtype must be one of biological, adoptive, foster"},
		{name: "a parent link's subtype on a couple", couple: true, patch: RelationshipPatch{Subtype: Opt[string]{Set: true, Value: "biological"}},
			wantErrMsg: "invalid relationship: subtype must be one of married, partner"},
		{name: "a NUL in the notes", patch: RelationshipPatch{Notes: Opt[*string]{Set: true, Value: new("Raised\x00by her aunt")}},
			wantErrMsg: "invalid relationship: the notes contain a character that isn't allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := addPeople(t, svc, admin, "Rosa", "Marie")
			in := parentLink(ids["Rosa"], ids["Marie"], "biological")
			in.Notes = new("Old notes")
			if tt.couple {
				in = RelationshipInput{Type: Couple, PersonIDs: []uuid.UUID{ids["Rosa"], ids["Marie"]}, Subtype: new("partner"), Notes: new("Old notes")}
			}
			before := addLink(t, svc, in)

			got, err := svc.UpdateRelationship(t.Context(), before.ID, tt.patch)

			if tt.wantErrMsg != "" {
				assert.EqualError(t, err, tt.wantErrMsg)
				assert.Equal(t, before, storedRelationship(t, svc, before.ID), "nothing changed")
				return
			}
			require.NoError(t, err)
			want := before
			tt.want(&want)
			assert.Equal(t, want, got)
			assert.Equal(t, want, storedRelationship(t, svc, before.ID))
		})
	}
}

func TestBecomingBiologicalCountsTheOtherParents(t *testing.T) {
	tests := []struct {
		name       string
		biological []string
		update     string
		subtype    string
		wantErr    bool
	}{
		{name: "an adopter turned biological with two biological parents already", biological: []string{"Rosa", "Jean"}, update: "Paul", subtype: "biological", wantErr: true},
		{name: "an adopter turned biological beside one biological parent", biological: []string{"Rosa"}, update: "Paul", subtype: "biological"},
		{name: "one of two biological parents, re-sent as biological", biological: []string{"Rosa", "Jean"}, update: "Rosa", subtype: "biological"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			ids := addPeople(t, svc, admin, "Rosa", "Jean", "Paul", "Marie")
			links := map[string]Relationship{}
			for _, name := range tt.biological {
				links[name] = addLink(t, svc, parentLink(ids[name], ids["Marie"], "biological"))
			}
			if _, ok := links[tt.update]; !ok {
				links[tt.update] = addLink(t, svc, parentLink(ids[tt.update], ids["Marie"], "adoptive"))
			}

			_, err := svc.UpdateRelationship(t.Context(), links[tt.update].ID, RelationshipPatch{Subtype: Opt[string]{Set: true, Value: tt.subtype}})

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, "relationship conflict: the child already has two biological parents")
			assert.Equal(t, "adoptive", storedRelationship(t, svc, links[tt.update].ID).Subtype)
		})
	}
}

func TestDeleteRelationship(t *testing.T) {
	svc, admin := newService(t)
	ids := addPeople(t, svc, admin, "Rosa", "Marie")
	link := addLink(t, svc, parentLink(ids["Rosa"], ids["Marie"], "biological"))

	require.NoError(t, svc.DeleteRelationship(t.Context(), link.ID))

	var n int
	require.NoError(t, svc.pool.QueryRow(t.Context(), `SELECT count(*) FROM relationships`).Scan(&n))
	assert.Zero(t, n)
	addLink(t, svc, parentLink(ids["Rosa"], ids["Marie"], "adoptive"))
}

func TestUnknownRelationshipIsNotFound(t *testing.T) {
	svc, _ := newService(t)
	id := uuid.New()

	_, updateErr := svc.UpdateRelationship(t.Context(), id, RelationshipPatch{})
	deleteErr := svc.DeleteRelationship(t.Context(), id)

	assert.ErrorIs(t, updateErr, ErrRelationshipNotFound)
	assert.ErrorIs(t, deleteErr, ErrRelationshipNotFound)
}

func TestRelationshipWritesAreBusyWhileAnotherHoldsTheLock(t *testing.T) {
	tests := []struct {
		name  string
		write func(ctx context.Context, s *Service, ids map[string]uuid.UUID, link Relationship) error
	}{
		{name: "create", write: func(ctx context.Context, s *Service, ids map[string]uuid.UUID, _ Relationship) error {
			_, err := s.CreateRelationship(ctx, parentLink(ids["Jean"], ids["Marie"], "biological"))
			return err
		}},
		{name: "update", write: func(ctx context.Context, s *Service, _ map[string]uuid.UUID, link Relationship) error {
			_, err := s.UpdateRelationship(ctx, link.ID, RelationshipPatch{})
			return err
		}},
		{name: "delete", write: func(ctx context.Context, s *Service, _ map[string]uuid.UUID, link Relationship) error {
			return s.DeleteRelationship(ctx, link.ID)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, admin := newService(t)
			svc.lockTimeout = 100 * time.Millisecond
			ids := addPeople(t, svc, admin, "Rosa", "Jean", "Marie")
			link := addLink(t, svc, parentLink(ids["Rosa"], ids["Marie"], "biological"))
			holder, err := svc.pool.Begin(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
			_, err = holder.Exec(t.Context(), `SELECT pg_advisory_xact_lock(hashtext('family.relationships'))`)
			require.NoError(t, err)

			err = tt.write(t.Context(), svc, ids, link)

			assert.ErrorIs(t, err, auth.ErrBusy)
		})
	}
}

func addPeople(t *testing.T, svc *Service, admin uuid.UUID, names ...string) map[string]uuid.UUID {
	t.Helper()
	ids := map[string]uuid.UUID{}
	for _, name := range names {
		p, err := svc.CreatePerson(t.Context(), admin, PersonInput{FirstName: name})
		require.NoError(t, err)
		ids[name] = p.ID
	}
	return ids
}

func parentLink(parent, child uuid.UUID, subtype string) RelationshipInput {
	return RelationshipInput{Type: ParentChild, ParentID: parent, ChildID: child, Subtype: &subtype}
}

func addLink(t *testing.T, svc *Service, in RelationshipInput) Relationship {
	t.Helper()
	rel, err := svc.CreateRelationship(t.Context(), in)
	require.NoError(t, err)
	return rel
}

func storedRelationship(t *testing.T, svc *Service, id uuid.UUID) Relationship {
	t.Helper()
	var r Relationship
	require.NoError(t, svc.pool.QueryRow(t.Context(),
		`SELECT id, type, subtype, ended, person_a, person_b, notes FROM relationships WHERE id = $1`, id).
		Scan(&r.ID, &r.Type, &r.Subtype, &r.Ended, &r.PersonA, &r.PersonB, &r.Notes))
	return r
}
