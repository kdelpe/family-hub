package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/mail"
	mailmocks "github.com/kdelpe/family-hub/internal/mail/mocks"
	"github.com/kdelpe/family-hub/internal/testdb"
)

const testBaseURL = "https://family.example.com"

func TestInvite(t *testing.T) {
	tests := []struct {
		name             string
		in               InviteInput
		existing         string
		sendErr          error
		wantSend         bool
		wantErr          error
		cancelDuringSend bool
		// person is who the invite names: a new person unless "none", "unknown", "deleted" or "taken".
		person   string
		wantUser auth.User
	}{
		{name: "a member", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "an admin", in: InviteInput{Email: testEmail, Role: auth.RoleAdmin}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleAdmin}},
		{name: "normalizes the email", in: InviteInput{Email: "  Test@Example.COM ", Role: auth.RoleMember}, wantSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "sent and committed even if the admin leaves during the send", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, cancelDuringSend: true, wantUser: auth.User{Email: testEmail, Role: auth.RoleMember}},
		{name: "invalid email", in: InviteInput{Email: "Test <test@example.com>", Role: auth.RoleMember}, wantErr: ErrInvalidEmail},
		{name: "unknown role", in: InviteInput{Email: testEmail, Role: "owner"}, wantErr: ErrInvalidRole},
		{name: "no role", in: InviteInput{Email: testEmail}, wantErr: ErrInvalidRole},
		{name: "email taken", in: InviteInput{Email: "TEST@example.com", Role: auth.RoleMember}, existing: testEmail, wantErr: ErrEmailTaken},
		{name: "no person", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, person: "none", wantErr: ErrNoPerson},
		{name: "unknown person", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, person: "unknown", wantErr: ErrPersonNotFound},
		{name: "deleted person", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, person: "deleted", wantErr: ErrPersonNotFound},
		{name: "a person who already has a login", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, existing: "other@example.com", person: "taken", wantErr: ErrPersonTaken},
		{name: "email not sent saves nothing", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, sendErr: errors.New("smtp: 550"), wantErr: ErrEmailNotSent},
		{name: "a refused message is not a send failure", in: InviteInput{Email: testEmail, Role: auth.RoleMember}, wantSend: true, sendErr: fmt.Errorf("%w: subject", mail.ErrInvalidMessage), wantErr: mail.ErrInvalidMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			person := insertPerson(t, pool)
			tt.in.SentBy = insertUser(t, pool, "admin@example.com")
			if tt.existing != "" {
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email) VALUES ($1)`, tt.existing)
				require.NoError(t, err)
			}
			switch tt.person {
			case "":
				tt.in.PersonID = person
				tt.wantUser.PersonID = &person
			case "unknown":
				tt.in.PersonID = uuid.New()
			case "deleted":
				_, err := pool.Exec(t.Context(), `UPDATE persons SET deleted_at = now() WHERE id = $1`, person)
				require.NoError(t, err)
				tt.in.PersonID = person
			case "taken":
				_, err := pool.Exec(t.Context(), `UPDATE users SET person_id = $1 WHERE email = $2`, person, tt.existing)
				require.NoError(t, err)
				tt.in.PersonID = person
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantSend {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(sendCtx context.Context, msg mail.Message) {
					sent = msg
					if tt.cancelDuringSend {
						cancel()
						assert.NoError(t, sendCtx.Err(), "the send isn't cut short")
					}
				}).Return(tt.sendErr).Once()
			}

			user, err := NewUsers(pool, mailer, testBaseURL).Invite(ctx, tt.in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				if errors.Is(tt.wantErr, mail.ErrInvalidMessage) {
					assert.NotErrorIs(t, err, ErrEmailNotSent)
				}
				var newUsers, tokens int
				require.NoError(t, pool.QueryRow(t.Context(),
					`SELECT (SELECT count(*) FROM users WHERE email NOT IN ($1, 'admin@example.com')), (SELECT count(*) FROM auth_tokens)`, tt.existing).Scan(&newUsers, &tokens))
				assert.Zero(t, newUsers, "no user left behind")
				assert.Zero(t, tokens)
				return
			}
			require.NoError(t, err)
			assert.NotZero(t, user.ID)
			tt.wantUser.ID = user.ID
			assert.Equal(t, tt.wantUser, user)
			assert.Equal(t, testEmail, sent.To)
			assert.Equal(t, "You're invited to the family hub", sent.Subject)
			assert.Contains(t, sent.Body, "Bonjou Rosa,")
			assert.Contains(t, sent.Body, "This link works for 7 days.")
			token := inviteToken(t, sent.Body)

			_, signedIn, err := auth.NewService(pool, auth.NewHasher()).SetPasswordWithToken(t.Context(), auth.SetPasswordInput{Token: token, Password: testPassword})

			require.NoError(t, err, "the emailed link sets the password")
			assert.Equal(t, user, signedIn)
		})
	}
}

func TestInviteFrom(t *testing.T) {
	const (
		fromAdmin  = "Maya Pluviose invited you to the Pluviose-Monestime Family Hub — a private place for our family's tree, photos and stories."
		fromNobody = "You're invited to the Pluviose-Monestime Family Hub — a private place for our family's tree, photos and stories."
		fromMember = "Stanley Jean-Louis invited you to the Pluviose-Monestime Family Hub — a private place for our family's tree, photos and stories."
		approved   = "Approved by Maya Pluviose, admin."
	)
	tests := []struct {
		name string
		// adminLinked says whether the sending admin is someone in the tree.
		adminLinked bool
		// from names who the invite is from: "", "Stanley", or someone who can't send one.
		from        string
		wantErr     error
		wantSubject string
		wantIntro   string
		wantFrom    string
	}{
		{name: "from the sending admin by default", adminLinked: true, wantSubject: "Maya invited you to the family hub",
			wantIntro: fromAdmin + "\n\n", wantFrom: "Maya"},
		{name: "from a member, approved by the admin", adminLinked: true, from: "Stanley", wantSubject: "Stanley invited you to the family hub",
			wantIntro: fromMember + "\n" + approved + "\n\n", wantFrom: "Stanley"},
		{name: "from nobody when the admin isn't in the tree", wantSubject: "You're invited to the family hub", wantIntro: fromNobody + "\n\n"},
		{name: "from a member, with no admin to name", from: "Stanley", wantSubject: "Stanley invited you to the family hub",
			wantIntro: fromMember + "\n\n", wantFrom: "Stanley"},
		{name: "not from someone without a login", adminLinked: true, from: "No login", wantErr: ErrInvalidSender},
		{name: "not from a disabled login", adminLinked: true, from: "Disabled", wantErr: ErrInvalidSender},
		{name: "not from a login that was never set up", adminLinked: true, from: "Invited", wantErr: ErrInvalidSender},
		{name: "not from someone unknown", adminLinked: true, from: "unknown", wantErr: ErrInvalidSender},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.MigratedPool(t, database.MigrateUp)
			people := map[string]uuid.UUID{"unknown": uuid.New()}
			for _, p := range []struct{ first, last, email, login string }{
				{"Maya", "Pluviose", "admin@example.com", "admin"},
				{"Stanley", "Jean-Louis", "stanley@example.com", "active"},
				{"Disabled", "", "disabled@example.com", "disabled"},
				{"Invited", "", "invited@example.com", "invited"},
				{"No login", "", "", ""},
				{"Kervens", "", "", ""},
			} {
				var id uuid.UUID
				require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES ($1, nullif($2, '')) RETURNING id`, p.first, p.last).Scan(&id))
				people[p.first] = id
				if p.email == "" {
					continue
				}
				_, err := pool.Exec(t.Context(), `INSERT INTO users (email, role, person_id, password_hash, password_changed_at, disabled_at)
					VALUES ($1, CASE WHEN $2 = 'admin' THEN 'admin' ELSE 'member' END, $3,
					  CASE WHEN $2 <> 'invited' THEN 'hash' END, CASE WHEN $2 <> 'invited' THEN now() END, CASE WHEN $2 = 'disabled' THEN now() END)`,
					p.email, p.login, id)
				require.NoError(t, err)
			}
			admin := userID(t, pool, "admin@example.com")
			if !tt.adminLinked {
				_, err := pool.Exec(t.Context(), `UPDATE users SET person_id = NULL WHERE id = $1`, admin)
				require.NoError(t, err)
			}
			in := InviteInput{Email: testEmail, Role: auth.RoleMember, PersonID: people["Kervens"], SentBy: admin}
			if tt.from != "" {
				from := people[tt.from]
				in.FromPersonID = &from
			}
			mailer := mailmocks.NewMailer(t)
			var sent mail.Message
			if tt.wantErr == nil {
				mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) { sent = msg }).Return(nil).Once()
			}

			user, err := NewUsers(pool, mailer, testBaseURL).Invite(t.Context(), in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				var invited int
				require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE email = $1`, testEmail).Scan(&invited))
				assert.Zero(t, invited, "nobody invited")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSubject, sent.Subject)
			assert.True(t, strings.HasPrefix(sent.Body, "Bonjou Kervens,\n\n"+tt.wantIntro+"Set up my account:\n"), sent.Body)
			var from *uuid.UUID
			var sentBy uuid.UUID
			require.NoError(t, pool.QueryRow(t.Context(), `SELECT from_person_id, sent_by_user_id FROM auth_tokens WHERE user_id = $1`, user.ID).Scan(&from, &sentBy))
			assert.Equal(t, admin, sentBy, "the record keeps who sent it")
			if tt.wantFrom == "" {
				assert.Nil(t, from)
			} else {
				assert.Equal(t, new(people[tt.wantFrom]), from)
			}
		})
	}
}

func TestInviteFromANameSavedWithALineBreak(t *testing.T) {
	pool := testdb.MigratedPool(t, database.MigrateUp)
	var maya, kervens uuid.UUID
	// Written straight to the table, as a name saved before line breaks were refused.
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name, last_name) VALUES (E'Maya\r\nAnne', 'Pluviose') RETURNING id`).Scan(&maya))
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name) VALUES ('Kervens') RETURNING id`).Scan(&kervens))
	var admin uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO users (email, role, person_id, password_hash, password_changed_at)
		VALUES ('admin@example.com', 'admin', $1, 'hash', now()) RETURNING id`, maya).Scan(&admin))
	mailer := mailmocks.NewMailer(t)
	var sent mail.Message
	mailer.EXPECT().Send(mock.Anything, mock.Anything).Run(func(_ context.Context, msg mail.Message) { sent = msg }).Return(nil).Once()

	_, err := NewUsers(pool, mailer, testBaseURL).Invite(t.Context(), InviteInput{Email: testEmail, Role: auth.RoleMember, PersonID: kervens, SentBy: admin})

	require.NoError(t, err)
	assert.Equal(t, "Maya Anne invited you to the family hub", sent.Subject)
	assert.Contains(t, sent.Body, "Maya Anne Pluviose invited you")
}

func insertPerson(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(t.Context(), `INSERT INTO persons (first_name) VALUES ('Rosa') RETURNING id`).Scan(&id))
	return id
}

func inviteToken(t *testing.T, body string) string {
	t.Helper()
	prefix := testBaseURL + auth.SetPasswordPath
	for line := range strings.Lines(body) {
		if token, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return token
		}
	}
	t.Fatalf("no %s link in the email:\n%s", prefix, body)
	return ""
}
