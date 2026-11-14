package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/database/gen"
	"github.com/kdelpe/family-hub/internal/mail"
	"github.com/kdelpe/family-hub/internal/ratelimit"
)

const (
	resetsPerEmail       = 5
	resetsPerEmailWindow = time.Hour
	// Caps the lookups and SMTP sends a flood of addresses can run at once.
	maxResetsSending = 4
	resetSlotWait    = time.Minute
	// Starts once a slot is free, so time spent waiting never shortens the send.
	resetSendTimeout = time.Minute
	resetSubject     = "Reset your family hub password"
	resetBody        = `Bonjou,

Someone asked to reset the password for this email on the family hub. To choose a new password, open this link:
%s

It works for %d minutes. If you didn't ask, ignore this email: your password stays the same.
`
)

var errShuttingDown = errors.New("canceled for shutdown")

// Resets sends password-reset emails in the background, so the answer never reveals who has an account.
type Resets struct {
	svc      *Service
	mailer   mail.Mailer
	baseURL  string
	perEmail *ratelimit.Limiter
	slots    chan struct{}

	mu       sync.Mutex
	closed   bool
	sending  sync.WaitGroup
	stopping chan struct{}
	stopOnce sync.Once
}

func NewResets(svc *Service, mailer mail.Mailer, baseURL string) *Resets {
	return &Resets{
		svc: svc, mailer: mailer, baseURL: baseURL,
		perEmail: ratelimit.New(resetsPerEmail, resetsPerEmailWindow),
		slots:    make(chan struct{}, maxResetsSending),
		stopping: make(chan struct{}),
	}
}

// Request emails a reset link if the address belongs to an enabled user. It returns at once and
// reports nothing; the work runs on after the request ends.
func (r *Resets) Request(ctx context.Context, email string) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return
	}
	// Taken for every address, known or not, so the limit can't tell them apart.
	if ok, _ := r.perEmail.Take(email); !ok {
		return
	}
	logger := zerolog.Ctx(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.sending.Go(func() {
		// Outlives the request, keeping only its logger; Close or the timeout ends it.
		ctx, cancel := context.WithCancelCause(logger.WithContext(context.Background()))
		defer cancel(nil)
		go func() {
			select {
			case <-r.stopping:
				cancel(errShuttingDown)
			case <-ctx.Done():
			}
		}()
		// Waits rather than drops, so a burst can delay a real reset but not lose it.
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		case <-ctx.Done():
			logger.Warn().Err(context.Cause(ctx)).Msg("password reset dropped before it started")
			return
		case <-time.After(resetSlotWait):
			logger.Warn().Msg("password reset dropped: no free slot in time")
			return
		}
		sendCtx, cancelSend := context.WithTimeout(ctx, resetSendTimeout)
		defer cancelSend()
		if err := r.send(sendCtx, email); err != nil {
			logger.Error().Err(err).AnErr("cause", context.Cause(sendCtx)).Msg("send password reset")
		}
	})
}

// Close refuses new resets, gives those running up to grace to finish, then cancels the rest
// and waits at most grace again.
func (r *Resets) Close(grace time.Duration) {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	done := make(chan struct{})
	go func() {
		r.sending.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(grace):
	}
	r.stopOnce.Do(func() { close(r.stopping) })
	select {
	case <-done:
	case <-time.After(grace):
	}
}

func (r *Resets) send(ctx context.Context, email string) error {
	tx, err := r.svc.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := r.svc.setLockTimeout(ctx, tx); err != nil {
		return err
	}
	// Locked, so a disable or email change can't land between this check and the new token.
	userID, err := gen.New(tx).LockEnabledUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	token, err := createToken(ctx, tx, userID, purposePasswordReset, ResetLifetime)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}

	minutes := int(ResetLifetime.Minutes())
	body := fmt.Sprintf(resetBody, SetPasswordLink(r.baseURL, token), minutes)
	return r.mailer.Send(ctx, mail.Message{To: email, Subject: resetSubject, Body: body})
}
