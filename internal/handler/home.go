package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/httpx"
)

type HomeService interface {
	Home(ctx context.Context, viewer family.Viewer, now time.Time, loc *time.Location) (family.Home, error)
}

// Home serves the home page's cards (design-notes §4.1).
type Home struct {
	svc      HomeService
	timezone *time.Location
	now      func() time.Time
}

// NewHome answers in the family's timezone (APP_TIMEZONE).
func NewHome(svc HomeService, timezone *time.Location) *Home {
	if timezone == nil {
		panic("handler: NewHome needs the family's timezone")
	}
	return &Home{svc: svc, timezone: timezone, now: time.Now}
}

type homeResponse struct {
	Birthdays []birthdayResponse `json:"birthdays"`
	Newcomers []newcomerResponse `json:"newcomers"`
}

type birthdayResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  *string   `json:"last_name"`
	On        string    `json:"on"`
	InDays    int       `json:"in_days"`
}

type newcomerResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  *string   `json:"last_name"`
	AddedAt   time.Time `json:"added_at"`
}

func (h *Home) Show(w http.ResponseWriter, r *http.Request) {
	viewer, ok := currentViewer(w, r)
	if !ok {
		return
	}
	home, err := h.svc.Home(r.Context(), viewer, h.now(), h.timezone)
	if err != nil {
		internalError(w, r, err, "home")
		return
	}
	resp := homeResponse{Birthdays: make([]birthdayResponse, 0, len(home.Birthdays)), Newcomers: make([]newcomerResponse, 0, len(home.Newcomers))}
	for _, b := range home.Birthdays {
		resp.Birthdays = append(resp.Birthdays, birthdayResponse{ID: b.ID, FirstName: b.FirstName, LastName: b.LastName, On: b.On.Format(time.DateOnly), InDays: b.InDays})
	}
	for _, n := range home.Newcomers {
		resp.Newcomers = append(resp.Newcomers, newcomerResponse{ID: n.ID, FirstName: n.FirstName, LastName: n.LastName, AddedAt: n.AddedAt})
	}
	httpx.JSON(w, http.StatusOK, resp)
}
