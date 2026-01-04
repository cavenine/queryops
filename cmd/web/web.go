package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/cavenine/queryops/background"
	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/db"
	"github.com/cavenine/queryops/internal/pubsub"
	"github.com/cavenine/queryops/migrations"
	"github.com/cavenine/queryops/router"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func NewWebCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the web server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if err := run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	return cmd
}

func run(ctx context.Context) error {
	addr := net.JoinHostPort(config.Global.Host, config.Global.Port)
	slog.InfoContext(ctx, "server started", "addr", addr)
	defer slog.InfoContext(ctx, "server shutdown complete")

	if config.Global.AutoMigrate {
		if config.Global.DatabaseURL == "" {
			return errors.New("AUTO_MIGRATE is true but DATABASE_URL is empty")
		}
		slog.InfoContext(ctx, "running automatic database migrations")
		if err := migrations.Up(config.Global.DatabaseURL); err != nil {
			return fmt.Errorf("running automatic migrations: %w", err)
		}

		// Always run River migrations alongside application migrations.
		if err := background.MigrateRiver(ctx, config.Global); err != nil {
			return fmt.Errorf("running river migrations: %w", err)
		}
	}

	eg, egctx := errgroup.WithContext(ctx)

	pool, err := db.NewPool(egctx, config.Global)
	if err != nil {
		return fmt.Errorf("creating database pool: %w", err)
	}
	defer pool.Close()

	var ps *pubsub.PubSub
	if config.Global.PubSubEnabled {
		ps, err = pubsub.New(egctx, &pubsub.Config{
			NATSUrl: config.Global.NATSUrl,
		})
		if err != nil {
			slog.WarnContext(egctx, "pubsub initialization failed; SSE will use polling", "error", err)
			ps = nil
		} else {
			defer func() {
				if closeErr := ps.Close(); closeErr != nil {
					slog.WarnContext(egctx, "error closing pubsub", "error", closeErr)
				}
			}()
		}
	}

	if config.Global.BackgroundProcessing && config.Global.Environment == config.Dev {
		clientCfg := background.DefaultClientConfig()
		eg.Go(func() error {
			slog.InfoContext(egctx, "starting in-process river workers")
			if runErr := background.RunWorker(egctx, pool, clientCfg); runErr != nil && !errors.Is(runErr, context.Canceled) {
				return fmt.Errorf("river client error: %w", runErr)
			}
			return nil
		})
	}

	r := chi.NewMux()
	r.Use(
		middleware.Logger,
		middleware.Recoverer,
	)

	// Initialize SCS session manager with PostgreSQL backend
	sessionManager := scs.New()
	sessionManager.Store = pgxstore.New(pool)
	const sessionLifetime = 30 * 24 * time.Hour
	sessionManager.Lifetime = sessionLifetime
	sessionManager.Cookie.Name = "session"
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = config.Global.Environment == config.Prod
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	if setupErr := router.SetupRoutes(egctx, r, sessionManager, pool, ps); setupErr != nil {
		return fmt.Errorf("error setting up routes: %w", setupErr)
	}

	const readHeaderTimeout = 5 * time.Second
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
		BaseContext: func(_ net.Listener) context.Context {
			return egctx
		},
		ErrorLog: slog.NewLogLogger(
			slog.Default().Handler(),
			slog.LevelError,
		),
	}

	eg.Go(func() error {
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", serveErr)
		}
		return nil
	})

	eg.Go(func() error {
		<-egctx.Done()
		const shutdownTimeout = 5 * time.Second
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		slog.DebugContext(egctx, "shutting down server...")

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			slog.ErrorContext(egctx, "error during shutdown", "error", shutdownErr)
			return shutdownErr
		}

		return nil
	})

	return eg.Wait()
}
