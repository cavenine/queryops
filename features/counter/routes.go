package counter

import (
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(router chi.Router, sessionManager *scs.SessionManager) error {
	handlers := NewHandlers(sessionManager)

	router.Get("/counter", handlers.CounterPage)
	router.Get("/counter/data", handlers.CounterData)

	router.Route("/counter/increment", func(incrementRouter chi.Router) {
		incrementRouter.Post("/global", handlers.IncrementGlobal)
		incrementRouter.Post("/user", handlers.IncrementUser)
	})

	return nil
}
