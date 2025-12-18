package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/cavenine/queryops/config"
	accountFeature "github.com/cavenine/queryops/features/account"
	authFeature "github.com/cavenine/queryops/features/auth"
	counterFeature "github.com/cavenine/queryops/features/counter"
	indexFeature "github.com/cavenine/queryops/features/index"
	monitorFeature "github.com/cavenine/queryops/features/monitor"
	reverseFeature "github.com/cavenine/queryops/features/reverse"
	sortableFeature "github.com/cavenine/queryops/features/sortable"
	"github.com/cavenine/queryops/web/resources"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

func SetupRoutes(_ context.Context, router chi.Router, sessionManager *scs.SessionManager, pool *pgxpool.Pool) error {
	if config.Global.Environment == config.Dev {
		setupReload(router)
	}

	// Healthcheck for kamal-proxy readiness.
	router.Get("/up", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Static assets (public)
	router.Handle("/static/*", resources.Handler())

	// Initialize auth feature (creates services once)
	auth, err := authFeature.NewAuthFeature(sessionManager, pool)
	if err != nil {
		return fmt.Errorf("initializing auth feature: %w", err)
	}

	// Auth routes (public) - wrapped with LoadAndSave for session access
	router.Group(func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		auth.SetupPublicRoutes(r)
	})

	// Protected routes - require authentication
	var setupErr error
	router.Group(func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(authFeature.RequireAuth(auth.UserService(), sessionManager))

		auth.SetupProtectedRoutes(r)
		accountFeature.SetupRoutes(r, auth.CredentialRepo())

		if setupErr = errors.Join(
			indexFeature.SetupRoutes(r, sessionManager, pool),
			counterFeature.SetupRoutes(r, sessionManager),
			monitorFeature.SetupRoutes(r),
			sortableFeature.SetupRoutes(r),
			reverseFeature.SetupRoutes(r),
		); setupErr != nil {
			return
		}
	})

	if setupErr != nil {
		return fmt.Errorf("error setting up routes: %w", setupErr)
	}

	return nil
}

func setupReload(router chi.Router) {
	reloadChan := make(chan struct{}, 1)
	var hotReloadOnce sync.Once

	router.Get("/reload", func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		reload := func() {
			if err := sse.ExecuteScript("window.location.reload()"); err != nil {
				// We can't do much here as SSE might be closed, but we should satisfy errcheck
				return
			}
		}
		hotReloadOnce.Do(reload)
		select {
		case <-reloadChan:
			reload()
		case <-r.Context().Done():
		}
	})

	router.Get("/hotreload", func(w http.ResponseWriter, _ *http.Request) {
		select {
		case reloadChan <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}
