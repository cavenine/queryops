package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"queryops/config"
	counterFeature "queryops/features/counter"
	indexFeature "queryops/features/index"
	monitorFeature "queryops/features/monitor"
	reverseFeature "queryops/features/reverse"
	sortableFeature "queryops/features/sortable"
	"queryops/web/resources"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
)

func SetupRoutes(ctx context.Context, router chi.Router, sessionStore *sessions.CookieStore, pool *pgxpool.Pool) (err error) {

	if config.Global.Environment == config.Dev {
		setupReload(router)
	}

	router.Handle("/static/*", resources.Handler())

	if err := errors.Join(
		indexFeature.SetupRoutes(router, sessionStore, pool),
		counterFeature.SetupRoutes(router, sessionStore),
		monitorFeature.SetupRoutes(router),
		sortableFeature.SetupRoutes(router),
		reverseFeature.SetupRoutes(router),
	); err != nil {
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
