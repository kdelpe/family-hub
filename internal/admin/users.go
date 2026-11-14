package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/mail"
)

const (
	commitTimeout = 10 * time.Second

	DefaultPageSize = 50
	MaxPageSize     = 100

	StatusActive   = "active"
	StatusInvited  = "invited"
	StatusDisabled = "disabled"
)

var (
	ErrInvalidPageSize = fmt.Errorf("limit must be between 1 and %d", MaxPageSize)
	ErrInvalidCursor   = errors.New("invalid cursor")
)

// Users is the admin's user management: inviting, listing and changing accounts.
type Users struct {
	pool    *pgxpool.Pool
	mailer  mail.Mailer
	baseURL string
}

func NewUsers(pool *pgxpool.Pool, mailer mail.Mailer, baseURL string) *Users {
	return &Users{pool: pool, mailer: mailer, baseURL: baseURL}
}

// Not canceled if the admin leaves: the server may already have the email, so the link must stay valid.
func (s *Users) send(ctx context.Context, msg mail.Message) error {
	err := s.mailer.Send(context.WithoutCancel(ctx), msg)
	switch {
	case errors.Is(err, mail.ErrInvalidMessage):
		return fmt.Errorf("build email: %w", err)
	case err != nil:
		return fmt.Errorf("%w: %w", ErrEmailNotSent, err)
	}
	return nil
}

func commit(ctx context.Context, tx pgx.Tx, what string) error {
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
	defer cancel()
	if err := tx.Commit(commitCtx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	return nil
}

type ListInput struct {
	Limit  int
	Cursor string
}

type Account struct {
	ID    uuid.UUID
	Email string
	Role  string
	// Active once a password is set or Google linked; invited until then.
	Status string
	// Only for an invited user: when the latest invite link expires, if there is one.
	InviteExpiresAt *time.Time
	DisabledAt      *time.Time
	LastLoginAt     *time.Time
	CreatedAt       time.Time
}

type Page struct {
	Accounts []Account
	// Empty on the last page.
	NextCursor string
}

// List pages through every user by email.
func (s *Users) List(ctx context.Context, in ListInput) (Page, error) {
	limit := in.Limit
	if limit < 1 || limit > MaxPageSize {
		return Page{}, ErrInvalidPageSize
	}
	afterEmail, afterID, err := decodeCursor(in.Cursor)
	if err != nil {
		return Page{}, err
	}
	// One extra row says whether there is a next page.
	rows, err := gen.New(s.pool).ListUsers(ctx, gen.ListUsersParams{AfterEmail: afterEmail, AfterID: afterID, PageSize: int32(limit + 1)})
	if err != nil {
		return Page{}, fmt.Errorf("list users: %w", err)
	}
	var page Page
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		page.NextCursor = encodeCursor(last.Email, last.ID)
	}
	page.Accounts = make([]Account, 0, len(rows))
	for _, row := range rows {
		page.Accounts = append(page.Accounts, accountFromRow(row))
	}
	return page, nil
}

func accountFromRow(row gen.ListUsersRow) Account {
	a := Account{
		ID: row.ID, Email: row.Email, Role: row.Role, Status: StatusActive,
		DisabledAt: row.DisabledAt, LastLoginAt: row.LastLoginAt, CreatedAt: row.CreatedAt,
	}
	switch {
	case row.DisabledAt != nil:
		a.Status = StatusDisabled
	case !row.SetUp:
		a.Status = StatusInvited
		if row.HasInvite {
			a.InviteExpiresAt = &row.InviteExpiresAt
		}
	}
	return a
}

// A valid email never holds a NUL, so it separates the two.
func encodeCursor(email string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(email + "\x00" + id.String()))
}

func decodeCursor(cursor string) (string, uuid.UUID, error) {
	if cursor == "" {
		return "", uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	email, idText, ok := strings.Cut(string(raw), "\x00")
	id, err := uuid.Parse(idText)
	// Postgres refuses text that isn't UTF-8, which would be a 500 instead of a 400.
	if !ok || email == "" || !utf8.ValidString(email) || err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return email, id, nil
}
