package handler_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/contacts"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var (
	testContactID = uuid.MustParse("01890000-0000-7000-8000-0000000000dd")
	testContact   = contacts.Contact{ID: testContactID, Kind: "phone", Label: new("Mobile"), Value: "+1 555 010 0000", Visibility: "on_request"}
	memberSession = auth.Session{UserID: testUser.ID, Role: auth.RoleMember, PersonID: &testPersonID}
	memberViewer  = family.Viewer{UserID: testUser.ID, PersonID: &testPersonID}
)

const (
	testContactBody = `{"id":"01890000-0000-7000-8000-0000000000dd","kind":"phone","platform":null,"label":"Mobile",
		"value":"+1 555 010 0000","visibility":"on_request","href":null,"shared_with_you":false}`
	contactNotFoundBody = `{"error":{"code":"not_found","message":"contact method not found"}}`
)

func TestListContacts(t *testing.T) {
	listing := contacts.Listing{
		Contacts: []contacts.Contact{testContact}, Requestable: []contacts.Requestable{{Kind: "address", MyRequestStatus: "requested"}}, Full: true,
	}
	tests := []struct {
		name       string
		mine       bool
		session    auth.Session
		wantPerson *uuid.UUID
		listErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "rows, then the kinds the viewer could ask for", session: memberSession, wantPerson: &testPersonID, wantStatus: http.StatusOK,
			wantBody: `{"contacts":[` + testContactBody + `,{"kind":"address","requestable":true,"my_request_status":"requested"}]}`},
		{name: "mine, saying whose they are", mine: true, session: memberSession, wantPerson: &testPersonID, wantStatus: http.StatusOK,
			wantBody: `{"person_id":"` + testPersonID.String() + `","contacts":[` + testContactBody + `,{"kind":"address","requestable":true,"my_request_status":"requested"}]}`},
		{name: "mine, linked to nobody", mine: true, session: auth.Session{UserID: testUser.ID, Role: auth.RoleMember}, wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"your login isn't linked to anyone in the tree yet"}}`},
		{name: "unknown person", session: memberSession, wantPerson: &testPersonID, listErr: family.ErrPersonNotFound,
			wantStatus: http.StatusNotFound, wantBody: personNotFoundBody},
		{name: "database error", session: memberSession, wantPerson: &testPersonID, listErr: errors.New("db down"),
			wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewContactsService(t)
			if tt.wantPerson != nil {
				svc.EXPECT().List(mock.Anything, memberViewer, *tt.wantPerson).Return(listing, tt.listErr)
			}
			h := handler.NewContacts(svc)
			serve := h.ListFor
			if tt.mine {
				serve = h.ListMine
			}
			req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/contacts", nil), "id", testPersonID.String())

			rec := serveWith(t, tt.session, serve, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestWriteContacts(t *testing.T) {
	adminSession := auth.Session{UserID: testUser.ID, Role: auth.RoleAdmin}
	adminViewer := family.Viewer{UserID: testUser.ID, IsAdmin: true}
	tests := []struct {
		name       string
		serve      func(h *handler.Contacts) http.HandlerFunc
		method     string
		session    auth.Session
		cid        string
		body       string
		expect     func(svc *mocks.ContactsService)
		wantStatus int
		wantBody   string
	}{
		{name: "add mine", serve: func(h *handler.Contacts) http.HandlerFunc { return h.CreateMine }, method: http.MethodPost, session: memberSession,
			body: `{"kind":"phone","label":"Mobile","value":"+1 555 010 0000"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Create(mock.Anything, memberViewer, testPersonID, contacts.ContactInput{Kind: "phone", Label: new("Mobile"), Value: "+1 555 010 0000"}).
					Return(testContact, nil)
			}, wantStatus: http.StatusCreated, wantBody: testContactBody},
		{name: "add for someone", serve: func(h *handler.Contacts) http.HandlerFunc { return h.CreateFor }, method: http.MethodPost, session: adminSession,
			body: `{"kind":"social","platform":"whatsapp","value":"+1 555 010 0000","visibility":"hidden"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Create(mock.Anything, adminViewer, testPersonID, contacts.ContactInput{
					Kind: "social", Platform: new("whatsapp"), Value: "+1 555 010 0000", Visibility: new("hidden"),
				}).Return(testContact, nil)
			}, wantStatus: http.StatusCreated, wantBody: testContactBody},
		{name: "refused by the rules", serve: func(h *handler.Contacts) http.HandlerFunc { return h.CreateMine }, method: http.MethodPost, session: memberSession,
			body: `{"kind":"fax","value":"1"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Create(mock.Anything, memberViewer, testPersonID, contacts.ContactInput{Kind: "fax", Value: "1"}).Return(contacts.Contact{}, contacts.ErrInvalidContact)
			}, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid contact method"}}`},
		{name: "someone else's", serve: func(h *handler.Contacts) http.HandlerFunc { return h.CreateFor }, method: http.MethodPost, session: memberSession,
			body: `{"kind":"other","value":"a"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Create(mock.Anything, memberViewer, testPersonID, contacts.ContactInput{Kind: "other", Value: "a"}).Return(contacts.Contact{}, contacts.ErrNotYours)
			}, wantStatus: http.StatusForbidden, wantBody: `{"error":{"code":"forbidden","message":"members can change only their own contact methods"}}`},
		{name: "edit mine; null clears the label", serve: func(h *handler.Contacts) http.HandlerFunc { return h.UpdateMine }, method: http.MethodPatch,
			session: memberSession, cid: testContactID.String(), body: `{"label":null,"visibility":"on_request"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Update(mock.Anything, memberViewer, testPersonID, testContactID, contacts.ContactPatch{
					Label: family.Opt[*string]{Set: true}, Visibility: family.Opt[string]{Set: true, Value: "on_request"},
				}).Return(testContact, nil)
			}, wantStatus: http.StatusOK, wantBody: testContactBody},
		{name: "the kind can't change", serve: func(h *handler.Contacts) http.HandlerFunc { return h.UpdateMine }, method: http.MethodPatch,
			session: memberSession, cid: testContactID.String(), body: `{"kind":"email"}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "the value can't be cleared", serve: func(h *handler.Contacts) http.HandlerFunc { return h.UpdateMine }, method: http.MethodPatch,
			session: memberSession, cid: testContactID.String(), body: `{"value":null}`,
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
		{name: "not a contact id", serve: func(h *handler.Contacts) http.HandlerFunc { return h.UpdateFor }, method: http.MethodPatch,
			session: adminSession, cid: "42", body: `{}`, wantStatus: http.StatusNotFound, wantBody: contactNotFoundBody},
		{name: "delete for someone", serve: func(h *handler.Contacts) http.HandlerFunc { return h.DeleteFor }, method: http.MethodDelete,
			session: adminSession, cid: testContactID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Delete(mock.Anything, adminViewer, testPersonID, testContactID).Return(nil)
			}, wantStatus: http.StatusNoContent},
		{name: "delete one already gone", serve: func(h *handler.Contacts) http.HandlerFunc { return h.DeleteMine }, method: http.MethodDelete,
			session: memberSession, cid: testContactID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Delete(mock.Anything, memberViewer, testPersonID, testContactID).Return(contacts.ErrContactNotFound)
			}, wantStatus: http.StatusNotFound, wantBody: contactNotFoundBody},
		{name: "busy", serve: func(h *handler.Contacts) http.HandlerFunc { return h.DeleteMine }, method: http.MethodDelete,
			session: memberSession, cid: testContactID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Delete(mock.Anything, memberViewer, testPersonID, testContactID).Return(auth.ErrBusy)
			}, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewContactsService(t)
			if tt.expect != nil {
				tt.expect(svc)
			}
			req := httptest.NewRequest(tt.method, "/api/contacts", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", testPersonID.String())
			rctx.URLParams.Add("cid", tt.cid)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := serveWith(t, tt.session, tt.serve(handler.NewContacts(svc)), req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestContactRequests(t *testing.T) {
	requestID := uuid.MustParse("01890000-0000-7000-8000-0000000000ee")
	asked := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		serve      func(h *handler.Contacts) http.HandlerFunc
		method     string
		rid        string
		body       string
		expect     func(svc *mocks.ContactsService)
		wantStatus int
		wantBody   string
		wantRetry  string
	}{
		{name: "ask", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Ask }, method: http.MethodPost, body: `{"kind":"phone"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Ask(mock.Anything, memberViewer, testPersonID, "phone").Return("requested", nil)
			}, wantStatus: http.StatusOK, wantBody: `{"kind":"phone","my_request_status":"requested"}`},
		{name: "ask for something not offered", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Ask }, method: http.MethodPost, body: `{"kind":"email"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Ask(mock.Anything, memberViewer, testPersonID, "email").Return("", contacts.ErrInvalidRequest)
			}, wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid contact request"}}`},
		{name: "asked too often", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Ask }, method: http.MethodPost, body: `{"kind":"phone"}`,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Ask(mock.Anything, memberViewer, testPersonID, "phone").Return("", &auth.RateLimitedError{RetryAfter: time.Hour})
			}, wantStatus: http.StatusTooManyRequests, wantRetry: "3600",
			wantBody: `{"error":{"code":"rate_limited","message":"too many attempts, try again later"}}`},
		{name: "requests for my details, saying whose they are", serve: func(h *handler.Contacts) http.HandlerFunc { return h.ListRequests }, method: http.MethodGet,
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Requests(mock.Anything, memberViewer).Return([]contacts.Request{{
					ID: requestID, Kind: "phone", Status: "pending", CreatedAt: asked,
					From: contacts.Requester{PersonID: otherPersonID, Name: "Darline Monestime"},
				}}, nil)
			}, wantStatus: http.StatusOK,
			wantBody: `{"person_id":"` + testPersonID.String() + `","requests":[{"id":"01890000-0000-7000-8000-0000000000ee","kind":"phone","status":"pending",
				"created_at":"2026-11-15T09:30:00Z","decided_at":null,
				"from":{"person_id":"01890000-0000-7000-8000-0000000000cc","name":"Darline Monestime"}}]}`},
		{name: "share", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Share }, method: http.MethodPost, rid: requestID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Decide(mock.Anything, memberViewer, requestID, contacts.Share).Return(nil)
			}, wantStatus: http.StatusNoContent},
		{name: "decline", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Decline }, method: http.MethodPost, rid: requestID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Decide(mock.Anything, memberViewer, requestID, contacts.Decline).Return(nil)
			}, wantStatus: http.StatusNoContent},
		{name: "revoke what was never shared", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Revoke }, method: http.MethodPost, rid: requestID.String(),
			expect: func(svc *mocks.ContactsService) {
				svc.EXPECT().Decide(mock.Anything, memberViewer, requestID, contacts.Revoke).Return(fmt.Errorf("%w: only a shared request can be taken back", contacts.ErrWrongState))
			}, wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"conflict","message":"contact request can't change that way: only a shared request can be taken back"}}`},
		{name: "not a request id", serve: func(h *handler.Contacts) http.HandlerFunc { return h.Share }, method: http.MethodPost, rid: "42",
			wantStatus: http.StatusNotFound, wantBody: `{"error":{"code":"not_found","message":"contact request not found"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewContactsService(t)
			if tt.expect != nil {
				tt.expect(svc)
			}
			req := httptest.NewRequest(tt.method, "/api/contact-requests", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", testPersonID.String())
			rctx.URLParams.Add("rid", tt.rid)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := serveWith(t, memberSession, tt.serve(handler.NewContacts(svc)), req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
			assert.Equal(t, tt.wantRetry, rec.Header().Get("Retry-After"))
		})
	}
}

func TestDirectory(t *testing.T) {
	entry := contacts.DirectoryEntry{ID: testPersonID, FirstName: "Maya", LastName: new("Pluviose"), Side: new("paternal"),
		Listing: contacts.Listing{Contacts: []contacts.Contact{testContact}, Requestable: []contacts.Requestable{{Kind: "phone", MyRequestStatus: "none"}}, Full: true}}
	tests := []struct {
		name       string
		query      string
		entries    []contacts.DirectoryEntry
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "each person with what the viewer may see", query: "maya", entries: []contacts.DirectoryEntry{entry}, wantStatus: http.StatusOK,
			wantBody: `{"people":[{"id":"` + testPersonID.String() + `","first_name":"Maya","last_name":"Pluviose","side":"paternal","contacts":[` +
				testContactBody + `,{"kind":"phone","requestable":true,"my_request_status":"none"}]}]}`},
		{name: "nobody", entries: []contacts.DirectoryEntry{}, wantStatus: http.StatusOK, wantBody: `{"people":[]}`},
		{name: "a search too long", query: "maya", err: fmt.Errorf("%w: search is longer than 100 characters", family.ErrInvalidFilter),
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid filter: search is longer than 100 characters"}}`},
		{name: "database error", err: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewContactsService(t)
			svc.EXPECT().Directory(mock.Anything, memberViewer, tt.query).Return(tt.entries, tt.err)
			req := httptest.NewRequest(http.MethodGet, "/api/directory?q="+tt.query, nil)

			rec := serveWith(t, memberSession, handler.NewContacts(svc).Directory, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestListContactsMarksSharesAndLinks(t *testing.T) {
	instagram := contacts.Contact{ID: testContactID, Kind: "social", Platform: new("instagram"), Value: "@rosa.m", Visibility: "family"}
	listing := contacts.Listing{Contacts: []contacts.Contact{instagram, testContact}}
	svc := mocks.NewContactsService(t)
	svc.EXPECT().List(mock.Anything, memberViewer, testPersonID).Return(listing, nil)
	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/contacts", nil), "id", testPersonID.String())

	rec := serveWith(t, memberSession, handler.NewContacts(svc).ListFor, req)

	assertBody(t, rec, http.StatusOK, `{"contacts":[`+
		`{"id":"01890000-0000-7000-8000-0000000000dd","kind":"social","platform":"instagram","label":null,"value":"@rosa.m",`+
		`"visibility":"family","href":"https://www.instagram.com/rosa.m","shared_with_you":false},`+
		`{"id":"01890000-0000-7000-8000-0000000000dd","kind":"phone","platform":null,"label":"Mobile","value":"+1 555 010 0000",`+
		`"visibility":"on_request","href":null,"shared_with_you":true}]}`)
}
