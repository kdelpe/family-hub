package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type FamilyService interface {
	ListPersons(ctx context.Context, viewer family.Viewer, in family.ListInput) ([]family.Person, error)
	GetPerson(ctx context.Context, viewer family.Viewer, id uuid.UUID) (family.Person, error)
	CreatePerson(ctx context.Context, createdBy uuid.UUID, in family.PersonInput) (family.Person, error)
	UpdatePerson(ctx context.Context, id uuid.UUID, patch family.PersonPatch) (family.Person, error)
	DeletePerson(ctx context.Context, id uuid.UUID) error
	RestorePerson(ctx context.Context, id uuid.UUID) (family.Person, error)
	CreateRelationship(ctx context.Context, in family.RelationshipInput) (family.Relationship, error)
	UpdateRelationship(ctx context.Context, id uuid.UUID, patch family.RelationshipPatch) (family.Relationship, error)
	DeleteRelationship(ctx context.Context, id uuid.UUID) error
	Tree(ctx context.Context) (family.Tree, error)
	Ancestors(ctx context.Context, viewer family.Viewer, id uuid.UUID, depth int) ([]family.Relative, error)
	Descendants(ctx context.Context, viewer family.Viewer, id uuid.UUID, depth int) ([]family.Relative, error)
	Summary(ctx context.Context) (family.Summary, error)
}

type Family struct {
	svc FamilyService
}

func NewFamily(svc FamilyService) *Family {
	return &Family{svc: svc}
}

type personResponse struct {
	ID                 uuid.UUID `json:"id"`
	FirstName          string    `json:"first_name"`
	MiddleName         *string   `json:"middle_name"`
	LastName           *string   `json:"last_name"`
	MaidenName         *string   `json:"maiden_name"`
	Side               *string   `json:"side"`
	BirthPlace         *string   `json:"birth_place"`
	Gender             *string   `json:"gender"`
	BirthDate          *string   `json:"birth_date"`
	BirthDatePrecision *string   `json:"birth_date_precision"`
	DeathDate          *string   `json:"death_date"`
	DeathDatePrecision *string   `json:"death_date_precision"`
	IsDeceased         bool      `json:"is_deceased"`
	Bio                *string   `json:"bio"`
}

type personsResponse struct {
	Persons []personResponse `json:"persons"`
}

type createPersonRequest struct {
	FirstName          string  `json:"first_name"`
	MiddleName         *string `json:"middle_name"`
	LastName           *string `json:"last_name"`
	MaidenName         *string `json:"maiden_name"`
	Side               *string `json:"side"`
	BirthPlace         *string `json:"birth_place"`
	Gender             *string `json:"gender"`
	BirthDate          *string `json:"birth_date"`
	BirthDatePrecision *string `json:"birth_date_precision"`
	DeathDate          *string `json:"death_date"`
	DeathDatePrecision *string `json:"death_date_precision"`
	IsDeceased         bool    `json:"is_deceased"`
	Bio                *string `json:"bio"`
}

type updatePersonRequest struct {
	FirstName          required[string]  `json:"first_name"`
	MiddleName         optional[*string] `json:"middle_name"`
	LastName           optional[*string] `json:"last_name"`
	MaidenName         optional[*string] `json:"maiden_name"`
	Side               optional[*string] `json:"side"`
	BirthPlace         optional[*string] `json:"birth_place"`
	Gender             optional[*string] `json:"gender"`
	BirthDate          optional[*string] `json:"birth_date"`
	BirthDatePrecision optional[*string] `json:"birth_date_precision"`
	DeathDate          optional[*string] `json:"death_date"`
	DeathDatePrecision optional[*string] `json:"death_date_precision"`
	IsDeceased         required[bool]    `json:"is_deceased"`
	Bio                optional[*string] `json:"bio"`
}

// optional tells a field left out of a PATCH (unchanged) from one sent as null (cleared).
type optional[T any] family.Opt[T]

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

// required is a PATCH field that can be left out but not cleared: null would otherwise decode as the zero value.
type required[T any] family.Opt[T]

func (o *required[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return errors.New("null for a field that can't be cleared")
	}
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

// relationshipResponse has parent_id and child_id for a parent link, person_ids for a couple, as the request does.
type relationshipResponse struct {
	ID        uuid.UUID   `json:"id"`
	Type      string      `json:"type"`
	Subtype   string      `json:"subtype"`
	Ended     bool        `json:"ended"`
	ParentID  uuid.UUID   `json:"parent_id,omitzero"`
	ChildID   uuid.UUID   `json:"child_id,omitzero"`
	PersonIDs []uuid.UUID `json:"person_ids,omitempty"`
	Notes     *string     `json:"notes"`
}

type createRelationshipRequest struct {
	Type      string      `json:"type"`
	ParentID  uuid.UUID   `json:"parent_id"`
	ChildID   uuid.UUID   `json:"child_id"`
	PersonIDs []uuid.UUID `json:"person_ids"`
	Subtype   *string     `json:"subtype"`
	Ended     bool        `json:"ended"`
	Notes     *string     `json:"notes"`
}

// The type and people of a link never change, so a PATCH naming them is refused as an unknown field.
type updateRelationshipRequest struct {
	Subtype required[string]  `json:"subtype"`
	Ended   required[bool]    `json:"ended"`
	Notes   optional[*string] `json:"notes"`
}

type treeResponse struct {
	Persons       []treePersonResponse `json:"persons"`
	Relationships []treeLinkResponse   `json:"relationships"`
}

type treePersonResponse struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	LastName   *string   `json:"last_name"`
	Side       *string   `json:"side"`
	Gender     *string   `json:"gender"`
	IsDeceased bool      `json:"is_deceased"`
	BioExcerpt *string   `json:"bio_excerpt"`
}

// treeLinkResponse is a relationshipResponse without the notes.
type treeLinkResponse struct {
	ID        uuid.UUID   `json:"id"`
	Type      string      `json:"type"`
	Subtype   string      `json:"subtype"`
	Ended     bool        `json:"ended"`
	ParentID  uuid.UUID   `json:"parent_id,omitzero"`
	ChildID   uuid.UUID   `json:"child_id,omitzero"`
	PersonIDs []uuid.UUID `json:"person_ids,omitempty"`
}

type relativesResponse struct {
	Persons []relativeResponse `json:"persons"`
}

type relativeResponse struct {
	ID                 uuid.UUID `json:"id"`
	FirstName          string    `json:"first_name"`
	LastName           *string   `json:"last_name"`
	IsDeceased         bool      `json:"is_deceased"`
	BirthDate          *string   `json:"birth_date"`
	BirthDatePrecision *string   `json:"birth_date_precision"`
	Depth              int       `json:"depth"`
}

type summaryResponse struct {
	People         int                     `json:"people"`
	Generations    int                     `json:"generations"`
	EarliestRecord *earliestRecordResponse `json:"earliest_record"`
}

type earliestRecordResponse struct {
	Year  int     `json:"year"`
	Circa bool    `json:"circa"`
	Place *string `json:"place"`
}

func (f *Family) SearchPersons(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	in := family.ListInput{Query: query.Get("q"), Side: query.Get("side")}
	switch query.Get("living") {
	case "":
	case "true":
		in.Deceased = new(false)
	case "false":
		in.Deceased = new(true)
	default:
		httpx.Error(w, http.StatusBadRequest, "validation_error", "living must be true or false")
		return
	}
	people, err := f.svc.ListPersons(r.Context(), viewer, in)
	if err != nil {
		f.fail(w, r, err, "list persons")
		return
	}
	resp := personsResponse{Persons: make([]personResponse, 0, len(people))}
	for _, p := range people {
		resp.Persons = append(resp.Persons, toPersonResponse(p))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (f *Family) GetPerson(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	id, ok := personID(w, r)
	if !ok {
		return
	}
	person, err := f.svc.GetPerson(r.Context(), viewer, id)
	f.answer(w, r, http.StatusOK, person, err, "get person")
}

func (f *Family) CreatePerson(w http.ResponseWriter, r *http.Request) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "create person mounted without RequireSession")
		return
	}
	var req createPersonRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	person, err := f.svc.CreatePerson(r.Context(), session.UserID, family.PersonInput(req))
	f.answer(w, r, http.StatusCreated, person, err, "create person")
}

func (f *Family) UpdatePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := personID(w, r)
	if !ok {
		return
	}
	var req updatePersonRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	patch := family.PersonPatch{
		FirstName: family.Opt[string](req.FirstName), MiddleName: family.Opt[*string](req.MiddleName),
		LastName: family.Opt[*string](req.LastName), MaidenName: family.Opt[*string](req.MaidenName),
		Side: family.Opt[*string](req.Side), BirthPlace: family.Opt[*string](req.BirthPlace), Gender: family.Opt[*string](req.Gender),
		BirthDate: family.Opt[*string](req.BirthDate), BirthDatePrecision: family.Opt[*string](req.BirthDatePrecision),
		DeathDate: family.Opt[*string](req.DeathDate), DeathDatePrecision: family.Opt[*string](req.DeathDatePrecision),
		IsDeceased: family.Opt[bool](req.IsDeceased), Bio: family.Opt[*string](req.Bio),
	}
	person, err := f.svc.UpdatePerson(r.Context(), id, patch)
	f.answer(w, r, http.StatusOK, person, err, "update person")
}

func (f *Family) DeletePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := personID(w, r)
	if !ok {
		return
	}
	if err := f.svc.DeletePerson(r.Context(), id); err != nil {
		f.fail(w, r, err, "delete person")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (f *Family) RestorePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := personID(w, r)
	if !ok {
		return
	}
	person, err := f.svc.RestorePerson(r.Context(), id)
	f.answer(w, r, http.StatusOK, person, err, "restore person")
}

func (f *Family) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	var req createRelationshipRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	rel, err := f.svc.CreateRelationship(r.Context(), family.RelationshipInput(req))
	if err != nil {
		f.fail(w, r, err, "create relationship")
		return
	}
	httpx.JSON(w, http.StatusCreated, toRelationshipResponse(rel))
}

func (f *Family) UpdateRelationship(w http.ResponseWriter, r *http.Request) {
	id, ok := relationshipID(w, r)
	if !ok {
		return
	}
	var req updateRelationshipRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	patch := family.RelationshipPatch{
		Subtype: family.Opt[string](req.Subtype), Ended: family.Opt[bool](req.Ended), Notes: family.Opt[*string](req.Notes),
	}
	rel, err := f.svc.UpdateRelationship(r.Context(), id, patch)
	if err != nil {
		f.fail(w, r, err, "update relationship")
		return
	}
	httpx.JSON(w, http.StatusOK, toRelationshipResponse(rel))
}

func (f *Family) DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	id, ok := relationshipID(w, r)
	if !ok {
		return
	}
	if err := f.svc.DeleteRelationship(r.Context(), id); err != nil {
		f.fail(w, r, err, "delete relationship")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (f *Family) Tree(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentViewer(w, r); !ok {
		return
	}
	tree, err := f.svc.Tree(r.Context())
	if err != nil {
		f.fail(w, r, err, "tree")
		return
	}
	resp := treeResponse{
		Persons:       make([]treePersonResponse, 0, len(tree.Persons)),
		Relationships: make([]treeLinkResponse, 0, len(tree.Relationships)),
	}
	for _, p := range tree.Persons {
		resp.Persons = append(resp.Persons, treePersonResponse(p))
	}
	for _, l := range tree.Relationships {
		link := treeLinkResponse{ID: l.ID, Type: l.Type, Subtype: l.Subtype, Ended: l.Ended}
		if l.Type == family.ParentChild {
			link.ParentID, link.ChildID = l.PersonA, l.PersonB
		} else {
			link.PersonIDs = []uuid.UUID{l.PersonA, l.PersonB}
		}
		resp.Relationships = append(resp.Relationships, link)
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (f *Family) Ancestors(w http.ResponseWriter, r *http.Request) {
	f.relatives(w, r, "ancestors", f.svc.Ancestors)
}

func (f *Family) Descendants(w http.ResponseWriter, r *http.Request) {
	f.relatives(w, r, "descendants", f.svc.Descendants)
}

func (f *Family) relatives(w http.ResponseWriter, r *http.Request, what string,
	walk func(ctx context.Context, viewer family.Viewer, id uuid.UUID, depth int) ([]family.Relative, error)) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	id, ok := personID(w, r)
	if !ok {
		return
	}
	depth := family.DefaultDepth
	if raw := r.URL.Query().Get("depth"); raw != "" {
		// Anything but a number parses as 0, which the service refuses like any bad depth.
		depth, _ = strconv.Atoi(raw)
	}
	relatives, err := walk(r.Context(), viewer, id, depth)
	if err != nil {
		f.fail(w, r, err, what)
		return
	}
	resp := relativesResponse{Persons: make([]relativeResponse, 0, len(relatives))}
	for _, rel := range relatives {
		resp.Persons = append(resp.Persons, relativeResponse(rel))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (f *Family) Summary(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentViewer(w, r); !ok {
		return
	}
	sum, err := f.svc.Summary(r.Context())
	if err != nil {
		f.fail(w, r, err, "family summary")
		return
	}
	httpx.JSON(w, http.StatusOK, summaryResponse{
		People: sum.People, Generations: sum.Generations, EarliestRecord: (*earliestRecordResponse)(sum.EarliestRecord),
	})
}

func (f *Family) answer(w http.ResponseWriter, r *http.Request, status int, person family.Person, err error, what string) {
	if err != nil {
		f.fail(w, r, err, what)
		return
	}
	httpx.JSON(w, status, toPersonResponse(person))
}

func (f *Family) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	switch {
	case clientGone(r):
	case errors.Is(err, family.ErrInvalidPerson), errors.Is(err, family.ErrInvalidFilter), errors.Is(err, family.ErrInvalidRelationship):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, family.ErrPersonNotFound), errors.Is(err, family.ErrRelationshipNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, family.ErrRelationshipConflict), errors.Is(err, family.ErrPersonHasLogin):
		httpx.Error(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, what)
	}
}

func currentViewer(w http.ResponseWriter, r *http.Request) (family.Viewer, bool) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "person read mounted without RequireSession")
		return family.Viewer{}, false
	}
	return family.Viewer{IsAdmin: session.Role == auth.RoleAdmin, PersonID: session.PersonID}, true
}

func personID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return pathID(w, r, family.ErrPersonNotFound)
}

func relationshipID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return pathID(w, r, family.ErrRelationshipNotFound)
}

// pathID answers 404 for an id that isn't a UUID: no such thing can exist.
func pathID(w http.ResponseWriter, r *http.Request, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", notFound.Error())
		return uuid.Nil, false
	}
	return id, true
}

func toPersonResponse(p family.Person) personResponse {
	return personResponse(p)
}

func toRelationshipResponse(rel family.Relationship) relationshipResponse {
	resp := relationshipResponse{ID: rel.ID, Type: rel.Type, Subtype: rel.Subtype, Ended: rel.Ended, Notes: rel.Notes}
	if rel.Type == family.ParentChild {
		resp.ParentID, resp.ChildID = rel.PersonA, rel.PersonB
	} else {
		resp.PersonIDs = []uuid.UUID{rel.PersonA, rel.PersonB}
	}
	return resp
}
