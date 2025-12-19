package organization

import (
	"net/http"

	"github.com/cavenine/queryops/features/auth"
	"github.com/google/uuid"
)

func (h *Handlers) SwitchOrganization(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	orgIDStr := r.FormValue("org_id")
	if orgIDStr == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	targetOrgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "invalid organization id", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Verify membership
	// Since we don't have a direct IsMember check, we fetch user orgs.
	// This is acceptable for now.
	userOrgs, err := h.orgService.GetUserOrganizations(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	isMember := false
	for _, org := range userOrgs {
		if org.ID == targetOrgID {
			isMember = true
			break
		}
	}

	if !isMember {
		http.Error(w, "forbidden: not a member of this organization", http.StatusForbidden)
		return
	}

	// Update session
	h.sessionManager.Put(r.Context(), "active_organization_id", targetOrgID.String())

	// Redirect back to home (or referrer if we wanted to be fancy, but home is safe)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) CreateOrganizationAction(w http.ResponseWriter, r *http.Request) {
	// Re-using CreateOrgSubmit logic but tailored for action from sidebar/index
	// For now, CreateOrgSubmit handles the form submission which redirects to /.
	// If we want a separate endpoint for API usage, we can add it, but the existing one works.
	// This function is just a placeholder if needed later.
}
