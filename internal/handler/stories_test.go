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

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
	"github.com/kdelpe/family-hub/internal/story"
)

var (
	testStoryID = uuid.MustParse("01890000-0000-7000-8000-0000000000ff")
	testStory   = story.Story{
		ID: testStoryID, Title: "How we came to Brooklyn", Status: "published", ConsentConfirmed: true,
		Teller:     &story.Person{ID: testPersonID, FirstName: "Jean-Robert", LastName: new("Pluviose")},
		RecordedOn: new("2026-10"), RecordedOnPrecision: new("month"), Language: new("mixed"),
		Chapters:   []story.Chapter{{At: 0, Title: "Jacmel"}},
		Transcript: []story.Line{{At: 3, Text: "Nou prale."}},
		Topics:     []string{"Moving"}, People: []story.Person{{ID: otherPersonID, FirstName: "Joseph"}},
		OtherPeople: []string{"Père Antoine"}, Featured: true, CreatedBy: &testUser.ID,
		CreatedAt: time.Date(2026, 11, 26, 15, 4, 5, 0, time.UTC),
	}
	testStoryBody = `{"id":"` + testStoryID.String() + `","title":"How we came to Brooklyn","status":"published","consent_confirmed":true,
		"teller":{"id":"` + testPersonID.String() + `","first_name":"Jean-Robert","last_name":"Pluviose"},
		"recorded_on":"2026-10","recorded_on_precision":"month","place":null,"summary":null,"language":"mixed",
		"chapters":[{"at":0,"title":"Jacmel"}],"transcript":[{"at":3,"text":"Nou prale."}],"topics":["Moving"],
		"people":[{"id":"` + otherPersonID.String() + `","first_name":"Joseph","last_name":null}],"other_people":["Père Antoine"],
		"featured":true,"created_by":"` + testUser.ID.String() + `","created_at":"2026-11-26T15:04:05Z"}`
	storyNotFoundBody    = `{"error":{"code":"not_found","message":"story not found"}}`
	memberViewerNoPerson = family.Viewer{UserID: testUser.ID}
	adminViewerNoPerson  = family.Viewer{UserID: testUser.ID, IsAdmin: true}
)

func TestListStories(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantFilter *story.Filter
		stories    []story.Story
		listErr    error
		wantStatus int
		wantBody   string
	}{
		{name: "published, without chapters or transcript", wantFilter: &story.Filter{}, stories: []story.Story{testStory}, wantStatus: http.StatusOK,
			wantBody: `{"stories":[` + strings.Replace(testStoryBody, `"chapters":[{"at":0,"title":"Jacmel"}],"transcript":[{"at":3,"text":"Nou prale."}],`, ``, 1) + `]}`},
		{name: "nothing yet", wantFilter: &story.Filter{}, wantStatus: http.StatusOK, wantBody: `{"stories":[]}`},
		{name: "a bare story lists empty, not null", wantFilter: &story.Filter{},
			stories: []story.Story{{ID: testStoryID, Title: "Draft", Status: "draft", CreatedAt: testStory.CreatedAt}}, wantStatus: http.StatusOK,
			wantBody: `{"stories":[{"id":"` + testStoryID.String() + `","title":"Draft","status":"draft","consent_confirmed":false,"teller":null,
				"recorded_on":null,"recorded_on_precision":null,"place":null,"summary":null,"language":null,
				"topics":[],"people":[],"other_people":[],"featured":false,"created_by":null,"created_at":"2026-11-26T15:04:05Z"}]}`},
		{name: "filtered", query: "?teller_id=" + testPersonID.String() + "&topic=Work&q=snow&status=published",
			wantFilter: &story.Filter{TellerID: &testPersonID, Topic: new("Work"), Q: new("snow")}, wantStatus: http.StatusOK, wantBody: `{"stories":[]}`},
		{name: "drafts", query: "?status=draft", wantFilter: &story.Filter{Drafts: true}, wantStatus: http.StatusOK, wantBody: `{"stories":[]}`},
		{name: "an unknown status", query: "?status=hidden", wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"status must be draft or published"}}`},
		{name: "a teller id that isn't one", query: "?teller_id=42", wantStatus: http.StatusBadRequest,
			wantBody: `{"error":{"code":"validation_error","message":"teller_id must be a person's id"}}`},
		{name: "database error", wantFilter: &story.Filter{}, listErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewStoryService(t)
			if tt.wantFilter != nil {
				svc.EXPECT().List(mock.Anything, memberViewerNoPerson, *tt.wantFilter).Return(tt.stories, tt.listErr)
			}

			rec := serveAs(t, auth.RoleMember, handler.NewStories(svc).List, httptest.NewRequest(http.MethodGet, "/api/stories"+tt.query, nil))

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestGetStory(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		getErr     error
		wantGet    bool
		wantStatus int
		wantBody   string
	}{
		{name: "found", wantGet: true, wantStatus: http.StatusOK, wantBody: testStoryBody},
		{name: "unknown, deleted or someone else's draft", wantGet: true, getErr: story.ErrStoryNotFound, wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
		{name: "not a story id", id: "42", wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewStoryService(t)
			if tt.wantGet {
				svc.EXPECT().Get(mock.Anything, memberViewerNoPerson, testStoryID).Return(testStory, tt.getErr)
			}
			id := storyIDOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/stories/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleMember, handler.NewStories(svc).Get, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestCreateStory(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantInput  *story.Input
		createErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "created", body: `{"title":"How we came to Brooklyn","teller_id":"` + testPersonID.String() + `","consent_confirmed":true,
			"chapters":[{"at":0,"title":"Jacmel"}],"transcript":[{"at":3,"text":"Nou prale."}],"topics":["Moving"],"other_people":["Père Antoine"]}`,
			wantInput: &story.Input{Title: "How we came to Brooklyn", TellerID: &testPersonID, ConsentConfirmed: true,
				Chapters: []story.Chapter{{At: 0, Title: "Jacmel"}}, Transcript: []story.Line{{At: 3, Text: "Nou prale."}},
				Topics: []string{"Moving"}, OtherPeople: []string{"Père Antoine"}},
			wantStatus: http.StatusCreated, wantBody: testStoryBody},
		{name: "refused by the service", body: `{"title":""}`, wantInput: &story.Input{}, createErr: fmt.Errorf("%w: a title is required", story.ErrInvalidStory),
			wantStatus: http.StatusBadRequest, wantBody: `{"error":{"code":"validation_error","message":"invalid story: a title is required"}}`},
		{name: "busy", body: `{"title":"A"}`, wantInput: &story.Input{Title: "A"}, createErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
		{name: "a video link isn't taken yet", body: `{"title":"A","media_id":"` + testStoryID.String() + `"}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "a transcript past the usual 1 MiB", body: `{"title":"A","transcript":[{"at":0,"text":"` + strings.Repeat("é", 1<<19+1) + `"}]}`,
			wantInput: &story.Input{Title: "A", Transcript: []story.Line{{At: 0, Text: strings.Repeat("é", 1<<19+1)}}}, wantStatus: http.StatusCreated, wantBody: testStoryBody},
		{name: "a chapter time that isn't a number", body: `{"title":"A","chapters":[{"at":"1:00","title":"B"}]}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewStoryService(t)
			if tt.wantInput != nil {
				svc.EXPECT().Create(mock.Anything, adminViewerNoPerson, *tt.wantInput).Return(testStory, tt.createErr)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/stories", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := serveAs(t, auth.RoleAdmin, handler.NewStories(svc).Create, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestUpdateStory(t *testing.T) {
	hide := &story.Patch{Status: family.Opt[string]{Set: true, Value: "draft"}, ConsentConfirmed: family.Opt[bool]{Set: true}}

	tests := []struct {
		name       string
		role       string
		id         string
		body       string
		wantPatch  *story.Patch
		updateErr  error
		wantStatus int
		wantBody   string
	}{
		{name: "an admin changes what's sent; null clears", role: auth.RoleAdmin, body: `{"title":"Brooklyn","place":null,"featured":true,"transcript":null}`,
			wantPatch: &story.Patch{Title: family.Opt[string]{Set: true, Value: "Brooklyn"}, Place: family.Opt[*string]{Set: true},
				Featured: family.Opt[bool]{Set: true, Value: true}, Transcript: family.Opt[[]story.Line]{Set: true}},
			wantStatus: http.StatusOK, wantBody: testStoryBody},
		{name: "a title can't be cleared", role: auth.RoleAdmin, body: `{"title":null}`, wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "a misspelled transcript field", role: auth.RoleAdmin, body: `{"transcript":[{"at_seconds":30,"text":"Hello"}]}`,
			wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "a misspelled chapter field", role: auth.RoleAdmin, body: `{"chapters":[{"at_seconds":30,"title":"Jacmel"}]}`,
			wantStatus: http.StatusBadRequest, wantBody: badJSONBody},
		{name: "a member hides a story they tell", role: auth.RoleMember, body: `{"status":"draft","consent_confirmed":false}`,
			wantPatch: hide, wantStatus: http.StatusOK, wantBody: testStoryBody},
		{name: "a member hiding one they don't tell is refused by the service", role: auth.RoleMember, body: `{"status":"draft","consent_confirmed":false}`,
			wantPatch: hide, updateErr: fmt.Errorf("%w: its teller can only hide it", story.ErrNotAllowed), wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"not allowed: its teller can only hide it"}}`},
		{name: "a member can't change anything else", role: auth.RoleMember, body: `{"status":"draft","title":"Mine"}`, wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"only admins change a story; its teller can only hide it"}}`},
		{name: "a member can't publish", role: auth.RoleMember, body: `{"status":"published"}`, wantStatus: http.StatusForbidden,
			wantBody: `{"error":{"code":"forbidden","message":"only admins change a story; its teller can only hide it"}}`},
		{name: "unknown or deleted", role: auth.RoleAdmin, body: `{}`, wantPatch: &story.Patch{}, updateErr: story.ErrStoryNotFound,
			wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
		{name: "not a story id", role: auth.RoleAdmin, id: "42", body: `{}`, wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
		{name: "busy", role: auth.RoleAdmin, body: `{}`, wantPatch: &story.Patch{}, updateErr: auth.ErrBusy, wantStatus: http.StatusServiceUnavailable, wantBody: busyBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewStoryService(t)
			if tt.wantPatch != nil && tt.role == auth.RoleAdmin {
				svc.EXPECT().Update(mock.Anything, adminViewerNoPerson, testStoryID, *tt.wantPatch).Return(testStory, tt.updateErr)
			}
			if tt.wantPatch != nil && tt.role == auth.RoleMember {
				svc.EXPECT().Hide(mock.Anything, memberViewerNoPerson, testStoryID, *tt.wantPatch).Return(testStory, tt.updateErr)
			}
			id := storyIDOr(tt.id)
			req := httptest.NewRequest(http.MethodPatch, "/api/stories/"+id, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParam(req, "id", id)

			rec := serveAs(t, tt.role, handler.NewStories(svc).Update, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func TestDeleteStory(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteErr  error
		wantDelete bool
		wantStatus int
		wantBody   string
	}{
		{name: "deleted", wantDelete: true, wantStatus: http.StatusNoContent},
		{name: "unknown or already deleted", wantDelete: true, deleteErr: story.ErrStoryNotFound, wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
		{name: "not a story id", id: "42", wantStatus: http.StatusNotFound, wantBody: storyNotFoundBody},
		{name: "database error", wantDelete: true, deleteErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewStoryService(t)
			if tt.wantDelete {
				svc.EXPECT().Delete(mock.Anything, adminViewerNoPerson, testStoryID).Return(tt.deleteErr)
			}
			id := storyIDOr(tt.id)
			req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/stories/"+id, nil), "id", id)

			rec := serveAs(t, auth.RoleAdmin, handler.NewStories(svc).Delete, req)

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}

func storyIDOr(id string) string {
	if id != "" {
		return id
	}
	return testStoryID.String()
}
