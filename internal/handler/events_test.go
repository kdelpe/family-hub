package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/event"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var (
	testEventID = uuid.MustParse("01890000-0000-7000-8000-0000000000ee")
	testEvent   = event.Event{
		ID: testEventID, Title: "Wedding in Jacmel", Date: new("1962-06-02"), DatePrecision: new("day"), Location: new("Jacmel"),
		People:    []event.Person{{ID: testPersonID, FirstName: "Rosa", LastName: new("Pierre")}},
		CreatedAt: time.Date(2026, 11, 21, 15, 4, 5, 0, time.UTC),
	}
	testEventBody = `{"id":"` + testEventID.String() + `","title":"Wedding in Jacmel","date":"1962-06-02","date_precision":"day",
		"location":"Jacmel","description":null,"people":[{"id":"` + testPersonID.String() + `","first_name":"Rosa","last_name":"Pierre"}],
		"created_at":"2026-11-21T15:04:05Z"}`
	eventNotFoundBody = `{"error":{"code":"not_found","message":"event not found"}}`
)

func TestListEvents(t *testing.T) {
	tests := []struct {
		name       string
		events     []event.Event
		listErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "the timeline", events: []event.Event{testEvent}, wantStatus: http.StatusOK, wantBody: `{"events":[` + testEventBody + `]}`},
		{name: "nothing yet", wantStatus: http.StatusOK, wantBody: `{"events":[]}`},
		{name: "an event nobody is tagged in", events: []event.Event{{ID: testEventID, Title: "Moved", CreatedAt: testEvent.CreatedAt}}, wantStatus: http.StatusOK,
			wantBody: `{"events":[{"id":"` + testEventID.String() + `","title":"Moved","date":null,"date_precision":null,"location":null,
				"description":null,"people":[],"created_at":"2026-11-21T15:04:05Z"}]}`},
		{name: "database error", listErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewEventService(t)
			svc.EXPECT().List(mock.Anything).Return(tt.events, tt.listErr)

			rec := serveAs(t, auth.RoleMember, handler.NewEvents(svc).List, httptest.NewRequest(http.MethodGet, "/api/events", nil))

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestGetEvent(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		getErr     error
		wantGet    bool
		wantStatus int
		wantBody   string
	}{
		{name: "found", wantGet: true, wantStatus: http.StatusOK, wantBody: testEventBody},
		{name: "unknown or deleted", wantGet: true, getErr: event.ErrEventNotFound, wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
		{name: "not an event id", id: "42", wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewEventService(t)
			if tt.wantGet {
				svc.EXPECT().Get(mock.Anything, testEventID).Return(testEvent, tt.getErr)
			}
			id := eventIDOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/events/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleMember, handler.NewEvents(svc).Get, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestCreateEvent(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantInput  *event.Input
		createErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "created", body: `{"title":"Wedding in Jacmel","date":"1962-06-02","date_precision":"day","location":"Jacmel","person_ids":["` + testPersonID.String() + `"]}`,
			wantInput:  &event.Input{Title: "Wedding in Jacmel", Date: new("1962-06-02"), DatePrecision: new("day"), Location: new("Jacmel"), PersonIDs: []uuid.UUID{testPersonID}},
			wantStatus: http.StatusCreated, wantBody: testEventBody},
		{name: "refused by the service", body: `{"title":""}`, wantInput: &event.Input{}, createErr: errors.Join(event.ErrInvalidEvent, errors.New("a title is required")),
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid event\na title is required"}}`},
		{name: "busy", body: `{"title":"Wedding"}`, wantInput: &event.Input{Title: "Wedding"}, createErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "a person id that isn't one", body: `{"title":"Wedding","person_ids":["42"]}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "unknown field", body: `{"title":"Wedding","media":[]}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "database error", body: `{"title":"Wedding"}`, wantInput: &event.Input{Title: "Wedding"}, createErr: errors.New("db down"),
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewEventService(t)
			if tt.wantInput != nil {
				svc.EXPECT().Create(mock.Anything, testUser.ID, *tt.wantInput).Return(testEvent, tt.createErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := serveAs(t, auth.RoleAdmin, handler.NewEvents(svc).Create, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestUpdateEvent(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		body       string
		wantPatch  *event.Patch
		updateErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "only the fields sent change; null clears", body: `{"title":"Rosa's wedding","location":null}`,
			wantPatch:  &event.Patch{Title: family.Opt[string]{Set: true, Value: "Rosa's wedding"}, Location: family.Opt[*string]{Set: true}},
			wantStatus: http.StatusOK, wantBody: testEventBody},
		{name: "who was there is replaced", body: `{"person_ids":["` + otherPersonID.String() + `"]}`,
			wantPatch: &event.Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true, Value: []uuid.UUID{otherPersonID}}}, wantStatus: http.StatusOK, wantBody: testEventBody},
		{name: "null tags nobody", body: `{"person_ids":null}`, wantPatch: &event.Patch{PersonIDs: family.Opt[[]uuid.UUID]{Set: true}},
			wantStatus: http.StatusOK, wantBody: testEventBody},
		{name: "a title can't be cleared", body: `{"title":null}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "unknown or deleted", body: `{}`, wantPatch: &event.Patch{}, updateErr: event.ErrEventNotFound, wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
		{name: "not an event id", id: "42", body: `{}`, wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
		{name: "busy", body: `{}`, wantPatch: &event.Patch{}, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewEventService(t)
			if tt.wantPatch != nil {
				svc.EXPECT().Update(mock.Anything, testEventID, *tt.wantPatch).Return(testEvent, tt.updateErr)
			}
			id := eventIDOr(tt.id)
			req := httptest.NewRequest(http.MethodPatch, "/api/events/"+id, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParam(req, "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewEvents(svc).Update, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestDeleteEvent(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteErr  error
		wantDelete bool
		wantStatus int
		wantBody   string
	}{
		{name: "deleted", wantDelete: true, wantStatus: http.StatusNoContent},
		{name: "unknown or already deleted", wantDelete: true, deleteErr: event.ErrEventNotFound, wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
		{name: "not an event id", id: "42", wantStatus: http.StatusNotFound, wantBody: eventNotFoundBody},
		{name: "database error", wantDelete: true, deleteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewEventService(t)
			if tt.wantDelete {
				svc.EXPECT().Delete(mock.Anything, testEventID).Return(tt.deleteErr)
			}
			id := eventIDOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/events/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewEvents(svc).Delete, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func eventIDOr(id string) string {
	if id != "" {
		return id
	}
	return testEventID.String()
}
