package organization

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/cavenine/queryops/features/auth"
	"github.com/cavenine/queryops/features/organization/pages"
	"github.com/cavenine/queryops/features/organization/services"
)

type Handlers struct {
	orgService     *services.OrganizationService
	sessionManager *scs.SessionManager
}

func NewHandlers(orgService *services.OrganizationService, sessionManager *scs.SessionManager) *Handlers {
	return &Handlers{
		orgService:     orgService,
		sessionManager: sessionManager,
	}
}

func (h *Handlers) CreateOrgPage(w http.ResponseWriter, r *http.Request) {
	if err := pages.CreatePage("").Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) CreateOrgSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderCreateError(w, r, "Invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.renderCreateError(w, r, "Organization name is required")
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	org, err := h.orgService.Create(r.Context(), name, user.ID)
	if err != nil {
		h.renderCreateError(w, r, err.Error())
		return
	}

	// Set active organization in session
	h.sessionManager.Put(r.Context(), "active_organization_id", org.ID.String())

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) renderCreateError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := pages.CreatePage(errorMsg).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
