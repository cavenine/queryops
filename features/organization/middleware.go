package organization

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/cavenine/queryops/features/auth"
	"github.com/cavenine/queryops/features/organization/services"
	"github.com/google/uuid"
)

type contextKey string

const (
	organizationContextKey contextKey = "organization"
	userOrgsContextKey     contextKey = "user_organizations"
	activeOrgIDKey         string     = "active_organization_id"
)

func GetOrganizationFromContext(ctx context.Context) *services.Organization {
	org, ok := ctx.Value(organizationContextKey).(*services.Organization)
	if !ok {
		return nil
	}
	return org
}

func GetUserOrganizationsFromContext(ctx context.Context) []*services.Organization {
	orgs, ok := ctx.Value(userOrgsContextKey).([]*services.Organization)
	if !ok {
		return nil
	}
	return orgs
}

func LoadOrganizations(orgService *services.OrganizationService, sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				// Should be handled by RequireAuth, but safe fallback.
				next.ServeHTTP(w, r)
				return
			}

			userOrgs, err := orgService.GetUserOrganizations(r.Context(), user.ID)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), userOrgsContextKey, userOrgs)

			// Resolve active org from session (if valid membership).
			orgIDStr := sessionManager.GetString(r.Context(), activeOrgIDKey)
			if orgIDStr != "" {
				orgID, err := uuid.Parse(orgIDStr)
				if err == nil {
					for _, o := range userOrgs {
						if o.ID == orgID {
							ctx = context.WithValue(ctx, organizationContextKey, o)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}

			// No active org set; if the user has orgs, default to first.
			if len(userOrgs) > 0 {
				firstOrg := userOrgs[0]
				sessionManager.Put(r.Context(), activeOrgIDKey, firstOrg.ID.String())
				ctx = context.WithValue(ctx, organizationContextKey, firstOrg)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireOrganization(orgService *services.OrganizationService, sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			LoadOrganizations(orgService, sessionManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if GetOrganizationFromContext(r.Context()) == nil {
					http.Redirect(w, r, "/onboarding/create-org", http.StatusSeeOther)
					return
				}
				next.ServeHTTP(w, r)
			})).ServeHTTP(w, r)
		})
	}
}
