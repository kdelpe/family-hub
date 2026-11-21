package family

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

// The relations an admin adds from a person's card (§7), each stored as parent links or a couple.
const (
	RelationParent  = "parent"
	RelationChild   = "child"
	RelationPartner = "partner"
	RelationSibling = "sibling"
)

var relations = []string{RelationParent, RelationChild, RelationPartner, RelationSibling}

// RelativeInput is someone to add beside a person: in the tree already (PersonID) or new (Person), not both. OtherParentID
// also links a child to a second parent; a sibling is linked through every family parent, or through SharedParentID only.
type RelativeInput struct {
	Relation       string
	PersonID       uuid.UUID
	Person         *PersonInput
	Subtype        *string
	Ended          bool
	OtherParentID  uuid.UUID
	SharedParentID uuid.UUID
}

// AddedRelative is the relative, new or existing, and the links that now join them to the person.
type AddedRelative struct {
	Person        Person
	Relationships []Relationship
}

// AddRelative is admin-only; the router checks the role. The new person and every link are saved together or
// not at all, so a refused link never leaves someone half added.
func (s *Service) AddRelative(ctx context.Context, createdBy, id uuid.UUID, in RelativeInput) (AddedRelative, error) {
	person, err := in.checked(id)
	if err != nil {
		return AddedRelative{}, err
	}
	var added AddedRelative
	err = s.writeRelationships(ctx, "add relative", func(q *gen.Queries) error {
		n, err := q.CountPersonsInTree(ctx, []uuid.UUID{id})
		if err != nil {
			return fmt.Errorf("check person: %w", err)
		}
		if n != 1 {
			return ErrPersonNotFound
		}
		relative := in.PersonID
		if person != nil {
			if relative, err = insertPerson(ctx, q, createdBy, *person); err != nil {
				return err
			}
		}
		links, err := linksFor(ctx, q, id, relative, in)
		if err != nil {
			return err
		}
		for _, link := range links {
			rel, err := newRelationship(link)
			if err != nil {
				return err
			}
			if rel, err = createLink(ctx, q, rel); err != nil {
				return err
			}
			added.Relationships = append(added.Relationships, rel)
		}
		added.Person, err = get(ctx, q, Viewer{IsAdmin: true}, relative)
		return err
	})
	if err != nil {
		return AddedRelative{}, err
	}
	return added, nil
}

// checked refuses what no stored data could make right, and returns the new person's fields, if there is one.
func (in RelativeInput) checked(id uuid.UUID) (*fields, error) {
	switch {
	case !slices.Contains(relations, in.Relation):
		return nil, invalidRelationship("relation must be one of " + strings.Join(relations, ", "))
	case (in.PersonID == uuid.Nil) == (in.Person == nil):
		return nil, invalidRelationship("give either person_id or a new person")
	case in.PersonID != uuid.Nil && in.PersonID == id:
		return nil, invalidRelationship("someone can't be their own relative")
	case in.Ended && in.Relation != RelationPartner:
		return nil, invalidRelationship("only a partner can be ended")
	case in.OtherParentID != uuid.Nil && in.Relation != RelationChild:
		return nil, invalidRelationship("other_parent_id is only for a child")
	case in.OtherParentID != uuid.Nil && in.OtherParentID == id:
		return nil, invalidRelationship("the other parent must be someone else")
	case in.SharedParentID != uuid.Nil && in.Relation != RelationSibling:
		return nil, invalidRelationship("shared_parent_id is only for a sibling")
	}
	if in.Person == nil {
		return nil, nil
	}
	f, err := normalize(*in.Person)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// linksFor is the relation as stored links, with relative standing in that relation to id.
func linksFor(ctx context.Context, q *gen.Queries, id, relative uuid.UUID, in RelativeInput) ([]RelationshipInput, error) {
	parentLink := func(parent, child uuid.UUID) RelationshipInput {
		return RelationshipInput{Type: ParentChild, ParentID: parent, ChildID: child, Subtype: in.Subtype}
	}
	switch in.Relation {
	case RelationParent:
		return []RelationshipInput{parentLink(relative, id)}, nil
	case RelationChild:
		links := []RelationshipInput{parentLink(id, relative)}
		if in.OtherParentID != uuid.Nil {
			links = append(links, parentLink(in.OtherParentID, relative))
		}
		return links, nil
	case RelationPartner:
		return []RelationshipInput{{Type: Couple, PersonIDs: []uuid.UUID{id, relative}, Subtype: in.Subtype, Ended: in.Ended}}, nil
	}
	// Siblings aren't stored: they share parents (§3.4).
	parents, err := q.FamilyParentsOf(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("family parents: %w", err)
	}
	if len(parents) == 0 {
		return nil, invalidRelationship("siblings are linked through their parents; add a parent first")
	}
	if in.SharedParentID != uuid.Nil {
		if !slices.Contains(parents, in.SharedParentID) {
			return nil, invalidRelationship("the shared parent must be one of their parents")
		}
		parents = []uuid.UUID{in.SharedParentID}
	}
	links := make([]RelationshipInput, 0, len(parents))
	for _, parent := range parents {
		links = append(links, parentLink(parent, relative))
	}
	return links, nil
}
