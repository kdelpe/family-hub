package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/kdelpe/family-hub/internal/admin"
	"github.com/kdelpe/family-hub/internal/auth"
	"github.com/kdelpe/family-hub/internal/config"
	"github.com/kdelpe/family-hub/internal/database"
	"github.com/kdelpe/family-hub/internal/handler"
	"github.com/kdelpe/family-hub/internal/httpx"
	"github.com/kdelpe/family-hub/internal/mail"
	"github.com/kdelpe/family-hub/internal/ratelimit"
	"github.com/kdelpe/family-hub/internal/router"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 2 * time.Minute
	// Under docker stop's default 10s grace before SIGKILL.
	shutdownTimeout = 8 * time.Second

	loginsPerIP         = 20
	loginsPerIPWindow   = 15 * time.Minute
	resetsPerIP         = 10
	resetsPerIPWindow   = time.Minute
	previewsPerIP       = 10
	previewsPerIPWindow = time.Minute
)

// Run serves until ctx is canceled: a clean stop even before the port opens, but not during
// AUTO_MIGRATE. The port opens only once the app can answer, so callers never hang on it.
func Run(ctx context.Context, cfg config.Config) error {
	if cfg.AutoMigrate {
		if err := database.MigrateUp(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}
	err := connectAndServe(ctx, cfg)
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		zerolog.Ctx(ctx).Info().Msg("stopped")
		return nil
	}
	return err
}

func connectAndServe(ctx context.Context, cfg config.Config) error {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	zerolog.Ctx(ctx).Info().Str("addr", ln.Addr().String()).Msg("listening")
	return serve(ctx, newServer(pool, cfg), ln, shutdownTimeout)
}

func newServer(pool *pgxpool.Pool, cfg config.Config) *http.Server {
	cookie := handler.NewSessionCookie(cfg.CookieSecure)
	routes := router.New(
		router.Handlers{
			Health: handler.NewHealth(pool),
			Auth:   handler.NewAuth(auth.NewService(pool, auth.NewHasher()), cookie),
			Admin:  handler.NewAdmin(admin.NewInvites(pool, mail.NewSMTP(cfg.SMTP), cfg.BaseURL)),
		},
		router.Middleware{
			RequireSession: handler.NewSessionGate(auth.NewSessions(pool), cookie).Require,
			LimitLogin:     httpx.LimitByIP(ratelimit.New(loginsPerIP, loginsPerIPWindow)),
			LimitReset:     httpx.LimitByIP(ratelimit.New(resetsPerIP, resetsPerIPWindow)),
			LimitPreview:   httpx.LimitByIP(ratelimit.New(previewsPerIP, previewsPerIPWindow)),
			TrustedProxy:   cfg.TrustedProxy,
		},
	)
	return &http.Server{
		Handler:           routes,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func serve(ctx context.Context, srv *http.Server, ln net.Listener, grace time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case err := <-served:
		// Serve leaves accepted connections open; closing them cancels those requests too.
		_ = srv.Close()
		return fmt.Errorf("http: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Closing the connections cancels the requests' contexts, so their queries give back pool connections.
		_ = srv.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http: %w", err)
	}
	zerolog.Ctx(ctx).Info().Msg("stopped")
	return nil
}
