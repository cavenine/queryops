package organization

import (
	"github.com/alexedwards/scs/v2"
	"github.com/cavenine/queryops/features/organization/services"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Feature struct {
	service  *services.OrganizationService
	handlers *Handlers
}

func NewFeature(pool *pgxpool.Pool, sessionManager *scs.SessionManager) *Feature {
	repo := services.NewOrganizationRepository(pool)
	service := services.NewOrganizationService(repo)
	handlers := NewHandlers(service, sessionManager)

	return &Feature{
		service:  service,
		handlers: handlers,
	}
}

func (f *Feature) Service() *services.OrganizationService {
	return f.service
}

func (f *Feature) SetupOnboardingRoutes(r chi.Router) {
	r.Route("/onboarding", func(r chi.Router) {
		r.Get("/create-org", f.handlers.CreateOrgPage)
		r.Post("/create-org", f.handlers.CreateOrgSubmit)
	})

	// Organization switching routes (authenticated)
	r.Route("/organization", func(r chi.Router) {
		r.Post("/switch", f.handlers.SwitchOrganization)
	})
}
