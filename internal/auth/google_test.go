package auth

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

func TestGoogleSignInLinksOnlyInvitedAuthoritativeAccounts(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		verified     bool
		hostedDomain string
		invited      bool
		wantSuccess  bool
	}{
		{name: "verified Gmail invite", email: "person@gmail.com", verified: true, invited: true, wantSuccess: true},
		{name: "verified Workspace invite", email: "person@family.example.com", verified: true, hostedDomain: "family.example.com", invited: true, wantSuccess: true},
		{name: "unverified Gmail", email: "person@gmail.com", invited: true},
		{name: "non-Google mailbox without hosted domain", email: "person@example.com", verified: true, invited: true},
		{name: "hosted domain does not match email", email: "person@example.com", verified: true, hostedDomain: "other.example.com", invited: true},
		{name: "no invitation", email: "person@gmail.com", verified: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, pool, userID := withTestUser(t)
			if tt.invited {
				_, err := pool.Exec(t.Context(), `UPDATE users SET email = $1, password_hash = NULL, password_changed_at = NULL`, tt.email)
				require.NoError(t, err)
				_, err = insertToken(t, pool, userID, purposeInvite)
				require.NoError(t, err)
			}
			google := &GoogleAuth{pool: pool}
			token, user, err := google.signIn(t.Context(), "google-sub", tt.email, tt.verified, tt.hostedDomain, GoogleCallbackInput{UserAgent: "browser"})

			if !tt.wantSuccess {
				require.ErrorIs(t, err, ErrGoogleAccess)
				assert.Empty(t, token)
				assert.Empty(t, user.ID)
				assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM sessions`))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, userID, user.ID)
			assert.Equal(t, tt.email, user.Email)
			var googleSub string
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT google_sub FROM users WHERE id = $1`, userID).Scan(&googleSub))
			assert.Equal(t, "google-sub", googleSub)
			assert.Equal(t, 0, count(t, pool, `SELECT count(*) FROM auth_tokens`), "linking consumes outstanding emailed credentials")
			session, err := NewSessions(pool).Authenticate(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t, userID, session.UserID)
		})
	}
}

func TestGoogleSignInUsesExistingSubjectAndRejectsDisabledUsers(t *testing.T) {
	tests := []struct {
		name       string
		disabled   bool
		wantAccess bool
	}{
		{name: "linked account signs in"},
		{name: "disabled linked account is refused", disabled: true, wantAccess: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, pool, userID := withTestUser(t)
			setup := `UPDATE users SET google_sub = 'google-sub'`
			if tt.disabled {
				setup += `, disabled_at = now()`
			}
			_, err := pool.Exec(t.Context(), setup)
			require.NoError(t, err)

			google := &GoogleAuth{pool: pool}
			token, user, err := google.signIn(t.Context(), "google-sub", "irrelevant@example.net", false, "", GoogleCallbackInput{})
			if tt.wantAccess {
				require.ErrorIs(t, err, ErrGoogleAccess)
				assert.Empty(t, token)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, userID, user.ID)
			session, err := NewSessions(pool).Authenticate(t.Context(), token)
			require.NoError(t, err)
			assert.Equal(t, userID, session.UserID)
		})
	}
}

func TestOAuthFlowCanBeConsumedOnlyOnce(t *testing.T) {
	_, pool, _ := withTestUser(t)
	handle, raw, err := secureRandomToken()
	require.NoError(t, err)
	q := gen.New(pool)
	require.NoError(t, q.CreateOAuthFlow(t.Context(), gen.CreateOAuthFlowParams{
		HandleHash: hashToken(raw), State: "state", Nonce: "nonce", PkceVerifier: "verifier", Lifetime: googleFlowLifetime,
	}))

	flow, err := q.ConsumeOAuthFlow(t.Context(), hashToken(raw))
	require.NoError(t, err)
	assert.Equal(t, "state", flow.State)
	assert.Equal(t, "nonce", flow.Nonce)
	assert.Equal(t, "verifier", flow.PkceVerifier)
	_, err = q.ConsumeOAuthFlow(t.Context(), hashToken(raw))
	assert.Error(t, err)
	assert.NotEmpty(t, handle)
}

func TestGoogleStartStoresFlowSecretsAndWrongStateConsumesFlow(t *testing.T) {
	_, pool, _ := withTestUser(t)
	google := &GoogleAuth{pool: pool, oauth: &oauth2.Config{
		ClientID: "client-id",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.com/authorize", TokenURL: "https://accounts.example.com/token"},
	}}
	handle, destination, err := google.Start(t.Context())
	require.NoError(t, err)
	parsed, err := url.Parse(destination)
	require.NoError(t, err)
	params := parsed.Query()
	assert.Equal(t, "S256", params.Get("code_challenge_method"))
	assert.NotEmpty(t, params.Get("code_challenge"))
	assert.NotEmpty(t, params.Get("nonce"))
	assert.NotEmpty(t, params.Get("state"))

	rawHandle, ok := decodeToken(handle)
	require.True(t, ok)
	var state, nonce, verifier string
	require.NoError(t, pool.QueryRow(t.Context(), `SELECT state, nonce, pkce_verifier FROM oauth_flows WHERE handle_hash = $1`, hashToken(rawHandle)).Scan(&state, &nonce, &verifier))
	assert.Equal(t, params.Get("state"), state)
	assert.Equal(t, params.Get("nonce"), nonce)
	assert.Equal(t, params.Get("code_challenge"), oauth2.S256ChallengeFromVerifier(verifier))

	_, _, err = google.Complete(t.Context(), GoogleCallbackInput{Handle: handle, State: "wrong-state", Code: "code"})
	assert.ErrorIs(t, err, ErrGoogleAccess)
	_, _, err = google.Complete(t.Context(), GoogleCallbackInput{Handle: handle, State: state, Code: "code"})
	assert.ErrorIs(t, err, ErrGoogleAccess, "the mismatched callback consumed the one-time flow")
}

func TestGoogleOwnsEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		hd    string
		want  bool
	}{
		{name: "Gmail", email: "person@gmail.com", want: true},
		{name: "old Googlemail alias is not authoritative", email: "person@googlemail.com"},
		{name: "matching Workspace", email: "person@family.example.com", hd: "family.example.com", want: true},
		{name: "mismatched Workspace", email: "person@family.example.com", hd: "other.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, googleOwnsEmail(tt.email, tt.hd))
		})
	}
}
