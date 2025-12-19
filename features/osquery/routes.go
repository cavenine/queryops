package osquery

import (
	orgServices "github.com/cavenine/queryops/features/organization/services"
	"github.com/cavenine/queryops/features/osquery/services"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRoutes(router chi.Router, pool *pgxpool.Pool, orgService *orgServices.OrganizationService) {
	repo := services.NewHostRepository(pool)
	handlers := NewHandlers(repo, orgService)

	router.Route("/osquery", func(r chi.Router) {
		r.Post("/enroll", handlers.Enroll)
		r.Post("/config", handlers.Config)
		r.Post("/logger", handlers.Logger)
		r.Post("/distributed_read", handlers.DistributedRead)
		r.Post("/distributed_write", handlers.DistributedWrite)
	})
}

func SetupProtectedRoutes(router chi.Router, pool *pgxpool.Pool, orgService *orgServices.OrganizationService) {
	repo := services.NewHostRepository(pool)
	handlers := NewHandlers(repo, orgService)

	router.Get("/hosts", handlers.HostsPage)
	router.Get("/hosts/{id}", handlers.HostDetailsPage)
	router.Get("/hosts/{id}/results", handlers.HostResultsSSE)
	router.Post("/hosts/{id}/query", handlers.RunQuery)
}
