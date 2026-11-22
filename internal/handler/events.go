package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/event"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type EventService interface {
	List(ctx context.Context) ([]event.Event, error)
	Get(ctx context.Context, id uuid.UUID) (event.Event, error)
	Create(ctx context.Context, createdBy uuid.UUID, in event.Input) (event.Event, error)
	Update(ctx context.Context, id uuid.UUID, patch event.Patch) (event.Event, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Events serves the family timeline (design-notes §4.5).
type Events struct {
	svc EventService
}

func NewEvents(svc EventService) *Events {
	return &Events{svc: svc}
}

type eventResponse struct {
	ID            uuid.UUID             `json:"id"`
	Title         string                `json:"title"`
	Date          *string               `json:"date"`
	DatePrecision *string               `json:"date_precision"`
	Location      *string               `json:"location"`
	Description   *string               `json:"description"`
	People        []eventPersonResponse `json:"people"`
	CreatedAt     time.Time             `json:"created_at"`
}

type eventPersonResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  *string   `json:"last_name"`
}

type eventsResponse struct {
	Events []eventResponse `json:"events"`
}

type createEventRequest struct {
	Title         string      `json:"title"`
	Date          *string     `json:"date"`
	DatePrecision *string     `json:"date_precision"`
	Location      *string     `json:"location"`
	Description   *string     `json:"description"`
	PersonIDs     []uuid.UUID `json:"person_ids"`
}

type updateEventRequest struct {
	Title         required[string]      `json:"title"`
	Date          optional[*string]     `json:"date"`
	DatePrecision optional[*string]     `json:"date_precision"`
	Location      optional[*string]     `json:"location"`
	Description   optional[*string]     `json:"description"`
	PersonIDs     optional[[]uuid.UUID] `json:"person_ids"`
}

func (e *Events) List(w http.ResponseWriter, r *http.Request) {
	events, err := e.svc.List(r.Context())
	if err != nil {
		e.fail(w, r, err, "list events")
		return
	}
	resp := eventsResponse{Events: make([]eventResponse, 0, len(events))}
	for _, ev := range events {
		resp.Events = append(resp.Events, toEventResponse(ev))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (e *Events) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	ev, err := e.svc.Get(r.Context(), id)
	e.answer(w, r, http.StatusOK, ev, err, "get event")
}

func (e *Events) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "create event mounted without RequireSession")
		return
	}
	var req createEventRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ev, err := e.svc.Create(r.Context(), session.UserID, event.Input(req))
	e.answer(w, r, http.StatusCreated, ev, err, "create event")
}

func (e *Events) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	var req updateEventRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	patch := event.Patch{
		Title: family.Opt[string](req.Title), Date: family.Opt[*string](req.Date), DatePrecision: family.Opt[*string](req.DatePrecision),
		Location: family.Opt[*string](req.Location), Description: family.Opt[*string](req.Description),
		PersonIDs: family.Opt[[]uuid.UUID](req.PersonIDs),
	}
	ev, err := e.svc.Update(r.Context(), id, patch)
	e.answer(w, r, http.StatusOK, ev, err, "update event")
}

func (e *Events) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	if err := e.svc.Delete(r.Context(), id); err != nil {
		e.fail(w, r, err, "delete event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (e *Events) answer(w http.ResponseWriter, r *http.Request, status int, ev event.Event, err error, what string) {
	if err != nil {
		e.fail(w, r, err, what)
		return
	}
	httpx.JSON(w, status, toEventResponse(ev))
}

func (e *Events) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	switch {
	case clientGone(r):
	case errors.Is(err, event.ErrInvalidEvent):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, event.ErrEventNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, what)
	}
}

func eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return pathID(w, r, "id", event.ErrEventNotFound)
}

func toEventResponse(ev event.Event) eventResponse {
	people := make([]eventPersonResponse, 0, len(ev.People))
	for _, p := range ev.People {
		people = append(people, eventPersonResponse(p))
	}
	return eventResponse{
		ID: ev.ID, Title: ev.Title, Date: ev.Date, DatePrecision: ev.DatePrecision, Location: ev.Location,
		Description: ev.Description, People: people, CreatedAt: ev.CreatedAt,
	}
}
