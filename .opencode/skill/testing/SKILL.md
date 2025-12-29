---
name: testing
description: Write unit and integration tests for handlers, services, and repositories using httptest and testcontainers
---

# Testing Skill

Write unit and integration tests following project conventions.

## When to Use

- Adding tests for new handlers or services
- Creating integration tests with real database
- Writing table-driven tests for edge cases
- Testing SSE endpoints and Datastar interactions

## Test Organization

```
features/<feature>/
├── handlers_test.go       # Handler unit tests
├── handlers.go
└── services/
    ├── <entity>_repository_test.go  # Repository integration tests
    └── <entity>_service_test.go     # Service unit tests

internal/
└── testdb/
    └── testdb.go          # Testcontainers helper
```

## Running Tests

```bash
# All tests
go test ./...

# Single package
go test ./features/osquery

# Single test
go test ./features/osquery -run TestEnroll

# Integration tests (requires Docker)
go test ./internal/testdb

# With verbose output
go test -v ./...

# Using Taskfile
go tool task test          # Unit tests
go tool task test:integration  # Integration tests
go tool task test:all      # Both
```

## Unit Tests (Handlers)

### 1. Define Test Interfaces (Stubs)

Create minimal interfaces for dependency injection:

```go
// handlers_test.go
package osquery

import (
	"context"
	"github.com/google/uuid"
	"github.com/cavenine/queryops/features/osquery/services"
)

// stubHostRepository implements hostRepository for testing.
type stubHostRepository struct {
	host           *services.Host
	getByNodeKeyFn func(ctx context.Context, nodeKey string) (*services.Host, error)
	enrollFn       func(ctx context.Context, hostIdentifier string, hostDetails map[string]any, orgID uuid.UUID) (*services.Host, error)
	// Add more as needed
}

func (s *stubHostRepository) GetByNodeKey(ctx context.Context, nodeKey string) (*services.Host, error) {
	if s.getByNodeKeyFn != nil {
		return s.getByNodeKeyFn(ctx, nodeKey)
	}
	return s.host, nil
}

func (s *stubHostRepository) Enroll(ctx context.Context, hostIdentifier string, hostDetails map[string]any, orgID uuid.UUID) (*services.Host, error) {
	if s.enrollFn != nil {
		return s.enrollFn(ctx, hostIdentifier, hostDetails, orgID)
	}
	return s.host, nil
}

// stubOrgLookup implements enrollmentOrgLookup for testing.
type stubOrgLookup struct {
	org *services.Organization
	err error
}

func (s *stubOrgLookup) GetOrganizationByEnrollSecret(ctx context.Context, secret string) (*services.Organization, error) {
	return s.org, s.err
}
```

### 2. Table-Driven Handler Tests

```go
package osquery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnroll(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    any
		orgLookup      *stubOrgLookup
		hostRepo       *stubHostRepository
		wantStatus     int
		wantNodeKey    bool
		wantNodeInvalid bool
	}{
		{
			name:        "invalid json",
			requestBody: "not json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "organization not found",
			requestBody: EnrollRequest{
				EnrollSecret:   "invalid-secret",
				HostIdentifier: "host-1",
			},
			orgLookup:       &stubOrgLookup{org: nil, err: nil},
			wantStatus:      http.StatusOK,
			wantNodeInvalid: true,
		},
		{
			name: "successful enrollment",
			requestBody: EnrollRequest{
				EnrollSecret:   "valid-secret",
				HostIdentifier: "host-1",
				HostDetails:    map[string]any{"platform": "darwin"},
			},
			orgLookup: &stubOrgLookup{
				org: &services.Organization{ID: uuid.New(), Name: "Test Org"},
			},
			hostRepo: &stubHostRepository{
				host: &services.Host{
					ID:      uuid.New(),
					NodeKey: "generated-node-key",
				},
			},
			wantStatus:  http.StatusOK,
			wantNodeKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup handler with stubs
			h := NewHandlers(tt.hostRepo, tt.orgLookup, nil, nil)

			// Create request
			var body []byte
			switch v := tt.requestBody.(type) {
			case string:
				body = []byte(v)
			default:
				body, _ = json.Marshal(v)
			}
			req := httptest.NewRequest(http.MethodPost, "/osquery/enroll", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Execute
			w := httptest.NewRecorder()
			h.Enroll(w, req)

			// Assert status
			assert.Equal(t, tt.wantStatus, w.Code)

			// Assert response body
			if w.Code == http.StatusOK {
				var resp EnrollResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)

				if tt.wantNodeInvalid {
					assert.True(t, resp.NodeInvalid)
					assert.Empty(t, resp.NodeKey)
				}
				if tt.wantNodeKey {
					assert.False(t, resp.NodeInvalid)
					assert.NotEmpty(t, resp.NodeKey)
				}
			}
		})
	}
}
```

### 3. Testing with Chi Router

When testing handlers that use URL parameters:

```go
func TestDetailPage(t *testing.T) {
	hostID := uuid.New()
	
	repo := &stubHostRepository{
		host: &services.Host{ID: hostID, Hostname: "test-host"},
	}
	h := NewHandlers(repo, nil, nil, nil)

	// Setup chi router for URL params
	r := chi.NewRouter()
	r.Get("/hosts/{id}", h.DetailPage)

	req := httptest.NewRequest(http.MethodGet, "/hosts/"+hostID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

## Integration Tests (Database)

### 1. Using Testcontainers Helper

```go
package services_test

import (
	"context"
	"testing"

	"github.com/cavenine/queryops/internal/testdb"
	"github.com/cavenine/queryops/features/osquery/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostRepository_Create(t *testing.T) {
	// Setup test database (requires Docker)
	pool := testdb.NewPool(t)
	repo := services.NewHostRepository(pool)

	ctx := context.Background()

	// Create host
	host, err := repo.Enroll(ctx, "test-host-1", map[string]any{
		"platform": "darwin",
	}, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, host)

	// Verify
	assert.NotEqual(t, uuid.Nil, host.ID)
	assert.NotEmpty(t, host.NodeKey)
	assert.Equal(t, "test-host-1", host.HostIdentifier)
}

func TestHostRepository_GetByNodeKey(t *testing.T) {
	pool := testdb.NewPool(t)
	repo := services.NewHostRepository(pool)
	ctx := context.Background()

	// Create host first
	created, err := repo.Enroll(ctx, "test-host", nil, testOrgID)
	require.NoError(t, err)

	// Retrieve by node key
	found, err := repo.GetByNodeKey(ctx, created.NodeKey)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.NodeKey, found.NodeKey)
}
```

### 2. Test Isolation with Reuse Mode

For faster local development, use container reuse:

```bash
# Enable container reuse (persists between runs)
QUERYOPS_TESTDB_REUSE=1 go test ./...
```

Each test gets an isolated database cloned from a migrated template.

## Service Tests

```go
package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cavenine/queryops/features/organization/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrgRepository for unit testing service logic
type mockOrgRepository struct {
	createFn func(ctx context.Context, name string, ownerID int) (*services.Organization, error)
}

func (m *mockOrgRepository) Create(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
	if m.createFn != nil {
		return m.createFn(ctx, name, ownerID)
	}
	return nil, nil
}

func TestOrganizationService_Create(t *testing.T) {
	tests := []struct {
		name    string
		orgName string
		ownerID int
		mockFn  func(ctx context.Context, name string, ownerID int) (*services.Organization, error)
		wantErr bool
	}{
		{
			name:    "successful creation",
			orgName: "Test Org",
			ownerID: 1,
			mockFn: func(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
				return &services.Organization{Name: name}, nil
			},
			wantErr: false,
		},
		{
			name:    "repository error",
			orgName: "Test Org",
			ownerID: 1,
			mockFn: func(ctx context.Context, name string, ownerID int) (*services.Organization, error) {
				return nil, errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOrgRepository{createFn: tt.mockFn}
			svc := services.NewOrganizationService(repo)

			org, err := svc.Create(context.Background(), tt.orgName, tt.ownerID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.orgName, org.Name)
			}
		})
	}
}
```

## Testing Patterns

### 1. Test Fixtures

```go
func setupTestOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	
	var orgID uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO organizations (name)
		VALUES ('test-org')
		RETURNING id
	`).Scan(&orgID)
	require.NoError(t, err)
	
	return orgID
}

func setupTestHost(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) *services.Host {
	t.Helper()
	
	repo := services.NewHostRepository(pool)
	host, err := repo.Enroll(context.Background(), "test-host", nil, orgID)
	require.NoError(t, err)
	
	return host
}
```

### 2. Parallel Tests

```go
func TestHostRepository(t *testing.T) {
	t.Parallel() // Run subtests in parallel
	
	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		pool := testdb.NewPool(t)
		// ...
	})
	
	t.Run("GetByID", func(t *testing.T) {
		t.Parallel()
		pool := testdb.NewPool(t)
		// ...
	})
}
```

### 3. Error Assertions

```go
func TestEnroll_Errors(t *testing.T) {
	t.Run("duplicate host identifier", func(t *testing.T) {
		pool := testdb.NewPool(t)
		repo := services.NewHostRepository(pool)
		ctx := context.Background()

		// Create first host
		_, err := repo.Enroll(ctx, "host-1", nil, orgID)
		require.NoError(t, err)

		// Try to create duplicate
		_, err = repo.Enroll(ctx, "host-1", nil, orgID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
	})
}
```

## Commands Reference

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test file
go test ./features/osquery/handlers_test.go ./features/osquery/handlers.go

# Run tests matching pattern
go test ./... -run "TestEnroll"

# Verbose output
go test -v ./features/osquery

# Skip integration tests (when Docker not available)
go test -short ./...
```

## Checklist

- [ ] Created test file with `_test.go` suffix
- [ ] Defined stub/mock interfaces for dependencies
- [ ] Used table-driven tests for multiple cases
- [ ] Used `t.Helper()` in setup functions
- [ ] Used `require` for fatal assertions, `assert` for non-fatal
- [ ] Added `t.Parallel()` where safe
- [ ] Ran `go test ./...` to verify
- [ ] Added integration tests for data layer (if applicable)
