package handler_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

var testAccessRequestID = uuid.MustParse("01890000-0000-7000-8000-0000000000ee")

func TestSubmitAccessRequest(t *testing.T) {
	const form = `{"full_name":"Darline Monestime","email":"darline@example.com","side":"second","relationship":"son",
		"related_names":["Frantz Monestime"],"note":"Cousin Maya told me about the site."}`
	tests := []struct {
		name       string
		body       string
		wantSubmit bool
		submitErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "received", body: form, wantSubmit: true, wantStatus: http.StatusAccepted},
		{name: "a honeypot filled in stores nothing, and says the same", body: `{"full_name":"Bot","email":"bot@example.com","website":"spam"}`,
			wantStatus: http.StatusAccepted},
		{name: "refused by the rules", body: form, wantSubmit: true, submitErr: fmt.Errorf("%w: side must be first, second or married_in", admin.ErrInvalidAccessRequest),
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid access request: side must be first, second or married_in"}}`},
		{name: "database error", body: form, wantSubmit: true, submitErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "unknown field", body: `{"full_name":"Darline","password":"x"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAccessService(t)
			if tt.wantSubmit {
				svc.EXPECT().Submit(mock.Anything, admin.AccessRequestInput{
					FullName: "Darline Monestime", Email: "darline@example.com", Side: new("second"), Relationship: new("son"),
					RelatedNames: []string{"Frantz Monestime"}, Note: new("Cousin Maya told me about the site."),
				}).Return(tt.submitErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/access-requests", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.NewAccess(svc).Submit(rec, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestListAccessRequests(t *testing.T) {
	asked := time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC)
	svc := mocks.NewAccessService(t)
	svc.EXPECT().List(mock.Anything).Return([]admin.AccessRequest{{
		ID: testAccessRequestID, FullName: "Kervens Pluviose", Email: "kervens@example.com", Side: new("first"), Relationship: new("son"),
		RelatedNames: []admin.RelatedName{{Name: "Maya Pluviose", Match: &admin.PersonName{PersonID: testPersonID, Name: "Maya Pluviose"}}, {Name: "Nobody"}},
		Source:       "member_suggestion", SuggestedBy: &admin.PersonName{PersonID: otherPersonID, Name: "Stanley Jean-Louis"},
		Status: "pending", CreatedAt: asked,
	}}, nil)
	rec := serveAs(t, "admin", handler.NewAccess(svc).List, httptest.NewRequest(http.MethodGet, "/api/admin/access-requests", nil))

	assertBody(t, rec, http.StatusOK, `{"requests":[{"id":"01890000-0000-7000-8000-0000000000ee","full_name":"Kervens Pluviose",
		"email":"kervens@example.com","side":"first","relationship":"son",
		"related_names":[{"name":"Maya Pluviose","match":{"person_id":"01890000-0000-7000-8000-0000000000aa","name":"Maya Pluviose"}},
			{"name":"Nobody","match":null}],
		"note":null,"source":"member_suggestion","suggested_by":{"person_id":"01890000-0000-7000-8000-0000000000cc","name":"Stanley Jean-Louis"},
		"status":"pending","reviewed_at":null,"created_at":"2026-11-15T09:30:00Z"}]}`)
}

func TestDeclineAndReopenAccessRequests(t *testing.T) {
	tests := []struct {
		name       string
		reopen     bool
		id         string
		err        error
		wantCall   bool
		wantStatus int
		wantBody   string
	}{
		{name: "declined", wantCall: true, wantStatus: http.StatusNoContent},
		{name: "reopened", reopen: true, wantCall: true, wantStatus: http.StatusNoContent},
		{name: "reopening while another is pending", reopen: true, wantCall: true,
			err: fmt.Errorf("%w: another request for that email is pending", admin.ErrAccessRequestState), wantStatus: http.StatusConflict,
			wantBody: `{"error":{"code":"conflict","message":"access request can't change that way: another request for that email is pending"}}`},
		{name: "unknown", wantCall: true, err: admin.ErrAccessRequestNotFound, wantStatus: http.StatusNotFound,
			wantBody: `{"error":{"code":"not_found","message":"access request not found"}}`},
		{name: "not an id", id: "42", wantStatus: http.StatusNotFound, wantBody: `{"error":{"code":"not_found","message":"access request not found"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAccessService(t)
			h := handler.NewAccess(svc)
			serve := h.Decline
			if tt.reopen {
				serve = h.Reopen
			}
			if tt.wantCall && tt.reopen {
				svc.EXPECT().Reopen(mock.Anything, testAccessRequestID).Return(tt.err)
			} else if tt.wantCall {
				svc.EXPECT().Decline(mock.Anything, testUser.ID, testAccessRequestID).Return(tt.err)
			}
			id := tt.id
			if id == "" {
				id = testAccessRequestID.String()
			}
			req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/admin/access-requests/"+id+"/decline", nil), "id", id)

			rec := serveAs(t, "admin", serve, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestSuggestARelative(t *testing.T) {
	const body = `{"full_name":"Kervens Pluviose","email":"kervens@example.com","relationship":"cousin","related_names":[]}`
	in := admin.AccessRequestInput{FullName: "Kervens Pluviose", Email: "kervens@example.com", Relationship: new("cousin"), RelatedNames: []string{}}
	tests := []struct {
		name       string
		body       string
		err        error
		wantCall   bool
		wantStatus int
		wantBody   string
	}{
		{name: "received", body: body, wantCall: true, wantStatus: http.StatusAccepted},
		{name: "a login linked to nobody", body: body, wantCall: true, err: admin.ErrNotLinked, wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"your login isn't linked to anyone in the tree yet"}}`},
		{name: "refused by the rules", body: body, wantCall: true, err: admin.ErrInvalidAccessRequest, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"invalid access request"}}`},
		{name: "no honeypot for members", body: `{"full_name":"Kervens","website":"x"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"bad_request","message":"invalid JSON body"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewAccessService(t)
			if tt.wantCall {
				svc.EXPECT().Suggest(mock.Anything, testUser.ID, in).Return(tt.err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/me/suggestions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := serveWith(t, memberSession, handler.NewAccess(svc).Suggest, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}
