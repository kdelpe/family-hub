package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type AccessService interface {
	Submit(ctx context.Context, in admin.AccessRequestInput) error
	Suggest(ctx context.Context, userID, asPerson uuid.UUID, in admin.AccessRequestInput) error
	List(ctx context.Context) ([]admin.AccessRequest, error)
	Decline(ctx context.Context, adminID, id uuid.UUID) error
	Reopen(ctx context.Context, id uuid.UUID) error
}

// Access serves the public Request access form and the admins' Requests queue.
type Access struct {
	svc AccessService
}

func NewAccess(svc AccessService) *Access {
	return &Access{svc: svc}
}

// accessForm is what someone asking, or a member suggesting a relative, fills in. A member's is the whole body:
// they're signed in, so it has no honeypot.
type accessForm struct {
	FullName     string   `json:"full_name"`
	Email        string   `json:"email"`
	Side         *string  `json:"side"`
	Relationship *string  `json:"relationship"`
	RelatedNames []string `json:"related_names"`
	Note         *string  `json:"note"`
}

// suggestionRequest also says who the member filled it in as, since an admin can relink the login meanwhile (§5.1).
type suggestionRequest struct {
	accessForm
	PersonID uuid.UUID `json:"person_id"`
}

type accessRequestRequest struct {
	accessForm
	// Website is a honeypot: the form hides it, so only a bot fills it in.
	Website string `json:"website"`
}

type accessRequestsResponse struct {
	Requests []accessRequestResponse `json:"requests"`
}

type accessRequestResponse struct {
	ID           uuid.UUID             `json:"id"`
	FullName     string                `json:"full_name"`
	Email        string                `json:"email"`
	Side         *string               `json:"side"`
	Relationship *string               `json:"relationship"`
	RelatedNames []relatedNameResponse `json:"related_names"`
	Note         *string               `json:"note"`
	Source       string                `json:"source"`
	SuggestedBy  *personNameResponse   `json:"suggested_by"`
	Status       string                `json:"status"`
	ReviewedAt   *time.Time            `json:"reviewed_at"`
	CreatedAt    time.Time             `json:"created_at"`
}

type relatedNameResponse struct {
	Name  string              `json:"name"`
	Match *personNameResponse `json:"match"`
}

type personNameResponse struct {
	PersonID uuid.UUID `json:"person_id"`
	Name     string    `json:"name"`
}

// Submit answers 202 whether or not the email has an account or a pending request (§5).
func (a *Access) Submit(w http.ResponseWriter, r *http.Request) {
	var req accessRequestRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Website == "" {
		err := a.svc.Submit(r.Context(), admin.AccessRequestInput(req.accessForm))
		if err != nil {
			a.fail(w, r, err, "submit access request")
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// Suggest answers 202 whether the suggestion was stored or dropped as a repeat or over the daily limit, like
// the public form (§5).
func (a *Access) Suggest(w http.ResponseWriter, r *http.Request) {
	session, ok := CurrentSession(r.Context())
	if !ok {
		internalError(w, r, errors.New("no session in context"), "suggestion mounted without RequireSession")
		return
	}
	var req suggestionRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.PersonID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, "validation_error", "person_id is required")
		return
	}
	if err := a.svc.Suggest(r.Context(), session.UserID, req.PersonID, admin.AccessRequestInput(req.accessForm)); err != nil {
		a.fail(w, r, err, "suggest a relative")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (a *Access) List(w http.ResponseWriter, r *http.Request) {
	requests, err := a.svc.List(r.Context())
	if err != nil {
		a.fail(w, r, err, "list access requests")
		return
	}
	resp := accessRequestsResponse{Requests: make([]accessRequestResponse, 0, len(requests))}
	for _, req := range requests {
		names := make([]relatedNameResponse, 0, len(req.RelatedNames))
		for _, n := range req.RelatedNames {
			names = append(names, relatedNameResponse{Name: n.Name, Match: (*personNameResponse)(n.Match)})
		}
		resp.Requests = append(resp.Requests, accessRequestResponse{
			ID: req.ID, FullName: req.FullName, Email: req.Email, Side: req.Side, Relationship: req.Relationship,
			RelatedNames: names, Note: req.Note, Source: req.Source, SuggestedBy: (*personNameResponse)(req.SuggestedBy),
			Status: req.Status, ReviewedAt: req.ReviewedAt, CreatedAt: req.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (a *Access) Decline(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentAdmin(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", admin.ErrAccessRequestNotFound)
	if !ok {
		return
	}
	a.done(w, r, a.svc.Decline(r.Context(), actor, id), "decline access request")
}

func (a *Access) Reopen(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", admin.ErrAccessRequestNotFound)
	if !ok {
		return
	}
	a.done(w, r, a.svc.Reopen(r.Context(), id), "reopen access request")
}

func (a *Access) done(w http.ResponseWriter, r *http.Request, err error, what string) {
	if err != nil {
		a.fail(w, r, err, what)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Access) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	switch {
	case clientGone(r):
	case errors.Is(err, admin.ErrInvalidAccessRequest):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, admin.ErrNotLinked):
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, admin.ErrAccessRequestNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, admin.ErrAccessRequestState), errors.Is(err, admin.ErrRelinked):
		httpx.Error(w, http.StatusConflict, "conflict", err.Error())
	default:
		internalError(w, r, err, what)
	}
}
