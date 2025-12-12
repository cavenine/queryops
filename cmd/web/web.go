package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"queryops/background"
	"queryops/config"
	"queryops/db"
	"queryops/migrations"
	"queryops/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := run(ctx); err != nil && err != http.ErrServerClosed {
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
			if err := background.RunWorker(egctx, pool, clientCfg); err != nil && err != context.Canceled {
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

	sessionStore := sessions.NewCookieStore([]byte(config.Global.SessionSecret))
	sessionStore.MaxAge(86400 * 30)
	sessionStore.Options.Path = "/"
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = false
	sessionStore.Options.SameSite = http.SameSiteLaxMode

	if err := router.SetupRoutes(egctx, r, sessionStore, pool); err != nil {
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
		if err != nil && err != http.ErrServerClosed {
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
