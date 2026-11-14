package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	// Caps the lookups and SMTP sends a flood of addresses can run at once, and the work waiting for them.
	resetWorkers     = 4
	resetQueueLength = 32
	resetSendTimeout = time.Minute
	resetSubject     = "Reset your family hub password"
	resetBody        = `Bonjou,

Someone asked to reset the password for this email on the family hub. To choose a new password, open this link:
%s

It works for %d minutes. If you didn't ask, ignore this email: your password stays the same.
`
)

var errShuttingDown = errors.New("canceled for shutdown")

type resetJob struct {
	email  string
	logger *zerolog.Logger
}

// Resets sends password-reset emails in the background, so the answer never reveals who has an account.
type Resets struct {
	svc      *Service
	mailer   mail.Mailer
	baseURL  string
	perEmail *ratelimit.Limiter

	mu       sync.Mutex
	closed   bool
	queue    chan resetJob
	workers  sync.WaitGroup
	stopping chan struct{}
	stopOnce sync.Once
}

// NewResets starts the workers; Close stops them.
func NewResets(svc *Service, mailer mail.Mailer, baseURL string) *Resets {
	r := &Resets{
		svc: svc, mailer: mailer, baseURL: baseURL,
		perEmail: ratelimit.New(resetsPerEmail, resetsPerEmailWindow),
		queue:    make(chan resetJob, resetQueueLength),
		stopping: make(chan struct{}),
	}
	for range resetWorkers {
		r.workers.Go(func() {
			for job := range r.queue {
				r.run(job)
			}
		})
	}
	return r
}

// Request emails a reset link if the address belongs to an enabled user. It returns at once and
// reports nothing; the work runs on after the request ends.
func (r *Resets) Request(ctx context.Context, email string) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return
	}
	// Trimmed from a body of up to 1 MiB: a copy, so the limiter key doesn't keep the whole body alive.
	email = strings.Clone(email)
	// Taken for every address, known or not, so the limit can't tell them apart.
	if ok, _ := r.perEmail.Take(email); !ok {
		return
	}
	logger := zerolog.Ctx(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		r.perEmail.Refund(email)
		return
	}
	select {
	case r.queue <- resetJob{email: email, logger: logger}:
	default:
		// Nothing was sent, so the address keeps its allowance for a retry.
		r.perEmail.Refund(email)
		logger.Warn().Msg("password reset dropped: queue full")
	}
}

func (r *Resets) run(job resetJob) {
	// Outlives the request, keeping only its logger; Close or the timeout ends it.
	ctx, cancel := context.WithCancelCause(job.logger.WithContext(context.Background()))
	defer cancel(nil)
	ctx, cancelTimeout := context.WithTimeout(ctx, resetSendTimeout)
	defer cancelTimeout()
	select {
	case <-r.stopping:
		job.logger.Warn().Err(errShuttingDown).Msg("password reset dropped before it started")
		return
	default:
	}
	go func() {
		select {
		case <-r.stopping:
			cancel(errShuttingDown)
		case <-ctx.Done():
		}
	}()
	if err := r.send(ctx, job.email); err != nil {
		job.logger.Error().Err(err).AnErr("cause", context.Cause(ctx)).Msg("send password reset")
	}
}

// Close refuses new resets, gives the queued and running ones up to grace to finish, then cancels
// the rest and waits at most grace again.
func (r *Resets) Close(grace time.Duration) {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue)
	}
	r.mu.Unlock()
	done := make(chan struct{})
	go func() {
		r.workers.Wait()
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
	// The send context, so a hung connection can't outlast the timeout or shutdown.
	defer func() { _ = tx.Rollback(ctx) }()
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
	token, err := CreatePasswordReset(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}

	return r.mailer.Send(ctx, ResetMessage(r.baseURL, email, token))
}

func ResetMessage(baseURL, email, token string) mail.Message {
	body := fmt.Sprintf(resetBody, SetPasswordLink(baseURL, token), int(ResetLifetime.Minutes()))
	return mail.Message{To: email, Subject: resetSubject, Body: body}
}
