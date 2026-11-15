package family

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	DefaultDepth = 5
	// A display bound, not a safety one: the walks keep one row per person and depth.
	maxDepth = 10
)

// Tree is everyone in it and every link between them, for the client to lay out (§10).
type Tree struct {
	Persons       []TreePerson
	Relationships []TreeLink
}

// TreePerson carries no dates and no contacts: the tree shows neither.
type TreePerson struct {
	ID         uuid.UUID
	FirstName  string
	LastName   *string
	Side       *string
	Gender     *string
	IsDeceased bool
	BioExcerpt *string
}

// TreeLink is a Relationship without the admins' notes.
type TreeLink struct {
	ID      uuid.UUID
	Type    string
	Subtype string
	Ended   bool
	PersonA uuid.UUID
	PersonB uuid.UUID
}

// Relative is an ancestor or descendant, Depth generations away. BirthDate is masked as on Person.
type Relative struct {
	ID                 uuid.UUID
	FirstName          string
	LastName           *string
	IsDeceased         bool
	BirthDate          *string
	BirthDatePrecision *string
	Depth              int
}

// Summary is the People page's family strip.
type Summary struct {
	People      int
	Generations int
	// EarliestRecord is the earliest-born deceased person's birth; nil when there is none.
	EarliestRecord *EarliestRecord
}

type EarliestRecord struct {
	Year  int
	Circa bool
	Place *string
}

func (s *Service) Tree(ctx context.Context) (Tree, error) {
	var tree Tree
	err := s.read(ctx, "tree", func(q *gen.Queries) error {
		people, err := q.TreePersons(ctx)
		if err != nil {
			return fmt.Errorf("tree persons: %w", err)
		}
		links, err := q.TreeRelationships(ctx)
		if err != nil {
			return fmt.Errorf("tree relationships: %w", err)
		}
		tree.Persons = make([]TreePerson, 0, len(people))
		for _, p := range people {
			person := TreePerson{ID: p.ID, FirstName: p.FirstName, LastName: p.LastName, Side: p.Side, Gender: p.Gender, IsDeceased: p.IsDeceased}
			if p.BioExcerpt != "" {
				person.BioExcerpt = &p.BioExcerpt
			}
			tree.Persons = append(tree.Persons, person)
		}
		tree.Relationships = make([]TreeLink, 0, len(links))
		for _, l := range links {
			tree.Relationships = append(tree.Relationships, TreeLink(l))
		}
		return nil
	})
	if err != nil {
		return Tree{}, err
	}
	return tree, nil
}

// Ancestors walks up to depth generations, closest first.
func (s *Service) Ancestors(ctx context.Context, viewer Viewer, id uuid.UUID, depth int) ([]Relative, error) {
	return s.relatives(ctx, "ancestors", id, depth, func(q *gen.Queries, limit int32) ([]Relative, error) {
		rows, err := q.Ancestors(ctx, gen.AncestorsParams{ViewerIsAdmin: viewer.IsAdmin, ViewerPersonID: viewer.PersonID, PersonID: id, MaxDepth: limit})
		return toRelatives(rows), err
	})
}

// Descendants walks down to depth generations, closest first.
func (s *Service) Descendants(ctx context.Context, viewer Viewer, id uuid.UUID, depth int) ([]Relative, error) {
	return s.relatives(ctx, "descendants", id, depth, func(q *gen.Queries, limit int32) ([]Relative, error) {
		rows, err := q.Descendants(ctx, gen.DescendantsParams{ViewerIsAdmin: viewer.IsAdmin, ViewerPersonID: viewer.PersonID, PersonID: id, MaxDepth: limit})
		return toRelatives(rows), err
	})
}

func (s *Service) relatives(ctx context.Context, what string, id uuid.UUID, depth int,
	walk func(q *gen.Queries, limit int32) ([]Relative, error)) ([]Relative, error) {
	if depth < 1 || depth > maxDepth {
		return nil, fmt.Errorf("%w: depth must be a whole number from 1 to %d", ErrInvalidFilter, maxDepth)
	}
	var relatives []Relative
	err := s.read(ctx, what, func(q *gen.Queries) error {
		n, err := q.CountPersonsInTree(ctx, []uuid.UUID{id})
		if err != nil {
			return fmt.Errorf("check person: %w", err)
		}
		if n == 0 {
			return ErrPersonNotFound
		}
		if relatives, err = walk(q, int32(depth)); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return relatives, nil
}

// The two walks return the same columns, so both row types convert to AncestorsRow.
func toRelatives[R gen.AncestorsRow | gen.DescendantsRow](rows []R) []Relative {
	relatives := make([]Relative, 0, len(rows))
	for _, row := range rows {
		r := gen.AncestorsRow(row)
		rel := Relative{ID: r.ID, FirstName: r.FirstName, LastName: r.LastName, IsDeceased: r.IsDeceased, Depth: int(r.Depth)}
		// The precision only goes out with a date the viewer may see.
		if r.BirthDate != "" {
			rel.BirthDate, rel.BirthDatePrecision = &r.BirthDate, r.BirthDatePrecision
		}
		relatives = append(relatives, rel)
	}
	return relatives
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	var sum Summary
	err := s.read(ctx, "family summary", func(q *gen.Queries) error {
		people, err := q.CountPersons(ctx)
		if err != nil {
			return fmt.Errorf("count persons: %w", err)
		}
		sum.People = int(people)
		links, err := q.FamilyParentLinks(ctx)
		if err != nil {
			return fmt.Errorf("family parent links: %w", err)
		}
		if people > 0 {
			sum.Generations = longestLine(links)
		}
		row, err := q.EarliestRecord(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("earliest record: %w", err)
		}
		sum.EarliestRecord = &EarliestRecord{Year: row.BirthDate.Year(), Circa: *row.BirthDatePrecision == "circa", Place: row.BirthPlace}
		return nil
	})
	if err != nil {
		return Summary{}, err
	}
	return sum, nil
}

// longestLine counts the people in the longest parent-to-child line; someone without links is a line of one.
// A person met again mid-walk counts once, so a loop in damaged data can't make it spin.
func longestLine(links []gen.FamilyParentLinksRow) int {
	children := map[uuid.UUID][]uuid.UUID{}
	for _, l := range links {
		children[l.ParentID] = append(children[l.ParentID], l.ChildID)
	}
	lengths := map[uuid.UUID]int{}
	var walk func(id uuid.UUID) int
	walk = func(id uuid.UUID) int {
		if n, ok := lengths[id]; ok {
			return n
		}
		lengths[id] = 1
		longest := 0
		for _, child := range children[id] {
			longest = max(longest, walk(child))
		}
		lengths[id] = 1 + longest
		return lengths[id]
	}
	longest := 1
	for parent := range children {
		longest = max(longest, walk(parent))
	}
	return longest
}
