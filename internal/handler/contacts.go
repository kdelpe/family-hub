package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/contacts"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type ContactsService interface {
	List(ctx context.Context, viewer family.Viewer, personID uuid.UUID) (contacts.Listing, error)
	Create(ctx context.Context, viewer family.Viewer, personID uuid.UUID, in contacts.ContactInput) (contacts.Contact, error)
	Update(ctx context.Context, viewer family.Viewer, personID, id uuid.UUID, patch contacts.ContactPatch) (contacts.Contact, error)
	Delete(ctx context.Context, viewer family.Viewer, personID, id uuid.UUID) error
}

// Contacts serves /api/me/contacts (the viewer's own person) and /api/persons/{id}/contacts.
type Contacts struct {
	svc ContactsService
}

func NewContacts(svc ContactsService) *Contacts {
	return &Contacts{svc: svc}
}

type contactResponse struct {
	ID         uuid.UUID `json:"id"`
	Kind       string    `json:"kind"`
	Platform   *string   `json:"platform"`
	Label      *string   `json:"label"`
	Value      string    `json:"value"`
	Visibility string    `json:"visibility"`
}

// requestableResponse stands in for on_request rows the viewer can't see: the kind, never the value (§5.2).
type requestableResponse struct {
	Kind            string `json:"kind"`
	Requestable     bool   `json:"requestable"`
	MyRequestStatus string `json:"my_request_status"`
}

type contactsResponse struct {
	Contacts []any `json:"contacts"`
}

type createContactRequest struct {
	Kind       string  `json:"kind"`
	Platform   *string `json:"platform"`
	Label      *string `json:"label"`
	Value      string  `json:"value"`
	Visibility *string `json:"visibility"`
}

// Kind and platform never change, so a PATCH naming them is refused as an unknown field.
type updateContactRequest struct {
	Label      optional[*string] `json:"label"`
	Value      required[string]  `json:"value"`
	Visibility required[string]  `json:"visibility"`
}

func (c *Contacts) ListMine(w http.ResponseWriter, r *http.Request)   { c.list(w, r, true) }
func (c *Contacts) CreateMine(w http.ResponseWriter, r *http.Request) { c.create(w, r, true) }
func (c *Contacts) UpdateMine(w http.ResponseWriter, r *http.Request) { c.update(w, r, true) }
func (c *Contacts) DeleteMine(w http.ResponseWriter, r *http.Request) { c.delete(w, r, true) }
func (c *Contacts) ListFor(w http.ResponseWriter, r *http.Request)    { c.list(w, r, false) }
func (c *Contacts) CreateFor(w http.ResponseWriter, r *http.Request)  { c.create(w, r, false) }
func (c *Contacts) UpdateFor(w http.ResponseWriter, r *http.Request)  { c.update(w, r, false) }
func (c *Contacts) DeleteFor(w http.ResponseWriter, r *http.Request)  { c.delete(w, r, false) }

func (c *Contacts) list(w http.ResponseWriter, r *http.Request, mine bool) {
	viewer, personID, ok := c.target(w, r, mine)
	if !ok {
		return
	}
	listing, err := c.svc.List(r.Context(), viewer, personID)
	if err != nil {
		c.fail(w, r, err, "list contacts")
		return
	}
	resp := contactsResponse{Contacts: make([]any, 0, len(listing.Contacts)+len(listing.Requestable))}
	for _, contact := range listing.Contacts {
		resp.Contacts = append(resp.Contacts, contactResponse(contact))
	}
	for _, kind := range listing.Requestable {
		// Requests arrive in the next PR; until then nobody has asked.
		resp.Contacts = append(resp.Contacts, requestableResponse{Kind: kind, Requestable: true, MyRequestStatus: "none"})
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (c *Contacts) create(w http.ResponseWriter, r *http.Request, mine bool) {
	viewer, personID, ok := c.target(w, r, mine)
	if !ok {
		return
	}
	var req createContactRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	contact, err := c.svc.Create(r.Context(), viewer, personID, contacts.ContactInput(req))
	c.answer(w, r, http.StatusCreated, contact, err, "create contact")
}

func (c *Contacts) update(w http.ResponseWriter, r *http.Request, mine bool) {
	viewer, personID, ok := c.target(w, r, mine)
	if !ok {
		return
	}
	id, ok := contactID(w, r)
	if !ok {
		return
	}
	var req updateContactRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	patch := contacts.ContactPatch{
		Label: family.Opt[*string](req.Label), Value: family.Opt[string](req.Value), Visibility: family.Opt[string](req.Visibility),
	}
	contact, err := c.svc.Update(r.Context(), viewer, personID, id, patch)
	c.answer(w, r, http.StatusOK, contact, err, "update contact")
}

func (c *Contacts) delete(w http.ResponseWriter, r *http.Request, mine bool) {
	viewer, personID, ok := c.target(w, r, mine)
	if !ok {
		return
	}
	id, ok := contactID(w, r)
	if !ok {
		return
	}
	if err := c.svc.Delete(r.Context(), viewer, personID, id); err != nil {
		c.fail(w, r, err, "delete contact")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// target is the viewer and the person the request is about: their own for /me, else the one in the path.
func (c *Contacts) target(w http.ResponseWriter, r *http.Request, mine bool) (family.Viewer, uuid.UUID, bool) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return family.Viewer{}, uuid.Nil, false
	}
	if !mine {
		id, ok := personID(w, r)
		return viewer, id, ok
	}
	id, err := contacts.Mine(viewer)
	if err != nil {
		c.fail(w, r, err, "my contacts")
		return family.Viewer{}, uuid.Nil, false
	}
	return viewer, id, true
}

func (c *Contacts) answer(w http.ResponseWriter, r *http.Request, status int, contact contacts.Contact, err error, what string) {
	if err != nil {
		c.fail(w, r, err, what)
		return
	}
	httpx.JSON(w, status, contactResponse(contact))
}

func (c *Contacts) fail(w http.ResponseWriter, r *http.Request, err error, what string) {
	switch {
	case clientGone(r):
	case errors.Is(err, contacts.ErrInvalidContact):
		httpx.Error(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, family.ErrPersonNotFound), errors.Is(err, contacts.ErrContactNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, contacts.ErrNotYours), errors.Is(err, contacts.ErrNotLinked):
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, auth.ErrBusy):
		busy(w)
	default:
		internalError(w, r, err, what)
	}
}

func contactID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return pathID(w, r, "cid", contacts.ErrContactNotFound)
}
