package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type FamilyService interface {
	List(ctx context.Context, viewer family.Viewer, in family.ListInput) ([]family.Person, error)
	Get(ctx context.Context, viewer family.Viewer, id uuid.UUID) (family.Person, error)
	Create(ctx context.Context, createdBy uuid.UUID, in family.PersonInput) (family.Person, error)
	Update(ctx context.Context, id uuid.UUID, patch family.PersonPatch) (family.Person, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) (family.Person, error)
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
	FirstName          optional[string]  `json:"first_name"`
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
	IsDeceased         optional[bool]    `json:"is_deceased"`
	Bio                optional[*string] `json:"bio"`
}

// optional tells a field left out of a PATCH (unchanged) from one sent as null (cleared).
type optional[T any] family.Opt[T]

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
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
	people, err := f.svc.List(r.Context(), viewer, in)
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
	person, err := f.svc.Get(r.Context(), viewer, id)
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
	person, err := f.svc.Create(r.Context(), session.UserID, family.PersonInput(req))
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
	person, err := f.svc.Update(r.Context(), id, patch)
	f.answer(w, r, http.StatusOK, person, err, "update person")
}

func (f *Family) DeletePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := personID(w, r)
	if !ok {
		return
	}
	if err := f.svc.Delete(r.Context(), id); err != nil {
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
	person, err := f.svc.Restore(r.Context(), id)
	f.answer(w, r, http.StatusOK, person, err, "restore person")
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
	case errors.Is(err, family.ErrInvalidPerson), errors.Is(err, family.ErrInvalidFilter):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, family.ErrPersonNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, what)
	}
}

// Linking a login to a person arrives in Phase 3 (§5.1); until then nobody is "self".
func currentViewer(w http.ResponseWriter, r *http.Request) (family.Viewer, bool) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "person read mounted without RequireSession")
		return family.Viewer{}, false
	}
	return family.Viewer{IsAdmin: session.Role == auth.RoleAdmin}, true
}

func personID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", family.ErrPersonNotFound.Error())
		return uuid.Nil, false
	}
	return id, true
}

func toPersonResponse(p family.Person) personResponse {
	return personResponse(p)
}
