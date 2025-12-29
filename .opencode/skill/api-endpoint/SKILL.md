---
name: api-endpoint
description: Create REST-like API endpoints with JSON responses, SSE streaming, and proper error handling
---

# API Endpoint Skill

Create REST-like API endpoints with JSON responses and SSE streaming.

## When to Use

- Adding new API endpoints (REST-like JSON or SSE)
- Creating endpoints for Datastar frontend interactions
- Implementing real-time streaming endpoints
- Building osquery-style TLS backend APIs

## Endpoint Types

| Type | Response | Use Case |
|------|----------|----------|
| JSON | `application/json` | CRUD operations, data APIs |
| HTML | `text/html` | Page renders, HTMX partials |
| SSE | `text/event-stream` | Real-time updates, Datastar patches |

## JSON API Endpoints

### 1. Basic Handler Structure

```go
package feature

import (
	"encoding/json"
	"net/http"
)

type CreateRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Name == "" {
		h.jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Business logic
	user, err := h.service.Create(r.Context(), req.Name, req.Email)
	if err != nil {
		h.jsonError(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Success response
	h.jsonResponse(w, CreateResponse{
		ID:      user.ID.String(),
		Message: "User created successfully",
	}, http.StatusCreated)
}

// Helper methods
func (h *Handlers) jsonResponse(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handlers) jsonError(w http.ResponseWriter, message string, status int) {
	h.jsonResponse(w, ErrorResponse{Error: message}, status)
}
```

### 2. Route Registration

```go
// routes.go
func (f *Feature) SetupAPIRoutes(r chi.Router) {
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Get("/", f.handlers.ListUsers)       // GET /api/v1/users
		r.Post("/", f.handlers.CreateUser)     // POST /api/v1/users
		r.Get("/{id}", f.handlers.GetUser)     // GET /api/v1/users/{id}
		r.Put("/{id}", f.handlers.UpdateUser)  // PUT /api/v1/users/{id}
		r.Delete("/{id}", f.handlers.DeleteUser) // DELETE /api/v1/users/{id}
	})
}
```

### 3. URL Parameters

```go
func (h *Handlers) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.jsonError(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.jsonError(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, user, http.StatusOK)
}
```

### 4. Query Parameters

```go
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		h.jsonError(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]any{
		"users":  users,
		"limit":  limit,
		"offset": offset,
	}, http.StatusOK)
}
```

## SSE Endpoints (Datastar)

### 1. Basic SSE Handler

```go
import (
	"github.com/starfederation/datastar-go/datastar"
)

func (h *Handlers) DataSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sse := datastar.NewSSE(w, r)

	// Send initial data
	data, err := h.service.GetData(ctx)
	if err != nil {
		_ = sse.ConsoleError(err)
		return
	}

	// Patch HTML element
	if err := sse.PatchElementTempl(components.DataView(data)); err != nil {
		return
	}

	// Keep connection alive with updates
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastData []byte
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := h.service.GetData(ctx)
			if err != nil {
				_ = sse.ConsoleError(err)
				return
			}

			// Only send if changed
			currentBytes, _ := json.Marshal(current)
			if !bytes.Equal(currentBytes, lastData) {
				lastData = currentBytes
				if err := sse.PatchElementTempl(components.DataView(current)); err != nil {
					return
				}
			}
		}
	}
}
```

### 2. Signal Patches (JSON Updates)

For updating reactive signals without HTML:

```go
func (h *Handlers) IncrementCounter(w http.ResponseWriter, r *http.Request) {
	newValue := h.counter.Add(1)

	sse := datastar.NewSSE(w, r)
	
	// Patch signals directly
	update := map[string]int64{"count": newValue}
	if err := sse.MarshalAndPatchSignals(update); err != nil {
		return
	}
}
```

### 3. Reading Signals from Request

```go
func (h *Handlers) UpdateItem(w http.ResponseWriter, r *http.Request) {
	// Read signals sent from frontend
	type Store struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	store := &Store{}

	if err := datastar.ReadSignals(r, store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use the data
	err := h.service.Update(r.Context(), store.Name, store.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with updated state
	sse := datastar.NewSSE(w, r)
	sse.MarshalAndPatchSignals(map[string]string{"status": "saved"})
}
```

### 4. Event-Driven SSE (Pub/Sub)

For real-time updates from background events:

```go
func (h *Handlers) ResultsSSE(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "id")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sse := datastar.NewSSE(w, r)

	// 1. Send initial state
	results, err := h.repo.GetRecentResults(ctx, hostID)
	if err != nil {
		_ = sse.ConsoleError(err)
		return
	}
	if err := sse.PatchElementTempl(pages.ResultsTable(results)); err != nil {
		return
	}

	// 2. Subscribe to events (if pub/sub available)
	if h.pubsub == nil {
		// Fall back to polling
		h.pollResults(ctx, sse, hostID)
		return
	}

	subscriber, err := h.pubsub.NewSubscriber(ctx)
	if err != nil {
		h.pollResults(ctx, sse, hostID)
		return
	}
	defer subscriber.Close()

	topic := pubsub.TopicQueryResults(hostID)
	messages, err := subscriber.Subscribe(ctx, topic)
	if err != nil {
		h.pollResults(ctx, sse, hostID)
		return
	}

	// 3. Stream updates from events
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-messages:
			msg.Ack()
			
			// Fetch fresh data and send
			results, err := h.repo.GetRecentResults(ctx, hostID)
			if err != nil {
				continue
			}
			if err := sse.PatchElementTempl(pages.ResultsTable(results)); err != nil {
				return
			}
		}
	}
}
```

## Error Handling Patterns

### 1. Structured Error Responses

```go
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (h *Handlers) jsonError(w http.ResponseWriter, code string, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{
		Code:    code,
		Message: message,
	})
}

// Usage
h.jsonError(w, "VALIDATION_ERROR", "Name is required", http.StatusBadRequest)
h.jsonError(w, "NOT_FOUND", "User not found", http.StatusNotFound)
h.jsonError(w, "INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError)
```

### 2. Logging Errors

```go
import "log/slog"

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	// ...
	
	user, err := h.service.Create(r.Context(), req.Name, req.Email)
	if err != nil {
		slog.Error("failed to create user",
			"error", err,
			"name", req.Name,
			"email", req.Email,
		)
		h.jsonError(w, "INTERNAL_ERROR", "Failed to create user", http.StatusInternalServerError)
		return
	}
	
	slog.Info("user created",
		"user_id", user.ID,
		"name", user.Name,
	)
	
	// ...
}
```

## Authentication & Authorization

### 1. Getting User from Context

```go
import "github.com/cavenine/queryops/features/auth"

func (h *Handlers) CreateItem(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.jsonError(w, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
		return
	}

	// Use user.ID, user.Email, etc.
}
```

### 2. Getting Active Organization

```go
import "github.com/cavenine/queryops/features/organization"

func (h *Handlers) ListHosts(w http.ResponseWriter, r *http.Request) {
	org := organization.GetOrgFromContext(r.Context())
	if org == nil {
		h.jsonError(w, "NO_ORG", "No active organization", http.StatusForbidden)
		return
	}

	// Filter by org.ID
	hosts, err := h.repo.ListByOrganization(r.Context(), org.ID)
	// ...
}
```

## Route Patterns

```go
// RESTful CRUD
r.Route("/api/v1/campaigns", func(r chi.Router) {
	r.Get("/", h.List)           // List all
	r.Post("/", h.Create)        // Create new
	r.Get("/{id}", h.Get)        // Get one
	r.Put("/{id}", h.Update)     // Update
	r.Delete("/{id}", h.Delete)  // Delete
	
	// Nested resources
	r.Get("/{id}/results", h.GetResults)
	
	// SSE endpoints
	r.Get("/{id}/results/sse", h.ResultsSSE)
	
	// Actions
	r.Post("/{id}/start", h.Start)
	r.Post("/{id}/cancel", h.Cancel)
})

// Datastar action endpoints
r.Post("/counter/increment", h.Increment)
r.Post("/counter/decrement", h.Decrement)
```

## Response Status Codes

| Code | Meaning | When to Use |
|------|---------|-------------|
| 200 | OK | Successful GET, PUT, PATCH |
| 201 | Created | Successful POST creating resource |
| 204 | No Content | Successful DELETE |
| 400 | Bad Request | Invalid JSON, validation errors |
| 401 | Unauthorized | Missing/invalid auth |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate, constraint violation |
| 500 | Internal Error | Unexpected server errors |

## Checklist

- [ ] Defined request/response structs with JSON tags
- [ ] Added input validation
- [ ] Used appropriate HTTP status codes
- [ ] Added error logging with `slog`
- [ ] Created helper methods for JSON responses
- [ ] Registered routes in `routes.go`
- [ ] Added authentication checks (if needed)
- [ ] Added organization scoping (if needed)
- [ ] Created corresponding tests
- [ ] Documented endpoint in API docs (if applicable)
