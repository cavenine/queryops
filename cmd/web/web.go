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
		RunE: func(cmd *cobra.Command, args []string) error {
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

	addr := fmt.Sprintf("%s:%s", config.Global.Host, config.Global.Port)
	slog.Info("server started", "addr", addr)
	defer slog.Info("server shutdown complete")

	if config.Global.AutoMigrate {
		if config.Global.DatabaseURL == "" {
			return fmt.Errorf("AUTO_MIGRATE is true but DATABASE_URL is empty")
		}
		slog.Info("running automatic database migrations")
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

	if config.Global.BackgroundProcessing && config.Global.Environment == config.Dev {
		clientCfg := background.DefaultClientConfig()
		eg.Go(func() error {
			slog.Info("starting in-process river workers")
			if err := background.RunWorker(egctx, pool, clientCfg); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("river client error: %w", err)
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
	sessionManager.Lifetime = 30 * 24 * time.Hour // 30 days
	sessionManager.Cookie.Name = "session"
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = config.Global.Environment == config.Prod
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	if err := router.SetupRoutes(egctx, r, sessionManager, pool); err != nil {
		return fmt.Errorf("error setting up routes: %w", err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
		BaseContext: func(l net.Listener) context.Context {
			return egctx
		},
		ErrorLog: slog.NewLogLogger(
			slog.Default().Handler(),
			slog.LevelError,
		),
	}

	eg.Go(func() error {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		<-egctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		slog.Debug("shutting down server...")

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("error during shutdown", "error", err)
			return err
		}

		return nil
	})

	return eg.Wait()
}
