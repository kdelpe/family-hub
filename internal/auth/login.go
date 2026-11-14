package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"golang.org/x/text/unicode/norm"

	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/ratelimit"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type User struct {
	ID    uuid.UUID
	Email string
	Role  string
}

const (
	signInLockTimeout = 5 * time.Second
	lockNotAvailable  = "55P03"

	loginFailuresPerEmailAndIP       = 5
	loginFailuresPerEmailAndIPWindow = 15 * time.Minute
	loginFailuresPerEmail            = 50
	loginFailuresPerEmailWindow      = time.Hour
	// RFC 5321's limits; longer can't be a real address, and would pin big limiter keys.
	maxEmailLen     = 254
	maxLocalPartLen = 64
	maxLabelLen     = 63
)

// The password was right but a credential changed meanwhile: refused, but not a failed guess.
var errChangedDuringLogin = errors.New("credentials changed during login")

type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return "too many failed sign-ins" }

type Service struct {
	pool        *pgxpool.Pool
	hasher      *Hasher
	lockTimeout time.Duration

	failuresByEmailAndIP *ratelimit.Limiter
	failuresByEmail      *ratelimit.Limiter
}

func NewService(pool *pgxpool.Pool, hasher *Hasher) *Service {
	return &Service{
		pool: pool, hasher: hasher, lockTimeout: signInLockTimeout,
		failuresByEmailAndIP: ratelimit.New(loginFailuresPerEmailAndIP, loginFailuresPerEmailAndIPWindow),
		failuresByEmail:      ratelimit.New(loginFailuresPerEmail, loginFailuresPerEmailWindow),
	}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFC.String(email)))
}

// ValidEmail reports whether a normalized email is a bare address, fit to store for a new user.
func ValidEmail(email string) bool {
	if len(email) > maxEmailLen {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	// RFC 5322 also allows hosts like "localhost" and "[127.0.0.1]", which no invite or reset mail would reach.
	at := strings.LastIndex(email, "@")
	return at <= maxLocalPartLen && printableASCII(email[:at]) && hostname(email[at+1:])
}

// Keeps out invisible and lookalike characters, which would make a second account that looks the same.
func printableASCII(s string) bool {
	for _, c := range []byte(s) {
		if c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

// A dotted DNS name: the only kind of domain an invite or reset mail can reach.
func hostname(domain string) bool {
	labels := strings.Split(domain, ".")
	// An all-digit top label makes it an IP address, not a name.
	if len(labels) < 2 || strings.Trim(labels[len(labels)-1], "0123456789") == "" {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > maxLabelLen || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range []byte(label) {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

type LoginInput struct {
	Email         string
	Password      string
	UserAgent     string
	ClientIP      string
	ReplacesToken string
}

// Login returns a new session token. Every refusal is ErrInvalidCredentials after one Argon2
// run, so the answer doesn't say why (timing differs only mid parameter migration, §5).
// A key over its failure limit gets *RateLimitedError at once, without hashing.
func (s *Service) Login(ctx context.Context, in LoginInput) (string, User, error) {
	email := NormalizeEmail(in.Email)
	// Postgres refuses NUL in text, which would turn this into a 500 instead of a refusal.
	if len(email) > maxEmailLen || strings.ContainsRune(email, 0) {
		return "", User{}, s.refuse(ctx, in.Password)
	}
	// Trimming can leave a short key that still holds the whole request body.
	email = strings.Clone(email)

	// Taken before the slow verify, so parallel guesses can't all pass an unspent limit.
	pair := email + "\x00" + in.ClientIP
	ok, wait := s.failuresByEmailAndIP.Take(pair)
	if !ok {
		return "", User{}, &RateLimitedError{RetryAfter: wait}
	}
	if ok, wait := s.failuresByEmail.Take(email); !ok {
		s.failuresByEmailAndIP.Refund(pair)
		return "", User{}, &RateLimitedError{RetryAfter: wait}
	}

	token, user, err := s.login(ctx, email, in)
	if !errors.Is(err, ErrInvalidCredentials) {
		s.failuresByEmailAndIP.Refund(pair)
		s.failuresByEmail.Refund(email)
	}
	if errors.Is(err, errChangedDuringLogin) {
		err = ErrInvalidCredentials
	}
	return token, user, err
}

func (s *Service) login(ctx context.Context, email string, in LoginInput) (string, User, error) {
	found, err := gen.New(s.pool).GetLoginUser(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, s.refuse(ctx, in.Password)
	case err != nil:
		return "", User{}, fmt.Errorf("find user: %w", err)
	case found.PasswordHash == nil || found.PasswordChangedAt == nil || found.DisabledAt != nil:
		return "", User{}, s.refuse(ctx, in.Password)
	}

	ok, rehash, err := s.hasher.Verify(ctx, *found.PasswordHash, in.Password)
	switch {
	case errors.Is(err, errMalformedHash):
		zerolog.Ctx(ctx).Warn().Stringer("user_id", found.ID).Msg("stored password hash is malformed")
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, err
	case !ok:
		return "", User{}, ErrInvalidCredentials
	}
	newHash := s.rehash(ctx, rehash, in.Password)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", User{}, fmt.Errorf("begin login: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	qtx := gen.New(tx)

	// A reset, email change or disable that committed since the verify wins. A concurrent
	// login's rehash keeps password_changed_at, so it doesn't refuse this one.
	locked, err := s.lockUser(ctx, tx, found.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", User{}, errChangedDuringLogin
	case err != nil:
		return "", User{}, err
	case locked.DisabledAt != nil || locked.Email != email ||
		locked.PasswordChangedAt == nil || !locked.PasswordChangedAt.Equal(*found.PasswordChangedAt):
		return "", User{}, errChangedDuringLogin
	}
	if err := qtx.RecordLogin(ctx, gen.RecordLoginParams{ID: found.ID, NewPasswordHash: newHash}); err != nil {
		return "", User{}, fmt.Errorf("record login: %w", err)
	}
	sessions := NewSessions(tx)
	if err := sessions.Delete(ctx, in.ReplacesToken); err != nil {
		return "", User{}, err
	}
	token, err := sessions.Create(ctx, found.ID, in.UserAgent)
	if err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", User{}, fmt.Errorf("commit login: %w", err)
	}
	return token, User{ID: found.ID, Email: locked.Email, Role: locked.Role}, nil
}

// lockUser takes the row lock that every credential change and new session waits on (§5),
// giving up with ErrBusy after lockTimeout.
func (s *Service) lockUser(ctx context.Context, tx pgx.Tx, id uuid.UUID) (gen.LockUserForSignInRow, error) {
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout', $1, true)`, fmt.Sprintf("%dms", s.lockTimeout.Milliseconds())); err != nil {
		return gen.LockUserForSignInRow{}, fmt.Errorf("set lock timeout: %w", err)
	}
	locked, err := gen.New(tx).LockUserForSignIn(ctx, id)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable:
		return gen.LockUserForSignInRow{}, ErrBusy
	case err != nil:
		return gen.LockUserForSignInRow{}, fmt.Errorf("lock user: %w", err)
	}
	return locked, nil
}

// A failed rehash only means the old parameters stay until the next sign-in.
func (s *Service) rehash(ctx context.Context, needed bool, password string) *string {
	if !needed {
		return nil
	}
	h, err := s.hasher.Hash(ctx, password)
	if err != nil {
		if !errors.Is(ctx.Err(), context.Canceled) {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("rehash password")
		}
		return nil
	}
	return &h
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return NewSessions(s.pool).Delete(ctx, token)
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := gen.New(s.pool).GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return User{ID: u.ID, Email: u.Email, Role: u.Role}, nil
}

func (s *Service) refuse(ctx context.Context, password string) error {
	if err := s.hasher.verifyNothing(ctx, password); err != nil {
		return err
	}
	return ErrInvalidCredentials
}
