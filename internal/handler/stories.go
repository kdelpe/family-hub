package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
	"github.com/kdelpe/family-hub/internal/story"
)

type StoryService interface {
	List(ctx context.Context, viewer family.Viewer, f story.Filter) ([]story.Story, error)
	Get(ctx context.Context, viewer family.Viewer, id uuid.UUID) (story.Story, error)
	Create(ctx context.Context, viewer family.Viewer, in story.Input) (story.Story, error)
	Update(ctx context.Context, viewer family.Viewer, id uuid.UUID, patch story.Patch) (story.Story, error)
	Hide(ctx context.Context, viewer family.Viewer, id uuid.UUID, patch story.Patch) (story.Story, error)
	Delete(ctx context.Context, viewer family.Viewer, id uuid.UUID) error
}

// Stories serves the family's recorded stories (architecture §9.6, design-notes §4.6–4.7b).
type Stories struct {
	svc StoryService
}

func NewStories(svc StoryService) *Stories {
	return &Stories{svc: svc}
}

// A story's transcript can reach 300,000 characters; in a multibyte script, with its JSON around each line, that is
// well past the usual 1 MiB.
const maxStoryBody = 4 << 20

type storyResponse struct {
	ID                  uuid.UUID             `json:"id"`
	Title               string                `json:"title"`
	Status              string                `json:"status"`
	ConsentConfirmed    bool                  `json:"consent_confirmed"`
	Teller              *storyPersonResponse  `json:"teller"`
	RecordedOn          *string               `json:"recorded_on"`
	RecordedOnPrecision *string               `json:"recorded_on_precision"`
	Place               *string               `json:"place"`
	Summary             *string               `json:"summary"`
	Language            *string               `json:"language"`
	Chapters            *[]story.Chapter      `json:"chapters,omitempty"`
	Transcript          *[]story.Line         `json:"transcript,omitempty"`
	Topics              []string              `json:"topics"`
	People              []storyPersonResponse `json:"people"`
	OtherPeople         []string              `json:"other_people"`
	Featured            bool                  `json:"featured"`
	CreatedBy           *uuid.UUID            `json:"created_by"`
	CreatedAt           time.Time             `json:"created_at"`
}

type storyPersonResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  *string   `json:"last_name"`
}

type storiesResponse struct {
	Stories []storyResponse `json:"stories"`
}

type createStoryRequest struct {
	Title               string          `json:"title"`
	TellerID            *uuid.UUID      `json:"teller_id"`
	ConsentConfirmed    bool            `json:"consent_confirmed"`
	RecordedOn          *string         `json:"recorded_on"`
	RecordedOnPrecision *string         `json:"recorded_on_precision"`
	Place               *string         `json:"place"`
	Summary             *string         `json:"summary"`
	Language            *string         `json:"language"`
	Chapters            []story.Chapter `json:"chapters"`
	Transcript          []story.Line    `json:"transcript"`
	Topics              []string        `json:"topics"`
	PersonIDs           []uuid.UUID     `json:"person_ids"`
	OtherPeople         []string        `json:"other_people"`
}

type updateStoryRequest struct {
	Title               required[string]          `json:"title"`
	TellerID            optional[*uuid.UUID]      `json:"teller_id"`
	Status              required[string]          `json:"status"`
	ConsentConfirmed    required[bool]            `json:"consent_confirmed"`
	Featured            required[bool]            `json:"featured"`
	RecordedOn          optional[*string]         `json:"recorded_on"`
	RecordedOnPrecision optional[*string]         `json:"recorded_on_precision"`
	Place               optional[*string]         `json:"place"`
	Summary             optional[*string]         `json:"summary"`
	Language            optional[*string]         `json:"language"`
	Chapters            optional[[]story.Chapter] `json:"chapters"`
	Transcript          optional[[]story.Line]    `json:"transcript"`
	Topics              optional[[]string]        `json:"topics"`
	PersonIDs           optional[[]uuid.UUID]     `json:"person_ids"`
	OtherPeople         optional[[]string]        `json:"other_people"`
}

// List takes ?teller_id=&topic=&q=, and ?status=draft for drafts instead of published stories.
func (s *Stories) List(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	f := story.Filter{Topic: queryValue(query, "topic"), Q: queryValue(query, "q")}
	switch query.Get("status") {
	case "", "published":
	case "draft":
		f.Drafts = true
	default:
		httpx.Error(w, http.StatusBadRequest, "validation_error", "status must be draft or published")
		return
	}
	if teller := query.Get("teller_id"); teller != "" {
		id, err := uuid.Parse(teller)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "validation_error", "teller_id must be a person's id")
			return
		}
		f.TellerID = &id
	}
	stories, err := s.svc.List(r.Context(), viewer, f)
	if err != nil {
		s.fail(w, r, err, "list stories")
		return
	}
	resp := storiesResponse{Stories: make([]storyResponse, 0, len(stories))}
	for _, st := range stories {
		resp.Stories = append(resp.Stories, toStoryResponse(st, false))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (s *Stories) Get(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	id, ok := storyID(w, r)
	if !ok {
		return
	}
	st, err := s.svc.Get(r.Context(), viewer, id)
	s.answer(w, r, http.StatusOK, st, err, "get story")
}

// Create is admin-only until MEMBERS_CAN_CONTRIBUTE exists; the router checks the role.
func (s *Stories) Create(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	var req createStoryRequest
	if !httpx.DecodeJSONUpTo(w, r, &req, maxStoryBody) {
		return
	}
	st, err := s.svc.Create(r.Context(), viewer, story.Input(req))
	s.answer(w, r, http.StatusCreated, st, err, "create story")
}

// Update is for admins; a member may only hide a story they tell, even one they added.
func (s *Stories) Update(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	id, ok := storyID(w, r)
	if !ok {
		return
	}
	var req updateStoryRequest
	if !httpx.DecodeJSONUpTo(w, r, &req, maxStoryBody) {
		return
	}
	patch := story.Patch{
		Title: family.Opt[string](req.Title), TellerID: family.Opt[*uuid.UUID](req.TellerID), Status: family.Opt[string](req.Status),
		ConsentConfirmed: family.Opt[bool](req.ConsentConfirmed), Featured: family.Opt[bool](req.Featured),
		RecordedOn: family.Opt[*string](req.RecordedOn), RecordedOnPrecision: family.Opt[*string](req.RecordedOnPrecision),
		Place: family.Opt[*string](req.Place), Summary: family.Opt[*string](req.Summary), Language: family.Opt[*string](req.Language),
		Chapters: family.Opt[[]story.Chapter](req.Chapters), Transcript: family.Opt[[]story.Line](req.Transcript),
		Topics: family.Opt[[]string](req.Topics), PersonIDs: family.Opt[[]uuid.UUID](req.PersonIDs),
		OtherPeople: family.Opt[[]string](req.OtherPeople),
	}
	update := s.svc.Update
	if !viewer.IsAdmin {
		if !patch.OnlyHides() {
			httpx.Error(w, http.StatusForbidden, "forbidden", "only admins change a story; its teller can only hide it")
			return
		}
		update = s.svc.Hide
	}
	st, err := update(r.Context(), viewer, id, patch)
	s.answer(w, r, http.StatusOK, st, err, "update story")
}

// Delete is admin-only; the router checks the role.
func (s *Stories) Delete(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	id, ok := storyID(w, r)
	if !ok {
		return
	}
	if err := s.svc.Delete(r.Context(), viewer, id); err != nil {
		s.fail(w, r, err, "delete story")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Stories) answer(w http.ResponseWriter, r *http.Request, status int, st story.Story, err error, what string) {
	if err != nil {
		s.fail(w, r, err, what)
		return
	}
	httpx.JSON(w, status, toStoryResponse(st, true))
}

func (s *Stories) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	switch {
	case clientGone(r):
	case errors.Is(err, story.ErrInvalidStory):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, story.ErrStoryNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, story.ErrNotAllowed):
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, what)
	}
}

// queryValue is nil for a parameter left out or empty, so the filter shows only what was asked.
func queryValue(query url.Values, key string) *string {
	if v := query.Get(key); v != "" {
		return &v
	}
	return nil
}

func storyID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return pathID(w, r, "id", story.ErrStoryNotFound)
}

// toStoryResponse leaves out chapters and transcript unless full: a list doesn't load them, and [] would read as none.
func toStoryResponse(st story.Story, full bool) storyResponse {
	people := make([]storyPersonResponse, 0, len(st.People))
	for _, p := range st.People {
		people = append(people, storyPersonResponse(p))
	}
	resp := storyResponse{
		ID: st.ID, Title: st.Title, Status: st.Status, ConsentConfirmed: st.ConsentConfirmed,
		RecordedOn: st.RecordedOn, RecordedOnPrecision: st.RecordedOnPrecision, Place: st.Place, Summary: st.Summary,
		Language: st.Language, Topics: nonNil(st.Topics),
		People: people, OtherPeople: nonNil(st.OtherPeople), Featured: st.Featured, CreatedBy: st.CreatedBy, CreatedAt: st.CreatedAt,
	}
	if full {
		resp.Chapters, resp.Transcript = new(nonNil(st.Chapters)), new(nonNil(st.Transcript))
	}
	if st.Teller != nil {
		resp.Teller = new(storyPersonResponse(*st.Teller))
	}
	return resp
}

// nonNil makes an empty list [] in JSON rather than null.
func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
