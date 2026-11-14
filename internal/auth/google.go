package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/kdelpe/family-hub/internal/database/gen"
)

const (
	googleIssuer       = "https://accounts.google.com"
	googleFlowLifetime = 10 * time.Minute
)

var ErrGoogleAccess = errors.New("google sign-in is unavailable for this account")

type googleVerifier interface {
	Verify(context.Context, string) (*oidc.IDToken, error)
}

type GoogleAuth struct {
	pool     *pgxpool.Pool
	oauth    *oauth2.Config
	verifier googleVerifier
}

// NewGoogleAuth discovers Google's OIDC endpoints during startup and keeps the verifier for reuse.
func NewGoogleAuth(ctx context.Context, pool *pgxpool.Pool, clientID, clientSecret, baseURL string) (*GoogleAuth, error) {
	provider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover Google OIDC provider: %w", err)
	}
	oauthConfig := &oauth2.Config{
		ClientID: clientID, ClientSecret: clientSecret,
		Endpoint:    provider.Endpoint(),
		RedirectURL: strings.TrimRight(baseURL, "/") + "/api/auth/google/callback",
		Scopes:      []string{oidc.ScopeOpenID, oidc.ScopeEmail},
	}
	return &GoogleAuth{pool: pool, oauth: oauthConfig, verifier: provider.Verifier(&oidc.Config{ClientID: clientID})}, nil
}

func (g *GoogleAuth) Start(ctx context.Context) (string, string, error) {
	handle, rawHandle, err := secureRandomToken()
	if err != nil {
		return "", "", fmt.Errorf("generate OAuth flow handle: %w", err)
	}
	state, _, err := secureRandomToken()
	if err != nil {
		return "", "", fmt.Errorf("generate OAuth state: %w", err)
	}
	nonce, _, err := secureRandomToken()
	if err != nil {
		return "", "", fmt.Errorf("generate OAuth nonce: %w", err)
	}
	verifier, _, err := secureRandomToken()
	if err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	if err := gen.New(g.pool).CreateOAuthFlow(ctx, gen.CreateOAuthFlowParams{
		HandleHash: hashToken(rawHandle), State: state, Nonce: nonce,
		PkceVerifier: verifier, Lifetime: googleFlowLifetime,
	}); err != nil {
		return "", "", fmt.Errorf("create OAuth flow: %w", err)
	}
	authorizeURL := g.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce))
	return handle, authorizeURL, nil
}

type GoogleCallbackInput struct {
	Handle        string
	State         string
	Code          string
	UserAgent     string
	ReplacesToken string
}

func (g *GoogleAuth) Complete(ctx context.Context, in GoogleCallbackInput) (string, User, error) {
	handleBytes, ok := decodeToken(in.Handle)
	if !ok {
		return "", User{}, ErrGoogleAccess
	}
	flow, err := gen.New(g.pool).ConsumeOAuthFlow(ctx, hashToken(handleBytes))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", User{}, ErrGoogleAccess
	}
	if err != nil {
		return "", User{}, fmt.Errorf("consume OAuth flow: %w", err)
	}
	if !constantTimeEqual(flow.State, in.State) || in.Code == "" {
		return "", User{}, ErrGoogleAccess
	}
	token, err := g.oauth.Exchange(ctx, in.Code, oauth2.VerifierOption(flow.PkceVerifier))
	if err != nil {
		return "", User{}, ErrGoogleAccess
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", User{}, ErrGoogleAccess
	}
	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil || !constantTimeEqual(flow.Nonce, idTokenNonce(idToken)) {
		return "", User{}, ErrGoogleAccess
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		HostedDomain  string `json:"hd"`
	}
	if err := idToken.Claims(&claims); err != nil || idToken.Subject == "" {
		return "", User{}, ErrGoogleAccess
	}
	return g.signIn(ctx, idToken.Subject, NormalizeEmail(claims.Email), claims.EmailVerified, strings.ToLower(strings.TrimSpace(claims.HostedDomain)), in)
}

func (g *GoogleAuth) signIn(ctx context.Context, subject, email string, emailVerified bool, hostedDomain string, in GoogleCallbackInput) (string, User, error) {
	q := gen.New(g.pool)
	userID, err := q.GetGoogleUserBySub(ctx, &subject)
	if err == nil {
		return g.signInLinked(ctx, userID, subject, in)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", User{}, fmt.Errorf("find Google account: %w", err)
	}
	if !emailVerified || !googleOwnsEmail(email, hostedDomain) || !ValidEmail(email) {
		return "", User{}, ErrGoogleAccess
	}
	userID, err = q.GetInvitedGoogleUser(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", User{}, ErrGoogleAccess
	}
	if err != nil {
		return "", User{}, fmt.Errorf("find invited Google account: %w", err)
	}

	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin Google sign-in: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)
	if err := setGoogleLockTimeout(ctx, tx); err != nil {
		return "", User{}, err
	}
	locked, err := qtx.LockUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", User{}, ErrGoogleAccess
		}
		return "", User{}, fmt.Errorf("recheck invited Google account: %w", err)
	}
	if locked.DisabledAt != nil || locked.Email != email || locked.PasswordChangedAt != nil {
		return "", User{}, ErrGoogleAccess
	}
	if locked.GoogleSub != nil {
		if *locked.GoogleSub != subject {
			return "", User{}, ErrGoogleAccess
		}
		if err := qtx.RecordGoogleLogin(ctx, userID); err != nil {
			return "", User{}, fmt.Errorf("record Google login: %w", err)
		}
		sessionToken, err := g.createSession(ctx, tx, userID, in)
		if err != nil {
			return "", User{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", User{}, fmt.Errorf("commit Google sign-in: %w", err)
		}
		return sessionToken, User{ID: userID, Email: locked.Email, Role: locked.Role}, nil
	}
	if err := qtx.LinkGoogleAndRecordLogin(ctx, gen.LinkGoogleAndRecordLoginParams{GoogleSub: &subject, ID: userID}); err != nil {
		if isUniqueViolation(err) {
			return "", User{}, ErrGoogleAccess
		}
		return "", User{}, fmt.Errorf("link Google account: %w", err)
	}
	if err := qtx.DeleteUserTokens(ctx, userID); err != nil {
		return "", User{}, fmt.Errorf("delete invite tokens after Google link: %w", err)
	}
	sessionToken, err := g.createSession(ctx, tx, userID, in)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit Google link: %w", err)
	}
	return sessionToken, User{ID: userID, Email: locked.Email, Role: locked.Role}, nil
}

func (g *GoogleAuth) signInLinked(ctx context.Context, userID uuid.UUID, subject string, in GoogleCallbackInput) (string, User, error) {
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin Google sign-in: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)
	if err := setGoogleLockTimeout(ctx, tx); err != nil {
		return "", User{}, err
	}
	locked, err := qtx.LockUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (locked.DisabledAt != nil || locked.GoogleSub == nil || *locked.GoogleSub != subject)) {
		return "", User{}, ErrGoogleAccess
	}
	if err != nil {
		return "", User{}, fmt.Errorf("lock linked Google account: %w", err)
	}
	if err := qtx.RecordGoogleLogin(ctx, userID); err != nil {
		return "", User{}, fmt.Errorf("record Google login: %w", err)
	}
	sessionToken, err := g.createSession(ctx, tx, userID, in)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit Google sign-in: %w", err)
	}
	return sessionToken, User{ID: userID, Email: locked.Email, Role: locked.Role}, nil
}

func (g *GoogleAuth) createSession(ctx context.Context, tx pgx.Tx, userID uuid.UUID, in GoogleCallbackInput) (string, error) {
	sessions := NewSessions(tx)
	if err := sessions.Delete(ctx, in.ReplacesToken); err != nil {
		return "", err
	}
	token, err := sessions.Create(ctx, userID, in.UserAgent)
	if err != nil {
		return "", err
	}
	return token, nil
}

func secureRandomToken() (string, []byte, error) {
	raw, encoded, err := newToken()
	return encoded, raw, err
}

func constantTimeEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func idTokenNonce(token *oidc.IDToken) string {
	if token == nil {
		return ""
	}
	return token.Nonce
}

func googleOwnsEmail(email, hostedDomain string) bool {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return false
	}
	domain := email[at+1:]
	return domain == "gmail.com" || (hostedDomain != "" && hostedDomain == domain)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func setGoogleLockTimeout(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SELECT set_config('lock_timeout', $1, true)", fmt.Sprintf("%dms", signInLockTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("set Google sign-in lock timeout: %w", err)
	}
	return nil
}
