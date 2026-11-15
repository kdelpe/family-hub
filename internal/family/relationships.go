package family

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	ParentChild = "parent_child"
	Couple      = "couple"

	biological     = "biological"
	maxNotesLength = 1000
)

var (
	ErrRelationshipNotFound = errors.New("relationship not found")
	ErrInvalidRelationship  = errors.New("invalid relationship")
	// ErrRelationshipConflict refuses a link that clashes with the stored ones: a duplicate, a loop, a third biological parent.
	ErrRelationshipConflict = errors.New("relationship conflict")
)

var subtypes = map[string][]string{
	ParentChild: {"biological", "adoptive", "foster"},
	Couple:      {"married", "partner"},
}

// Relationship is a stored link. For a parent link PersonA is the parent and PersonB the child;
// a couple is stored once, with PersonA the lower id.
type Relationship struct {
	ID      uuid.UUID
	Type    string
	Subtype string
	Ended   bool
	PersonA uuid.UUID
	PersonB uuid.UUID
	Notes   *string
}

// RelationshipInput is a new link: ParentID and ChildID for a parent link, PersonIDs (two, in any order)
// for a couple. A parent link's Subtype defaults to biological; a couple's is required.
type RelationshipInput struct {
	Type      string
	ParentID  uuid.UUID
	ChildID   uuid.UUID
	PersonIDs []uuid.UUID
	Subtype   *string
	Ended     bool
	Notes     *string
}

// RelationshipPatch changes a link's details; its type and people never change (delete and re-create instead).
type RelationshipPatch struct {
	Subtype Opt[string]
	Ended   Opt[bool]
	Notes   Opt[*string]
}

// CreateRelationship is admin-only; the router checks the role.
func (s *Service) CreateRelationship(ctx context.Context, in RelationshipInput) (Relationship, error) {
	rel, err := newRelationship(in)
	if err != nil {
		return Relationship{}, err
	}
	err = s.writeRelationships(ctx, "create relationship", func(q *gen.Queries) error {
		n, err := q.CountPersonsInTree(ctx, []uuid.UUID{rel.PersonA, rel.PersonB})
		if err != nil {
			return fmt.Errorf("check people: %w", err)
		}
		if n != 2 {
			return invalidRelationship("both people must be in the tree")
		}
		exists, err := q.RelationshipExists(ctx, gen.RelationshipExistsParams{Type: rel.Type, PersonA: rel.PersonA, PersonB: rel.PersonB})
		if err != nil {
			return fmt.Errorf("check duplicate: %w", err)
		}
		if exists {
			return fmt.Errorf("%w: they're already linked this way; edit that link instead", ErrRelationshipConflict)
		}
		if rel.Type == ParentChild {
			if err := checkNewParent(ctx, q, rel.PersonA, rel.PersonB, rel.Subtype); err != nil {
				return err
			}
		}
		row, err := q.CreateRelationship(ctx, gen.CreateRelationshipParams{
			Type: rel.Type, Subtype: rel.Subtype, Ended: rel.Ended, PersonA: rel.PersonA, PersonB: rel.PersonB, Notes: rel.Notes,
		})
		if err != nil {
			return fmt.Errorf("create relationship: %w", err)
		}
		rel = Relationship(row)
		return nil
	})
	if err != nil {
		return Relationship{}, err
	}
	return rel, nil
}

// UpdateRelationship is admin-only; the router checks the role.
func (s *Service) UpdateRelationship(ctx context.Context, id uuid.UUID, patch RelationshipPatch) (Relationship, error) {
	var rel Relationship
	err := s.writeRelationships(ctx, "update relationship", func(q *gen.Queries) error {
		row, err := q.LockRelationship(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRelationshipNotFound
		}
		if err != nil {
			return fmt.Errorf("lock relationship: %w", err)
		}
		before := Relationship(row)
		next, err := patch.apply(before).checked()
		if err != nil {
			return err
		}
		if next.Type == ParentChild && next.Subtype == biological && before.Subtype != biological {
			if err := checkBiologicalParents(ctx, q, next.PersonB, next.ID); err != nil {
				return err
			}
		}
		updated, err := q.UpdateRelationship(ctx, gen.UpdateRelationshipParams{ID: id, Subtype: next.Subtype, Ended: next.Ended, Notes: next.Notes})
		if err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
		rel = Relationship(updated)
		return nil
	})
	if err != nil {
		return Relationship{}, err
	}
	return rel, nil
}

// DeleteRelationship removes the link for good; a person's soft delete is what keeps links for a restore.
func (s *Service) DeleteRelationship(ctx context.Context, id uuid.UUID) error {
	return s.writeRelationships(ctx, "delete relationship", func(q *gen.Queries) error {
		n, err := q.DeleteRelationship(ctx, id)
		if err != nil {
			return fmt.Errorf("delete relationship: %w", err)
		}
		if n == 0 {
			return ErrRelationshipNotFound
		}
		return nil
	})
}

// writeRelationships is write under one lock for every relationship write, so their checks can't race (§10).
func (s *Service) writeRelationships(ctx context.Context, what string, fn func(q *gen.Queries) error) error {
	return s.write(ctx, what, func(q *gen.Queries) error {
		if err := q.LockRelationshipWrites(ctx); err != nil {
			return fmt.Errorf("lock relationships: %w", err)
		}
		return fn(q)
	})
}

// checkNewParent refuses a link that would make someone their own ancestor or give a child a third biological parent.
func checkNewParent(ctx context.Context, q *gen.Queries, parent, child uuid.UUID, subtype string) error {
	loop, err := q.IsAncestor(ctx, gen.IsAncestorParams{ParentID: parent, ChildID: child})
	if err != nil {
		return fmt.Errorf("check ancestry: %w", err)
	}
	if loop {
		return fmt.Errorf("%w: the child is already an ancestor of the parent", ErrRelationshipConflict)
	}
	if subtype == biological {
		return checkBiologicalParents(ctx, q, child, uuid.Nil)
	}
	return nil
}

func checkBiologicalParents(ctx context.Context, q *gen.Queries, child, except uuid.UUID) error {
	n, err := q.CountBiologicalParents(ctx, gen.CountBiologicalParentsParams{ChildID: child, ExceptID: except})
	if err != nil {
		return fmt.Errorf("count biological parents: %w", err)
	}
	if n >= 2 {
		return fmt.Errorf("%w: the child already has two biological parents", ErrRelationshipConflict)
	}
	return nil
}

func newRelationship(in RelationshipInput) (Relationship, error) {
	rel := Relationship{Type: in.Type, Ended: in.Ended, Notes: in.Notes}
	switch in.Type {
	case ParentChild:
		if len(in.PersonIDs) > 0 {
			return Relationship{}, invalidRelationship("a parent link takes parent_id and child_id, not person_ids")
		}
		if in.ParentID == uuid.Nil || in.ChildID == uuid.Nil {
			return Relationship{}, invalidRelationship("a parent link needs parent_id and child_id")
		}
		if in.ParentID == in.ChildID {
			return Relationship{}, invalidRelationship("someone can't be their own parent")
		}
		rel.PersonA, rel.PersonB, rel.Subtype = in.ParentID, in.ChildID, biological
		if in.Subtype != nil {
			rel.Subtype = *in.Subtype
		}
	case Couple:
		if in.ParentID != uuid.Nil || in.ChildID != uuid.Nil {
			return Relationship{}, invalidRelationship("a couple takes person_ids, not parent_id and child_id")
		}
		if len(in.PersonIDs) != 2 || in.PersonIDs[0] == in.PersonIDs[1] || slices.Contains(in.PersonIDs, uuid.Nil) {
			return Relationship{}, invalidRelationship("a couple needs person_ids with two different people")
		}
		if in.Subtype == nil {
			return Relationship{}, invalidRelationship("a couple needs a subtype: married or partner")
		}
		rel.PersonA, rel.PersonB, rel.Subtype = in.PersonIDs[0], in.PersonIDs[1], *in.Subtype
		// Postgres orders uuids byte by byte, which is what rel_canonical checks.
		if bytes.Compare(rel.PersonA[:], rel.PersonB[:]) > 0 {
			rel.PersonA, rel.PersonB = rel.PersonB, rel.PersonA
		}
	default:
		return Relationship{}, invalidRelationship("type must be parent_child or couple")
	}
	return rel.checked()
}

// checked validates what both a new link and an edit can set.
func (r Relationship) checked() (Relationship, error) {
	if !slices.Contains(subtypes[r.Type], r.Subtype) {
		return Relationship{}, invalidRelationship("subtype must be one of " + strings.Join(subtypes[r.Type], ", "))
	}
	if r.Ended && r.Type != Couple {
		return Relationship{}, invalidRelationship("only a couple can be ended")
	}
	r.Notes = trimmed(r.Notes)
	if r.Notes != nil && !storable(*r.Notes) {
		return Relationship{}, invalidRelationship("the notes contain a character that isn't allowed")
	}
	if r.Notes != nil && utf8.RuneCountInString(*r.Notes) > maxNotesLength {
		return Relationship{}, invalidRelationship(fmt.Sprintf("the notes are longer than %d characters", maxNotesLength))
	}
	return r, nil
}

func (p RelationshipPatch) apply(r Relationship) Relationship {
	set(&r.Subtype, p.Subtype)
	set(&r.Ended, p.Ended)
	set(&r.Notes, p.Notes)
	return r
}

func invalidRelationship(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidRelationship, msg)
}
