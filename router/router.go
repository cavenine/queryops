package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"queryops/config"
	accountFeature "queryops/features/account"
	authFeature "queryops/features/auth"
	counterFeature "queryops/features/counter"
	indexFeature "queryops/features/index"
	monitorFeature "queryops/features/monitor"
	reverseFeature "queryops/features/reverse"
	sortableFeature "queryops/features/sortable"
	"queryops/web/resources"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

func SetupRoutes(ctx context.Context, router chi.Router, sessionManager *scs.SessionManager, pool *pgxpool.Pool) (err error) {

	if config.Global.Environment == config.Dev {
		setupReload(router)
	}

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
	router.Group(func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(authFeature.RequireAuth(auth.UserService(), sessionManager))

		auth.SetupProtectedRoutes(r)
		accountFeature.SetupRoutes(r, auth.CredentialRepo())

		if err = errors.Join(
			indexFeature.SetupRoutes(r, sessionManager, pool),
			counterFeature.SetupRoutes(r, sessionManager),
			monitorFeature.SetupRoutes(r),
			sortableFeature.SetupRoutes(r),
			reverseFeature.SetupRoutes(r),
		); err != nil {
			return
		}
	})

	if err != nil {
		return fmt.Errorf("error setting up routes: %w", err)
	}

	return nil
}

func setupReload(router chi.Router) {
	reloadChan := make(chan struct{}, 1)
	var hotReloadOnce sync.Once

	router.Get("/reload", func(w http.ResponseWriter, r *http.Request) {
		sse := datastar.NewSSE(w, r)
		reload := func() { sse.ExecuteScript("window.location.reload()") }
		hotReloadOnce.Do(reload)
		select {
		case <-reloadChan:
			reload()
		case <-r.Context().Done():
		}
	})

	router.Get("/hotreload", func(w http.ResponseWriter, r *http.Request) {
		select {
		case reloadChan <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

}
