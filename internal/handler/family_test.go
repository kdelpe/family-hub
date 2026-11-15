package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var (
	testPersonID = uuid.MustParse("01890000-0000-7000-8000-0000000000aa")
	testPerson   = family.Person{
		ID: testPersonID, FirstName: "Rosa", LastName: new("Pierre"), Side: new("first"),
		BirthDate: new("--03-14"), BirthDatePrecision: new("day"),
	}
)

const (
	testPersonBody = `{"id":"01890000-0000-7000-8000-0000000000aa","first_name":"Rosa","middle_name":null,"last_name":"Pierre",
		"maiden_name":null,"side":"first","birth_place":null,"gender":null,"birth_date":"--03-14","birth_date_precision":"day",
		"death_date":null,"death_date_precision":null,"is_deceased":false,"bio":null}`
	personNotFoundBody = `{"error":{"code":"not_found","message":"person not found"}}`
)

func TestSearchPersons(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		query      string
		wantIn     *family.ListInput
		people     []family.Person
		listErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "everyone, as a member", role: auth.RoleMember, wantIn: &family.ListInput{}, people: []family.Person{testPerson},
			wantStatus: http.StatusOK, wantBody: `{"persons":[` + testPersonBody + `]}`},
		{name: "nobody yet", role: auth.RoleAdmin, wantIn: &family.ListInput{}, wantStatus: http.StatusOK, wantBody: `{"persons":[]}`},
		{name: "search and filters", role: auth.RoleMember, query: "?q=ros&side=first&living=true",
			wantIn: &family.ListInput{Query: "ros", Side: "first", Deceased: new(false)}, wantStatus: http.StatusOK, wantBody: `{"persons":[]}`},
		{name: "the departed", role: auth.RoleMember, query: "?living=false", wantIn: &family.ListInput{Deceased: new(true)}, wantStatus: http.StatusOK, wantBody: `{"persons":[]}`},
		{name: "living not a boolean", role: auth.RoleMember, query: "?living=maybe", wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"living must be true or false"}}`},
		{name: "bad filter", role: auth.RoleMember, query: "?side=north", wantIn: &family.ListInput{Side: "north"}, listErr: family.ErrInvalidFilter,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid filter"}}`},
		{name: "database error", role: auth.RoleMember, wantIn: &family.ListInput{}, listErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantIn != nil {
				svc.EXPECT().List(mock.Anything, family.Viewer{IsAdmin: tt.role == auth.RoleAdmin}, *tt.wantIn).Return(tt.people, tt.listErr)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/persons"+tt.query, nil)

			rec := serveAs(t, tt.role, handler.NewFamily(svc).SearchPersons, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestGetPerson(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		id         string
		getErr     error
		wantGet    bool
		wantStatus int
		wantBody   string
	}{
		{name: "found, as a member", role: auth.RoleMember, wantGet: true, wantStatus: http.StatusOK, wantBody: testPersonBody},
		{name: "found, as an admin", role: auth.RoleAdmin, wantGet: true, wantStatus: http.StatusOK, wantBody: testPersonBody},
		{name: "unknown or deleted", role: auth.RoleMember, wantGet: true, getErr: family.ErrPersonNotFound, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "not a person id", role: auth.RoleMember, id: "42", wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", role: auth.RoleMember, wantGet: true, getErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantGet {
				svc.EXPECT().Get(mock.Anything, family.Viewer{IsAdmin: tt.role == auth.RoleAdmin}, testPersonID).Return(testPerson, tt.getErr)
			}
			id := idOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/persons/"+id, nil), "id", id)

			rec := serveAs(t, tt.role, handler.NewFamily(svc).GetPerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestCreatePerson(t *testing.T) {
	const body = `{"first_name":"Rosa","last_name":"Pierre","birth_date":"1950","birth_date_precision":"year","is_deceased":true}`
	want := family.PersonInput{FirstName: "Rosa", LastName: new("Pierre"), BirthDate: new("1950"), BirthDatePrecision: new("year"), IsDeceased: true}

	tests := []struct {
		name       string
		session    bool
		body       string
		createErr  error
		wantCreate bool
		wantStatus int
		wantBody   string
	}{
		{name: "created", session: true, body: body, wantCreate: true, wantStatus: http.StatusCreated, wantBody: testPersonBody},
		{name: "refused by the rules", session: true, body: body, wantCreate: true, createErr: family.ErrInvalidPerson,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid person"}}`},
		{name: "unknown field", session: true, body: `{"first_name":"Rosa","email":"x@example.com"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "database error", session: true, body: body, wantCreate: true, createErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", body: body, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantCreate {
				svc.EXPECT().Create(mock.Anything, testUser.ID, want).Return(testPerson, tt.createErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/persons", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			role := ""
			if tt.session {
				role = auth.RoleAdmin
			}

			rec := serveAs(t, role, handler.NewFamily(svc).CreatePerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestUpdatePerson(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		body       string
		wantPatch  *family.PersonPatch
		updateErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "only the fields sent change; null clears", body: `{"bio":"Tells the best stories.","middle_name":null}`,
			wantPatch: &family.PersonPatch{
				Bio:        family.Opt[*string]{Set: true, Value: new("Tells the best stories.")},
				MiddleName: family.Opt[*string]{Set: true},
			}, wantStatus: http.StatusOK, wantBody: testPersonBody},
		{name: "a first name and deceased flag", body: `{"first_name":"Rose","is_deceased":true}`,
			wantPatch: &family.PersonPatch{
				FirstName:  family.Opt[string]{Set: true, Value: "Rose"},
				IsDeceased: family.Opt[bool]{Set: true, Value: true},
			}, wantStatus: http.StatusOK, wantBody: testPersonBody},
		{name: "refused by the rules", body: `{"first_name":""}`, wantPatch: &family.PersonPatch{FirstName: family.Opt[string]{Set: true}},
			updateErr: family.ErrInvalidPerson, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid person"}}`},
		{name: "unknown or deleted", body: `{}`, wantPatch: &family.PersonPatch{}, updateErr: family.ErrPersonNotFound, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "not a person id", id: "42", body: `{}`, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "busy", body: `{}`, wantPatch: &family.PersonPatch{}, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable,
			wantBody: `{"error":{"code":"busy","message":"try again in a moment"}}`},
		{name: "unknown field", body: `{"email":"x@example.com"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "database error", body: `{}`, wantPatch: &family.PersonPatch{}, updateErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantPatch != nil {
				svc.EXPECT().Update(mock.Anything, testPersonID, *tt.wantPatch).Return(testPerson, tt.updateErr)
			}
			id := idOr(tt.id)
			req := httptest.NewRequest(http.MethodPatch, "/api/persons/"+id, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParam(req, "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).UpdatePerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestDeletePerson(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteErr  error
		wantDelete bool
		wantStatus int
		wantBody   string
	}{
		{name: "deleted", wantDelete: true, wantStatus: http.StatusNoContent},
		{name: "unknown or already deleted", wantDelete: true, deleteErr: family.ErrPersonNotFound, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "not a person id", id: "42", wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", wantDelete: true, deleteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantDelete {
				svc.EXPECT().Delete(mock.Anything, testPersonID).Return(tt.deleteErr)
			}
			id := idOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/persons/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).DeletePerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestRestorePerson(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		restoreErr  error
		wantRestore bool
		wantStatus  int
		wantBody    string
	}{
		{name: "restored", wantRestore: true, wantStatus: http.StatusOK, wantBody: testPersonBody},
		{name: "unknown or not deleted", wantRestore: true, restoreErr: family.ErrPersonNotFound, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "not a person id", id: "42", wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", wantRestore: true, restoreErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantRestore {
				svc.EXPECT().Restore(mock.Anything, testPersonID).Return(testPerson, tt.restoreErr)
			}
			id := idOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/persons/"+id+"/restore", nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).RestorePerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

// serveAs runs h behind the session gate as testUser with role; an empty role skips the gate.
func serveAs(t *testing.T, role string, h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	if role == "" {
		h.ServeHTTP(rec, req)
		return rec
	}
	sessions := mocks.NewSessionAuthenticator(t)
	sessions.EXPECT().Authenticate(mock.Anything, "tok").Return(auth.Session{UserID: testUser.ID, Role: role}, nil)
	req.AddCookie(&http.Cookie{Name: "__Host-fh_session", Value: "tok"})
	handler.NewSessionGate(sessions, handler.NewSessionCookie(true)).Require(h).ServeHTTP(rec, req)
	return rec
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	assert.Equal(t, wantStatus, rec.Code)
	if wantBody == "" {
		assert.Empty(t, rec.Body.String())
		return
	}
	assert.JSONEq(t, wantBody, rec.Body.String())
}

func idOr(id string) string {
	if id != "" {
		return id
	}
	return testPersonID.String()
}
