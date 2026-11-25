package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/family"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/handler/mocks"
)

func TestHome(t *testing.T) {
	added := time.Date(2026, 11, 10, 15, 4, 5, 0, time.UTC)
	tests := []struct {
		name       string
		home       family.Home
		homeErr    error
		noSession  bool
		wantStatus int
		wantBody   string
	}{
		{name: "birthdays and newcomers", home: family.Home{
			Birthdays: []family.Birthday{{ID: testPersonID, FirstName: "Nadège", LastName: new("Pluviose"), On: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), InDays: 6}},
			Newcomers: []family.Newcomer{{ID: otherPersonID, FirstName: "Naomie", AddedAt: added}},
			Counts:    family.Counts{People: 26, Generations: 6, NewThisMonth: 3},
		}, wantStatus: http.StatusOK,
			wantBody: `{"birthdays":[{"id":"` + testPersonID.String() + `","first_name":"Nadège","last_name":"Pluviose","on":"2026-11-19","in_days":6}],
				"newcomers":[{"id":"` + otherPersonID.String() + `","first_name":"Naomie","last_name":null,"added_at":"2026-11-10T15:04:05Z"}],
				"counts":{"people":26,"generations":6,"new_this_month":3}}`},
		{name: "nothing coming up", wantStatus: http.StatusOK, wantBody: `{"birthdays":[],"newcomers":[],"counts":{"people":0,"generations":0,"new_this_month":0}}`},
		{name: "database error", homeErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantBody: internalBody},
		{name: "mounted without a session gate", noSession: true, wantStatus: http.StatusInternalServerError, wantBody: internalBody},
	}

	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewHomeService(t)
			show := handler.NewHome(svc, paris).Show
			req := httptest.NewRequest(http.MethodGet, "/api/home", nil)
			var rec *httptest.ResponseRecorder
			if tt.noSession {
				rec = serveAs(t, "", show, req)
			} else {
				svc.EXPECT().Home(mock.Anything, family.Viewer{UserID: testUser.ID, PersonID: &testPersonID}, mock.Anything, paris).Return(tt.home, tt.homeErr)
				rec = serveWith(t, auth.Session{UserID: testUser.ID, Role: auth.RoleMember, PersonID: &testPersonID}, show, req)
			}

			assertBody(t, rec, tt.wantStatus, tt.wantBody)
		})
	}
}
