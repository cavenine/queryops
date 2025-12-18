package auth

import (
	"fmt"

	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/features/auth/services"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthFeature holds all auth-related services and handlers.
type AuthFeature struct {
	userService     *services.UserService
	webauthnService *services.WebAuthnService
	credentialRepo  *services.CredentialRepository
	handlers        *Handlers
	passkeyHandlers *PasskeyHandlers
}

// NewAuthFeature creates a new AuthFeature with all services initialized.
func NewAuthFeature(sessionManager *scs.SessionManager, pool *pgxpool.Pool) (*AuthFeature, error) {
	userRepo := services.NewUserRepository(pool)
	userService := services.NewUserService(userRepo)
	credentialRepo := services.NewCredentialRepository(pool)

	webauthnService, err := services.NewWebAuthnService(config.Global, credentialRepo, userRepo, sessionManager)
	if err != nil {
		return nil, fmt.Errorf("creating webauthn service: %w", err)
	}

	handlers := NewHandlers(userService, sessionManager)
	passkeyHandlers := NewPasskeyHandlers(webauthnService, userService, sessionManager)

	return &AuthFeature{
		userService:     userService,
		webauthnService: webauthnService,
		credentialRepo:  credentialRepo,
		handlers:        handlers,
		passkeyHandlers: passkeyHandlers,
	}, nil
}

// UserService returns the user service for use by other packages (e.g., middleware).
func (f *AuthFeature) UserService() *services.UserService {
	return f.userService
}

// CredentialRepo returns the credential repository for use by other packages (e.g., account feature).
func (f *AuthFeature) CredentialRepo() *services.CredentialRepository {
	return f.credentialRepo
}

// SetupPublicRoutes registers authentication routes that don't require authentication.
func (f *AuthFeature) SetupPublicRoutes(router chi.Router) {
	// Standard auth routes
	router.Get("/login", f.handlers.LoginPage)
	router.Post("/login", f.handlers.LoginSubmit)
	router.Get("/register", f.handlers.RegisterPage)
	router.Post("/register", f.handlers.RegisterSubmit)
	router.Post("/logout", f.handlers.Logout)

	// Public passkey login routes
	router.Post("/passkey/login/begin", f.passkeyHandlers.LoginBegin)
	router.Post("/passkey/login/finish", f.passkeyHandlers.LoginFinish)
}

// SetupProtectedRoutes registers authentication routes that require the user to be logged in.
func (f *AuthFeature) SetupProtectedRoutes(router chi.Router) {
	// Protected passkey registration routes (must be logged in to add a passkey)
	router.Post("/passkey/register/begin", f.passkeyHandlers.RegisterBegin)
	router.Post("/passkey/register/finish", f.passkeyHandlers.RegisterFinish)
}

// NewUserService creates a UserService for use by other packages.
// Deprecated: Use NewAuthFeature and AuthFeature.UserService() instead.
func NewUserService(pool *pgxpool.Pool) *services.UserService {
	userRepo := services.NewUserRepository(pool)
	return services.NewUserService(userRepo)
}
