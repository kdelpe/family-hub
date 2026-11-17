package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	memberViewer  = family.Viewer{PersonID: &testPersonID}
)

const (
	testContactBody = `{"id":"01890000-0000-7000-8000-0000000000dd","kind":"phone","platform":null,"label":"Mobile",
		"value":"+1 555 010 0000","visibility":"on_request"}`
	contactNotFoundBody = `{"error":{"code":"not_found","message":"contact method not found"}}`
)

func TestListContacts(t *testing.T) {
	listing := contacts.Listing{Contacts: []contacts.Contact{testContact}, Requestable: []string{"address"}}
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
			wantBody: `{"contacts":[` + testContactBody + `,{"kind":"address","requestable":true,"my_request_status":"none"}]}`},
		{name: "mine", mine: true, session: memberSession, wantPerson: &testPersonID, wantStatus: http.StatusOK,
			wantBody: `{"contacts":[` + testContactBody + `,{"kind":"address","requestable":true,"my_request_status":"none"}]}`},
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
	adminViewer := family.Viewer{IsAdmin: true}
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
