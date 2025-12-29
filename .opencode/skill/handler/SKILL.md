---
name: handler
description: Create HTTP handlers with dependency injection, error handling, and proper response patterns
---

# Handler Pattern Skill

Create HTTP handlers following project conventions for dependency injection, error handling, and responses.

## When to Use

- Creating new HTTP handlers for a feature
- Adding handlers for page renders, API endpoints, or SSE streams
- Implementing form handling with validation
- Refactoring handlers for testability

## Handler Structure

```go
package feature

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/cavenine/queryops/features/<feature>/pages"
	"github.com/cavenine/queryops/features/<feature>/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers contains HTTP handlers for the feature.
type Handlers struct {
	service        *services.EntityService
	sessionManager *scs.SessionManager
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(service *services.EntityService, sessionManager *scs.SessionManager) *Handlers {
	return &Handlers{
		service:        service,
		sessionManager: sessionManager,
	}
}
```

## Page Handlers

### Basic Page Render

```go
func (h *Handlers) IndexPage(w http.ResponseWriter, r *http.Request) {
	if err := pages.IndexPage().Render(r.Context(), w); err != nil {
		slog.Error("failed to render index page", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

### Page with Data

```go
func (h *Handlers) ListPage(w http.ResponseWriter, r *http.Request) {
	entities, err := h.service.List(r.Context())
	if err != nil {
		slog.Error("failed to list entities", "error", err)
		http.Error(w, "Failed to load data", http.StatusInternalServerError)
		return
	}

	if err := pages.ListPage(entities).Render(r.Context(), w); err != nil {
		slog.Error("failed to render list page", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

### Detail Page with URL Parameter

```go
func (h *Handlers) DetailPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("failed to get entity", "error", err, "id", id)
		http.Error(w, "Failed to load data", http.StatusInternalServerError)
		return
	}
	if entity == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if err := pages.DetailPage(entity).Render(r.Context(), w); err != nil {
		slog.Error("failed to render detail page", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

## Form Handlers

### Create Form Page

```go
func (h *Handlers) CreatePage(w http.ResponseWriter, r *http.Request) {
	if err := pages.CreatePage("").Render(r.Context(), w); err != nil {
		slog.Error("failed to render create page", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

### Form Submission

```go
func (h *Handlers) CreateSubmit(w http.ResponseWriter, r *http.Request) {
	// Parse form
	if err := r.ParseForm(); err != nil {
		h.renderCreateError(w, r, "Invalid form data")
		return
	}

	// Extract and validate fields
	name := r.FormValue("name")
	if name == "" {
		h.renderCreateError(w, r, "Name is required")
		return
	}

	email := r.FormValue("email")
	if email == "" {
		h.renderCreateError(w, r, "Email is required")
		return
	}

	// Get authenticated user
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Perform action
	entity, err := h.service.Create(r.Context(), name, email, user.ID)
	if err != nil {
		slog.Error("failed to create entity", "error", err, "name", name)
		h.renderCreateError(w, r, err.Error())
		return
	}

	slog.Info("entity created", "id", entity.ID, "name", name, "user_id", user.ID)

	// Redirect to list or detail
	http.Redirect(w, r, "/entities/"+entity.ID.String(), http.StatusSeeOther)
}

func (h *Handlers) renderCreateError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := pages.CreatePage(errorMsg).Render(r.Context(), w); err != nil {
		slog.Error("failed to render create error page", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

### Edit Form

```go
func (h *Handlers) EditPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to load data", http.StatusInternalServerError)
		return
	}
	if entity == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if err := pages.EditPage(entity, "").Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) EditSubmit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderEditError(w, r, id, "Invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.renderEditError(w, r, id, "Name is required")
		return
	}

	err = h.service.Update(r.Context(), id, name)
	if err != nil {
		h.renderEditError(w, r, id, err.Error())
		return
	}

	http.Redirect(w, r, "/entities/"+id.String(), http.StatusSeeOther)
}
```

## Action Handlers

### Delete Handler

```go
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Check authorization
	user := auth.GetUserFromContext(r.Context())
	entity, _ := h.service.GetByID(r.Context(), id)
	if entity == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Perform delete
	if err := h.service.Delete(r.Context(), id); err != nil {
		slog.Error("failed to delete entity", "error", err, "id", id)
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	slog.Info("entity deleted", "id", id, "user_id", user.ID)

	// Redirect to list
	http.Redirect(w, r, "/entities", http.StatusSeeOther)
}
```

### Toggle/Action Handler

```go
func (h *Handlers) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = h.service.ToggleStatus(r.Context(), id)
	if err != nil {
		slog.Error("failed to toggle status", "error", err, "id", id)
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	// For HTMX/Datastar, redirect back or return partial
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}
```

## Dependency Injection Patterns

### Interface-Based Dependencies

For testability, define minimal interfaces:

```go
// Handler file
type entityService interface {
	List(ctx context.Context) ([]*services.Entity, error)
	GetByID(ctx context.Context, id uuid.UUID) (*services.Entity, error)
	Create(ctx context.Context, name string) (*services.Entity, error)
}

type Handlers struct {
	service entityService  // Interface, not concrete type
}
```

### Wiring in Routes

```go
// routes.go
func (f *Feature) SetupRoutes(r chi.Router) {
	// Concrete types wired here
	repo := services.NewEntityRepository(f.pool)
	service := services.NewEntityService(repo)
	handlers := NewHandlers(service, f.sessionManager)

	r.Get("/", handlers.ListPage)
	// ...
}
```

## Context Patterns

### Getting User from Context

```go
import "github.com/cavenine/queryops/features/auth"

func (h *Handlers) CreateSubmit(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	
	// Use user.ID, user.Email, etc.
}
```

### Getting Organization from Context

```go
import "github.com/cavenine/queryops/features/organization"

func (h *Handlers) ListPage(w http.ResponseWriter, r *http.Request) {
	org := organization.GetOrgFromContext(r.Context())
	if org == nil {
		http.Redirect(w, r, "/onboarding/create-org", http.StatusSeeOther)
		return
	}

	// Use org.ID for scoped queries
	entities, err := h.service.ListByOrg(r.Context(), org.ID)
	// ...
}
```

## Logging Patterns

```go
import "log/slog"

func (h *Handlers) CreateSubmit(w http.ResponseWriter, r *http.Request) {
	// Log with context fields
	slog.Info("creating entity",
		"name", name,
		"user_id", user.ID,
	)

	entity, err := h.service.Create(r.Context(), name)
	if err != nil {
		// Log errors with context
		slog.Error("failed to create entity",
			"error", err,
			"name", name,
			"user_id", user.ID,
		)
		h.renderCreateError(w, r, err.Error())
		return
	}

	slog.Info("entity created",
		"entity_id", entity.ID,
		"name", entity.Name,
		"user_id", user.ID,
	)
}
```

## Error Response Patterns

### HTML Error Pages

```go
func (h *Handlers) renderError(w http.ResponseWriter, r *http.Request, msg string, status int) {
	w.WriteHeader(status)
	if err := pages.ErrorPage(msg, status).Render(r.Context(), w); err != nil {
		http.Error(w, msg, status)
	}
}
```

### JSON Error Responses

```go
func (h *Handlers) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
```

## Handler Naming Conventions

| Pattern | Name | Example |
|---------|------|---------|
| List page | `ListPage` | `CampaignsListPage` |
| Detail page | `DetailPage` | `CampaignDetailPage` |
| Create form | `CreatePage` | `CampaignCreatePage` |
| Create submit | `CreateSubmit` | `CampaignCreateSubmit` |
| Edit form | `EditPage` | `CampaignEditPage` |
| Edit submit | `EditSubmit` | `CampaignEditSubmit` |
| Delete | `Delete` | `CampaignDelete` |
| API list | `List` | `ListCampaigns` |
| API get | `Get` | `GetCampaign` |
| API create | `Create` | `CreateCampaign` |
| SSE stream | `*SSE` | `CampaignResultsSSE` |

## Checklist

- [ ] Created Handlers struct with dependencies
- [ ] Created NewHandlers constructor
- [ ] Used interface types for testability (optional)
- [ ] Added proper error handling and logging
- [ ] Used `slog` for structured logging
- [ ] Extracted URL parameters with `chi.URLParam`
- [ ] Validated input before processing
- [ ] Got user/org from context where needed
- [ ] Used appropriate HTTP status codes
- [ ] Created helper methods for error rendering
- [ ] Wired handlers in routes.go
