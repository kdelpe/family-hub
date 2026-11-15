package handler_test

import (
	"errors"
	"fmt"
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
	otherPersonID      = uuid.MustParse("01890000-0000-7000-8000-0000000000cc")
	testRelationshipID = uuid.MustParse("01890000-0000-7000-8000-0000000000bb")
	testParentLink     = family.Relationship{ID: testRelationshipID, Type: family.ParentChild, Subtype: "biological", PersonA: testPersonID, PersonB: otherPersonID}
	testCouple         = family.Relationship{ID: testRelationshipID, Type: family.Couple, Subtype: "married", Ended: true,
		PersonA: testPersonID, PersonB: otherPersonID, Notes: new("Married in Jacmel")}
)

const (
	testPersonBody = `{"id":"01890000-0000-7000-8000-0000000000aa","first_name":"Rosa","middle_name":null,"last_name":"Pierre",
		"maiden_name":null,"side":"first","birth_place":null,"gender":null,"birth_date":"--03-14","birth_date_precision":"day",
		"death_date":null,"death_date_precision":null,"is_deceased":false,"bio":null}`
	personNotFoundBody = `{"error":{"code":"not_found","message":"person not found"}}`
	busyBody           = `{"error":{"code":"busy","message":"try again in a moment"}}`
	parentLinkBody     = `{"id":"01890000-0000-7000-8000-0000000000bb","type":"parent_child","subtype":"biological","ended":false,
		"parent_id":"01890000-0000-7000-8000-0000000000aa","child_id":"01890000-0000-7000-8000-0000000000cc","notes":null}`
	coupleBody = `{"id":"01890000-0000-7000-8000-0000000000bb","type":"couple","subtype":"married","ended":true,
		"person_ids":["01890000-0000-7000-8000-0000000000aa","01890000-0000-7000-8000-0000000000cc"],"notes":"Married in Jacmel"}`
	relationshipNotFoundBody = `{"error":{"code":"not_found","message":"relationship not found"}}`
	badJSONBody              = `{"error":{"code":"bad_request","message":"invalid JSON body"}}`
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
				svc.EXPECT().ListPersons(mock.Anything, family.Viewer{IsAdmin: tt.role == auth.RoleAdmin}, *tt.wantIn).Return(tt.people, tt.listErr)
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
				svc.EXPECT().GetPerson(mock.Anything, family.Viewer{IsAdmin: tt.role == auth.RoleAdmin}, testPersonID).Return(testPerson, tt.getErr)
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
		{name: "busy", session: true, body: body, wantCreate: true, createErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "unknown field", session: true, body: `{"first_name":"Rosa","email":"x@example.com"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "database error", session: true, body: body, wantCreate: true, createErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", body: body, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantCreate {
				svc.EXPECT().CreatePerson(mock.Anything, testUser.ID, want).Return(testPerson, tt.createErr)
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
		{name: "deceased can't be cleared", body: `{"is_deceased":null}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "a first name can't be cleared", body: `{"first_name":null}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "busy", body: `{}`, wantPatch: &family.PersonPatch{}, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "unknown field", body: `{"email":"x@example.com"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "database error", body: `{}`, wantPatch: &family.PersonPatch{}, updateErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantPatch != nil {
				svc.EXPECT().UpdatePerson(mock.Anything, testPersonID, *tt.wantPatch).Return(testPerson, tt.updateErr)
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
		{name: "busy", wantDelete: true, deleteErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "not a person id", id: "42", wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", wantDelete: true, deleteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantDelete {
				svc.EXPECT().DeletePerson(mock.Anything, testPersonID).Return(tt.deleteErr)
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
		{name: "busy", wantRestore: true, restoreErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "not a person id", id: "42", wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", wantRestore: true, restoreErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantRestore {
				svc.EXPECT().RestorePerson(mock.Anything, testPersonID).Return(testPerson, tt.restoreErr)
			}
			id := idOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/persons/"+id+"/restore", nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).RestorePerson, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestCreateRelationship(t *testing.T) {
	const (
		parentBody = `{"type":"parent_child","parent_id":"01890000-0000-7000-8000-0000000000aa","child_id":"01890000-0000-7000-8000-0000000000cc"}`
		couple     = `{"type":"couple","person_ids":["01890000-0000-7000-8000-0000000000cc","01890000-0000-7000-8000-0000000000aa"],
			"subtype":"married","ended":true,"notes":"Married in Jacmel"}`
	)
	parentIn := family.RelationshipInput{Type: family.ParentChild, ParentID: testPersonID, ChildID: otherPersonID}
	coupleIn := family.RelationshipInput{Type: family.Couple, PersonIDs: []uuid.UUID{otherPersonID, testPersonID},
		Subtype: new("married"), Ended: true, Notes: new("Married in Jacmel")}

	tests := []struct {
		name       string
		body       string
		wantIn     *family.RelationshipInput
		created    family.Relationship
		createErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "a parent link", body: parentBody, wantIn: &parentIn, created: testParentLink, wantStatus: http.StatusCreated, wantBody: parentLinkBody},
		{name: "a couple", body: couple, wantIn: &coupleIn, created: testCouple, wantStatus: http.StatusCreated, wantBody: coupleBody},
		{name: "refused by the rules", body: parentBody, wantIn: &parentIn, createErr: family.ErrInvalidRelationship, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"invalid relationship"}}`},
		{name: "a loop", body: parentBody, wantIn: &parentIn, createErr: fmt.Errorf("%w: the child is already an ancestor of the parent", family.ErrRelationshipConflict),
			wantStatus: http.StatusConflict, wantBody: `{"error":{"code":"conflict","message":"relationship conflict: the child is already an ancestor of the parent"}}`},
		{name: "busy", body: parentBody, wantIn: &parentIn, createErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "database error", body: parentBody, wantIn: &parentIn, createErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "an id that isn't a UUID", body: `{"type":"parent_child","parent_id":"42"}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "unknown field", body: `{"type":"sibling","sibling_id":"01890000-0000-7000-8000-0000000000cc"}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantIn != nil {
				svc.EXPECT().CreateRelationship(mock.Anything, *tt.wantIn).Return(tt.created, tt.createErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/relationships", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).CreateRelationship, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestUpdateRelationship(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		body       string
		wantPatch  *family.RelationshipPatch
		updateErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "only the fields sent change; null clears the notes", body: `{"ended":true,"notes":null}`,
			wantPatch:  &family.RelationshipPatch{Ended: family.Opt[bool]{Set: true, Value: true}, Notes: family.Opt[*string]{Set: true}},
			wantStatus: http.StatusOK, wantBody: coupleBody},
		{name: "a subtype", body: `{"subtype":"partner"}`, wantPatch: &family.RelationshipPatch{Subtype: family.Opt[string]{Set: true, Value: "partner"}},
			wantStatus: http.StatusOK, wantBody: coupleBody},
		{name: "a third biological parent", body: `{"subtype":"biological"}`, wantPatch: &family.RelationshipPatch{Subtype: family.Opt[string]{Set: true, Value: "biological"}},
			updateErr: fmt.Errorf("%w: the child already has two biological parents", family.ErrRelationshipConflict), wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"conflict","message":"relationship conflict: the child already has two biological parents"}}`},
		{name: "unknown", body: `{}`, wantPatch: &family.RelationshipPatch{}, updateErr: family.ErrRelationshipNotFound, wantStatus: http.StatusNotFound, wantBody: relationshipNotFoundBody},
		{name: "not a relationship id", id: "42", body: `{}`, wantStatus: http.StatusNotFound, wantBody: relationshipNotFoundBody},
		{name: "busy", body: `{}`, wantPatch: &family.RelationshipPatch{}, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "the people can't change", body: `{"parent_id":"01890000-0000-7000-8000-0000000000cc"}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "a subtype can't be cleared", body: `{"subtype":null}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantPatch != nil {
				svc.EXPECT().UpdateRelationship(mock.Anything, testRelationshipID, *tt.wantPatch).Return(testCouple, tt.updateErr)
			}
			id := testRelationshipID.String()
			if tt.id != "" {
				id = tt.id
			}
			req := httptest.NewRequest(http.MethodPatch, "/api/relationships/"+id, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParam(req, "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).UpdateRelationship, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestDeleteRelationship(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteErr  error
		wantDelete bool
		wantStatus int
		wantBody   string
	}{
		{name: "deleted", wantDelete: true, wantStatus: http.StatusNoContent},
		{name: "unknown", wantDelete: true, deleteErr: family.ErrRelationshipNotFound, wantStatus: http.StatusNotFound, wantBody: relationshipNotFoundBody},
		{name: "not a relationship id", id: "42", wantStatus: http.StatusNotFound, wantBody: relationshipNotFoundBody},
		{name: "busy", wantDelete: true, deleteErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "database error", wantDelete: true, deleteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			if tt.wantDelete {
				svc.EXPECT().DeleteRelationship(mock.Anything, testRelationshipID).Return(tt.deleteErr)
			}
			id := testRelationshipID.String()
			if tt.id != "" {
				id = tt.id
			}
			req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/relationships/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewFamily(svc).DeleteRelationship, req)

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

func TestTree(t *testing.T) {
	tree := family.Tree{
		Persons: []family.TreePerson{{ID: testPersonID, FirstName: "Rosa", LastName: new("Pierre"), BioExcerpt: new("Born in Jacmel")}},
		Relationships: []family.TreeLink{
			{ID: testRelationshipID, Type: family.ParentChild, Subtype: "biological", PersonA: testPersonID, PersonB: otherPersonID},
			{ID: testRelationshipID, Type: family.Couple, Subtype: "married", Ended: true, PersonA: testPersonID, PersonB: otherPersonID},
		},
	}
	tests := []struct {
		name       string
		tree       family.Tree
		treeErr    error
		noSession  bool
		wantStatus int
		wantBody   string
	}{
		{name: "people and links", tree: tree, wantStatus: http.StatusOK, wantBody: `{"persons":[{"id":"01890000-0000-7000-8000-0000000000aa",
			"first_name":"Rosa","last_name":"Pierre","side":null,"gender":null,"is_deceased":false,"bio_excerpt":"Born in Jacmel"}],
			"relationships":[
			{"id":"01890000-0000-7000-8000-0000000000bb","type":"parent_child","subtype":"biological","ended":false,
			 "parent_id":"01890000-0000-7000-8000-0000000000aa","child_id":"01890000-0000-7000-8000-0000000000cc"},
			{"id":"01890000-0000-7000-8000-0000000000bb","type":"couple","subtype":"married","ended":true,
			 "person_ids":["01890000-0000-7000-8000-0000000000aa","01890000-0000-7000-8000-0000000000cc"]}]}`},
		{name: "an empty tree", wantStatus: http.StatusOK, wantBody: `{"persons":[],"relationships":[]}`},
		{name: "busy", treeErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "database error", treeErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", noSession: true, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			role := auth.RoleMember
			if tt.noSession {
				role = ""
			} else {
				svc.EXPECT().Tree(mock.Anything).Return(tt.tree, tt.treeErr)
			}

			rec := serveAs(t, role, handler.NewFamily(svc).Tree, httptest.NewRequest(http.MethodGet, "/api/tree", nil))

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestAncestorsAndDescendants(t *testing.T) {
	relatives := []family.Relative{{ID: otherPersonID, FirstName: "Jean", BirthDate: new("--03-14"), BirthDatePrecision: new("day"), Depth: 2}}
	const relativesBody = `{"persons":[{"id":"01890000-0000-7000-8000-0000000000cc","first_name":"Jean","last_name":null,
		"is_deceased":false,"birth_date":"--03-14","birth_date_precision":"day","depth":2}]}`

	tests := []struct {
		name       string
		role       string
		id         string
		query      string
		wantDepth  int
		noWalk     bool
		walkErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "the default depth, as a member", role: auth.RoleMember, wantDepth: family.DefaultDepth, wantStatus: http.StatusOK, wantBody: relativesBody},
		{name: "a depth, as an admin", role: auth.RoleAdmin, query: "?depth=2", wantDepth: 2, wantStatus: http.StatusOK, wantBody: relativesBody},
		{name: "depth not a number goes to the service as out of range", role: auth.RoleMember, query: "?depth=all", wantDepth: 0,
			walkErr: family.ErrInvalidFilter, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid filter"}}`},
		{name: "depth out of range", role: auth.RoleMember, query: "?depth=99", wantDepth: 99, walkErr: family.ErrInvalidFilter,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid filter"}}`},
		{name: "unknown or deleted", role: auth.RoleMember, wantDepth: family.DefaultDepth, walkErr: family.ErrPersonNotFound,
			wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "not a person id", role: auth.RoleMember, id: "42", noWalk: true, wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", role: auth.RoleMember, wantDepth: family.DefaultDepth, walkErr: errors.New("db down"),
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", noWalk: true, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, direction := range []string{"ancestors", "descendants"} {
		for _, tt := range tests {
			t.Run(direction+": "+tt.name, func(t *testing.T) {
				svc := mocks.NewFamilyService(t)
				h := handler.NewFamily(svc).Ancestors
				if !tt.noWalk {
					viewer := family.Viewer{IsAdmin: tt.role == auth.RoleAdmin}
					if direction == "ancestors" {
						svc.EXPECT().Ancestors(mock.Anything, viewer, testPersonID, tt.wantDepth).Return(relatives, tt.walkErr)
					} else {
						svc.EXPECT().Descendants(mock.Anything, viewer, testPersonID, tt.wantDepth).Return(relatives, tt.walkErr)
					}
				}
				if direction == "descendants" {
					h = handler.NewFamily(svc).Descendants
				}
				id := idOr(tt.id)
				req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/persons/"+id+"/"+direction+tt.query, nil), "id", id)

				rec := serveAs(t, tt.role, h, req)

				assertBody(t, rec, tt.wantStatus, tt.wantBody)
			})
		}
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name       string
		summary    family.Summary
		sumErr     error
		noSession  bool
		wantStatus int
		wantBody   string
	}{
		{name: "with an earliest record", summary: family.Summary{People: 38, Generations: 5,
			EarliestRecord: &family.EarliestRecord{Year: 1900, Circa: true, Place: new("Jacmel")}},
			wantStatus: http.StatusOK, wantBody: `{"people":38,"generations":5,"earliest_record":{"year":1900,"circa":true,"place":"Jacmel"}}`},
		{name: "nobody yet", wantStatus: http.StatusOK, wantBody: `{"people":0,"generations":0,"earliest_record":null}`},
		{name: "database error", sumErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", noSession: true, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewFamilyService(t)
			role := auth.RoleMember
			if tt.noSession {
				role = ""
			} else {
				svc.EXPECT().Summary(mock.Anything).Return(tt.summary, tt.sumErr)
			}

			rec := serveAs(t, role, handler.NewFamily(svc).Summary, httptest.NewRequest(http.MethodGet, "/api/family/summary", nil))

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}
