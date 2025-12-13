package auth

import (
	"context"
	"net/http"

	"queryops/features/auth/services"

	"github.com/alexedwards/scs/v2"
)

type contextKey string

const (
	userContextKey contextKey = "user"
	userIDKey      string     = "user_id"
)

// GetUserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is authenticated.
func GetUserFromContext(ctx context.Context) *services.User {
	user, ok := ctx.Value(userContextKey).(*services.User)
	if !ok {
		return nil
	}
	return user
}

// SetUserInContext stores the user in the context.
func SetUserInContext(ctx context.Context, user *services.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// RequireAuth is middleware that ensures the user is authenticated.
// If not authenticated, redirects to /login.
func RequireAuth(userService *services.UserService, sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := sessionManager.GetInt(r.Context(), userIDKey)
			if userID == 0 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			user, err := userService.GetByID(r.Context(), userID)
			if err != nil || user == nil {
				// Invalid session, destroy it
				_ = sessionManager.Destroy(r.Context())
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Store user in context and continue
			ctx := SetUserInContext(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetSessionUserID stores the user ID in the session and regenerates the token.
func SetSessionUserID(ctx context.Context, sessionManager *scs.SessionManager, userID int) error {
	// Renew token to prevent session fixation attacks
	if err := sessionManager.RenewToken(ctx); err != nil {
		return err
	}
	sessionManager.Put(ctx, userIDKey, userID)
	return nil
}

// ClearSession destroys the current session.
func ClearSession(ctx context.Context, sessionManager *scs.SessionManager) error {
	return sessionManager.Destroy(ctx)
}
