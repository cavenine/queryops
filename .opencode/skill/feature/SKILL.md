---
name: feature
description: Scaffold a new feature module with handlers, routes, pages, services, and repository following project conventions
---

# Feature Scaffolding Skill

Create new feature modules following the project's feature-based modular architecture.

## When to Use

- Adding a new domain/feature area (e.g., "campaigns", "reports", "notifications")
- Creating a self-contained module with its own routes, handlers, and data access
- Following the established patterns for consistency

## Project Architecture

```
features/
├── <feature>/
│   ├── handlers.go          # HTTP request handlers
│   ├── routes.go            # Chi router setup + Feature struct
│   ├── pages/               # Templ page templates
│   │   ├── <page>.templ
│   │   └── <page>_templ.go  # Generated
│   ├── components/          # Templ component templates (optional)
│   │   └── <component>.templ
│   └── services/            # Business logic + data access
│       ├── <entity>_service.go
│       └── <entity>_repository.go
```

## Step-by-Step Workflow

### 1. Create Directory Structure

```bash
mkdir -p features/<feature>/pages features/<feature>/services
```

### 2. Create Repository (Data Access Layer)

**File**: `features/<feature>/services/<entity>_repository.go`

```go
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entity represents a <entity> in the system.
type Entity struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EntityRepository handles database operations for entities.
type EntityRepository struct {
	pool *pgxpool.Pool
}

// NewEntityRepository creates a new EntityRepository.
func NewEntityRepository(pool *pgxpool.Pool) *EntityRepository {
	return &EntityRepository{pool: pool}
}

// Create inserts a new entity.
func (r *EntityRepository) Create(ctx context.Context, name string) (*Entity, error) {
	entity := &Entity{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO entities (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`, name).Scan(&entity.ID, &entity.Name, &entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating entity: %w", err)
	}
	return entity, nil
}

// GetByID retrieves an entity by ID.
func (r *EntityRepository) GetByID(ctx context.Context, id uuid.UUID) (*Entity, error) {
	entity := &Entity{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, created_at, updated_at
		FROM entities
		WHERE id = $1
	`, id).Scan(&entity.ID, &entity.Name, &entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("getting entity by id: %w", err)
	}
	return entity, nil
}

// List retrieves all entities.
func (r *EntityRepository) List(ctx context.Context) ([]*Entity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, created_at, updated_at
		FROM entities
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing entities: %w", err)
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		entity := &Entity{}
		if err := rows.Scan(&entity.ID, &entity.Name, &entity.CreatedAt, &entity.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning entity: %w", err)
		}
		entities = append(entities, entity)
	}
	return entities, nil
}
```

### 3. Create Service (Business Logic Layer)

**File**: `features/<feature>/services/<entity>_service.go`

```go
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// EntityService provides business logic for entities.
type EntityService struct {
	repo *EntityRepository
}

// NewEntityService creates a new EntityService.
func NewEntityService(repo *EntityRepository) *EntityService {
	return &EntityService{repo: repo}
}

// Create creates a new entity with validation.
func (s *EntityService) Create(ctx context.Context, name string) (*Entity, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return s.repo.Create(ctx, name)
}

// GetByID retrieves an entity by ID.
func (s *EntityService) GetByID(ctx context.Context, id uuid.UUID) (*Entity, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all entities.
func (s *EntityService) List(ctx context.Context) ([]*Entity, error) {
	return s.repo.List(ctx)
}
```

### 4. Create Handlers

**File**: `features/<feature>/handlers.go`

```go
package feature

import (
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

// ListPage renders the entity list page.
func (h *Handlers) ListPage(w http.ResponseWriter, r *http.Request) {
	entities, err := h.service.List(r.Context())
	if err != nil {
		http.Error(w, "Failed to load entities", http.StatusInternalServerError)
		return
	}

	if err := pages.ListPage(entities).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// DetailPage renders a single entity detail page.
func (h *Handlers) DetailPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	entity, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to load entity", http.StatusInternalServerError)
		return
	}
	if entity == nil {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	if err := pages.DetailPage(entity).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// CreatePage renders the create form.
func (h *Handlers) CreatePage(w http.ResponseWriter, r *http.Request) {
	if err := pages.CreatePage("").Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// CreateSubmit handles form submission.
func (h *Handlers) CreateSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderCreateError(w, r, "Invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.renderCreateError(w, r, "Name is required")
		return
	}

	_, err := h.service.Create(r.Context(), name)
	if err != nil {
		h.renderCreateError(w, r, err.Error())
		return
	}

	http.Redirect(w, r, "/<feature>", http.StatusSeeOther)
}

func (h *Handlers) renderCreateError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := pages.CreatePage(errorMsg).Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
```

### 5. Create Routes (Feature Entry Point)

**File**: `features/<feature>/routes.go`

```go
package feature

import (
	"github.com/alexedwards/scs/v2"
	"github.com/cavenine/queryops/features/<feature>/services"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Feature encapsulates the feature module.
type Feature struct {
	service  *services.EntityService
	handlers *Handlers
}

// NewFeature creates a new Feature instance with all dependencies.
func NewFeature(pool *pgxpool.Pool, sessionManager *scs.SessionManager) *Feature {
	repo := services.NewEntityRepository(pool)
	service := services.NewEntityService(repo)
	handlers := NewHandlers(service, sessionManager)

	return &Feature{
		service:  service,
		handlers: handlers,
	}
}

// Service returns the feature's service for use by other features.
func (f *Feature) Service() *services.EntityService {
	return f.service
}

// SetupRoutes configures the feature's routes.
func (f *Feature) SetupRoutes(r chi.Router) {
	r.Route("/<feature>", func(r chi.Router) {
		r.Get("/", f.handlers.ListPage)
		r.Get("/new", f.handlers.CreatePage)
		r.Post("/new", f.handlers.CreateSubmit)
		r.Get("/{id}", f.handlers.DetailPage)
	})
}
```

### 6. Create Page Templates

**File**: `features/<feature>/pages/list.templ`

```templ
package pages

import (
	"github.com/cavenine/queryops/features/common/layouts"
	"github.com/cavenine/queryops/features/<feature>/services"
)

templ ListPage(entities []*services.Entity) {
	@layouts.Base("Entities") {
		<div class="container mx-auto p-4">
			<div class="flex justify-between items-center mb-4">
				<h1 class="text-2xl font-bold">Entities</h1>
				<a href="/<feature>/new" class="btn btn-primary">Create New</a>
			</div>
			
			<div class="space-y-2">
				for _, entity := range entities {
					<div class="card p-4">
						<a href={ templ.SafeURL("/<feature>/" + entity.ID.String()) }>
							{ entity.Name }
						</a>
					</div>
				}
			</div>
		</div>
	}
}
```

### 7. Wire Up in Router

**File**: `router/router.go` (add to SetupRoutes)

```go
// Create feature
featureModule := feature.NewFeature(pool, sessionManager)

// Add routes (inside authenticated group if needed)
featureModule.SetupRoutes(r)
```

### 8. Generate Templ Files

```bash
go tool templ generate
```

### 9. Create Migration (if new tables needed)

```bash
go tool task migrate:create -- add_entities_table
```

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| Package | lowercase singular | `campaign` |
| Feature struct | `Feature` | `Feature` |
| Handlers struct | `Handlers` | `Handlers` |
| Service | `<Entity>Service` | `CampaignService` |
| Repository | `<Entity>Repository` | `CampaignRepository` |
| Handler methods | `<Resource><Action>` | `CampaignListPage` |
| Route paths | kebab-case | `/campaigns/{id}/results` |

## Import Ordering

Follow this order with blank lines between groups:

```go
import (
	// Standard library
	"context"
	"net/http"

	// External packages
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	// Internal packages
	"github.com/cavenine/queryops/features/<feature>/services"
)
```

## Checklist

- [ ] Created directory structure
- [ ] Created repository with CRUD operations
- [ ] Created service with business logic
- [ ] Created handlers for HTTP endpoints
- [ ] Created routes.go with Feature struct
- [ ] Created templ page templates
- [ ] Generated templ files (`go tool templ generate`)
- [ ] Wired up in router/router.go
- [ ] Created database migration (if needed)
- [ ] Ran migrations (`go tool task migrate`)
- [ ] Tested with `go tool task live`
