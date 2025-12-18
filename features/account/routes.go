package account

import (
	"github.com/cavenine/queryops/features/auth/services"

	"github.com/go-chi/chi/v5"
)

// SetupRoutes registers account routes.
// These routes require authentication and should be mounted in the protected group.
func SetupRoutes(router chi.Router, credentialRepo *services.CredentialRepository) {
	handlers := NewHandlers(credentialRepo)

	router.Get("/account", handlers.AccountPage)
	router.Delete("/account/passkey/{id}", handlers.DeletePasskey)
}
