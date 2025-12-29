---
name: repository
description: Create PostgreSQL repository layer with pgxpool for CRUD operations, transactions, and query patterns
---

# Repository Pattern Skill

Create data access layer repositories using pgxpool for PostgreSQL operations.

## When to Use

- Creating new entity repositories for database access
- Adding CRUD operations for a new table
- Implementing complex queries (joins, aggregations)
- Working with transactions

## Repository Structure

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entity struct matching database table
type Campaign struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Query          string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Repository struct
type CampaignRepository struct {
	pool *pgxpool.Pool
}

// Constructor
func NewCampaignRepository(pool *pgxpool.Pool) *CampaignRepository {
	return &CampaignRepository{pool: pool}
}
```

## CRUD Operations

### Create

```go
func (r *CampaignRepository) Create(ctx context.Context, orgID uuid.UUID, name, query string) (*Campaign, error) {
	campaign := &Campaign{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO campaigns (organization_id, name, query, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, organization_id, name, query, status, created_at, updated_at
	`, orgID, name, query).Scan(
		&campaign.ID,
		&campaign.OrganizationID,
		&campaign.Name,
		&campaign.Query,
		&campaign.Status,
		&campaign.CreatedAt,
		&campaign.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating campaign: %w", err)
	}
	return campaign, nil
}
```

### Read (Single)

```go
func (r *CampaignRepository) GetByID(ctx context.Context, id uuid.UUID) (*Campaign, error) {
	campaign := &Campaign{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, name, query, status, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`, id).Scan(
		&campaign.ID,
		&campaign.OrganizationID,
		&campaign.Name,
		&campaign.Query,
		&campaign.Status,
		&campaign.CreatedAt,
		&campaign.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("getting campaign by id: %w", err)
	}
	return campaign, nil
}
```

### Read (With Organization Scoping)

```go
func (r *CampaignRepository) GetByIDAndOrg(ctx context.Context, id, orgID uuid.UUID) (*Campaign, error) {
	campaign := &Campaign{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, name, query, status, created_at, updated_at
		FROM campaigns
		WHERE id = $1 AND organization_id = $2
	`, id, orgID).Scan(
		&campaign.ID,
		&campaign.OrganizationID,
		&campaign.Name,
		&campaign.Query,
		&campaign.Status,
		&campaign.CreatedAt,
		&campaign.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting campaign by id and org: %w", err)
	}
	return campaign, nil
}
```

### Read (List)

```go
func (r *CampaignRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, organization_id, name, query, status, created_at, updated_at
		FROM campaigns
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		campaign := &Campaign{}
		if err := rows.Scan(
			&campaign.ID,
			&campaign.OrganizationID,
			&campaign.Name,
			&campaign.Query,
			&campaign.Status,
			&campaign.CreatedAt,
			&campaign.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning campaign: %w", err)
		}
		campaigns = append(campaigns, campaign)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating campaigns: %w", err)
	}

	return campaigns, nil
}
```

### Read (With Pagination)

```go
func (r *CampaignRepository) ListPaginated(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Campaign, int, error) {
	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM campaigns WHERE organization_id = $1
	`, orgID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting campaigns: %w", err)
	}

	// Get paginated results
	rows, err := r.pool.Query(ctx, `
		SELECT id, organization_id, name, query, status, created_at, updated_at
		FROM campaigns
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		campaign := &Campaign{}
		if err := rows.Scan(
			&campaign.ID,
			&campaign.OrganizationID,
			&campaign.Name,
			&campaign.Query,
			&campaign.Status,
			&campaign.CreatedAt,
			&campaign.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning campaign: %w", err)
		}
		campaigns = append(campaigns, campaign)
	}

	return campaigns, total, nil
}
```

### Update

```go
func (r *CampaignRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE campaigns
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, status, id)
	if err != nil {
		return fmt.Errorf("updating campaign status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("campaign not found")
	}

	return nil
}
```

### Delete

```go
func (r *CampaignRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM campaigns WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("deleting campaign: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("campaign not found")
	}

	return nil
}
```

## Advanced Patterns

### Upsert (Insert or Update)

```go
func (r *CampaignRepository) Upsert(ctx context.Context, campaign *Campaign) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO campaigns (id, organization_id, name, query, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    query = EXCLUDED.query,
		    status = EXCLUDED.status,
		    updated_at = CURRENT_TIMESTAMP
	`, campaign.ID, campaign.OrganizationID, campaign.Name, campaign.Query, campaign.Status)
	if err != nil {
		return fmt.Errorf("upserting campaign: %w", err)
	}
	return nil
}
```

### Transactions

```go
func (r *CampaignRepository) CreateWithTargets(ctx context.Context, campaign *Campaign, hostIDs []uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert campaign
	err = tx.QueryRow(ctx, `
		INSERT INTO campaigns (organization_id, name, query, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, created_at, updated_at
	`, campaign.OrganizationID, campaign.Name, campaign.Query).Scan(
		&campaign.ID, &campaign.CreatedAt, &campaign.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting campaign: %w", err)
	}

	// Insert targets
	for _, hostID := range hostIDs {
		_, err = tx.Exec(ctx, `
			INSERT INTO campaign_targets (campaign_id, host_id, status)
			VALUES ($1, $2, 'pending')
		`, campaign.ID, hostID)
		if err != nil {
			return fmt.Errorf("inserting target: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
```

### Batch Insert

```go
func (r *CampaignRepository) BatchInsertTargets(ctx context.Context, campaignID uuid.UUID, hostIDs []uuid.UUID) error {
	batch := &pgx.Batch{}
	
	for _, hostID := range hostIDs {
		batch.Queue(`
			INSERT INTO campaign_targets (campaign_id, host_id, status)
			VALUES ($1, $2, 'pending')
		`, campaignID, hostID)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range hostIDs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("batch inserting target: %w", err)
		}
	}

	return nil
}
```

### Joins

```go
type CampaignWithTargetCount struct {
	Campaign
	TargetCount     int
	CompletedCount  int
}

func (r *CampaignRepository) ListWithStats(ctx context.Context, orgID uuid.UUID) ([]*CampaignWithTargetCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT 
			c.id, c.organization_id, c.name, c.query, c.status, c.created_at, c.updated_at,
			COUNT(ct.host_id) as target_count,
			COUNT(ct.host_id) FILTER (WHERE ct.status = 'completed') as completed_count
		FROM campaigns c
		LEFT JOIN campaign_targets ct ON c.id = ct.campaign_id
		WHERE c.organization_id = $1
		GROUP BY c.id
		ORDER BY c.created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing campaigns with stats: %w", err)
	}
	defer rows.Close()

	var results []*CampaignWithTargetCount
	for rows.Next() {
		r := &CampaignWithTargetCount{}
		if err := rows.Scan(
			&r.ID, &r.OrganizationID, &r.Name, &r.Query, &r.Status,
			&r.CreatedAt, &r.UpdatedAt, &r.TargetCount, &r.CompletedCount,
		); err != nil {
			return nil, fmt.Errorf("scanning: %w", err)
		}
		results = append(results, r)
	}

	return results, nil
}
```

### JSONB Columns

```go
type Host struct {
	ID          uuid.UUID
	HostDetails map[string]any // JSONB column
}

func (r *HostRepository) Create(ctx context.Context, hostDetails map[string]any) (*Host, error) {
	host := &Host{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO hosts (host_details)
		VALUES ($1)
		RETURNING id, host_details
	`, hostDetails).Scan(&host.ID, &host.HostDetails)
	if err != nil {
		return nil, fmt.Errorf("creating host: %w", err)
	}
	return host, nil
}

// Query JSONB field
func (r *HostRepository) ListByPlatform(ctx context.Context, platform string) ([]*Host, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, host_details
		FROM hosts
		WHERE host_details->>'platform' = $1
	`, platform)
	// ...
}
```

## Error Handling

```go
import (
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")
var ErrDuplicate = errors.New("duplicate entry")

func (r *CampaignRepository) GetByID(ctx context.Context, id uuid.UUID) (*Campaign, error) {
	// ...
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting campaign: %w", err)
	}
	// ...
}

func (r *CampaignRepository) Create(ctx context.Context, c *Campaign) error {
	_, err := r.pool.Exec(ctx, ...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return ErrDuplicate
			}
		}
		return fmt.Errorf("creating campaign: %w", err)
	}
	return nil
}
```

## Scan Helper Pattern

For reusable scanning:

```go
func scanCampaign(row pgx.Row) (*Campaign, error) {
	c := &Campaign{}
	err := row.Scan(
		&c.ID,
		&c.OrganizationID,
		&c.Name,
		&c.Query,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func scanCampaigns(rows pgx.Rows) ([]*Campaign, error) {
	var campaigns []*Campaign
	for rows.Next() {
		c := &Campaign{}
		if err := rows.Scan(
			&c.ID,
			&c.OrganizationID,
			&c.Name,
			&c.Query,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, rows.Err()
}
```

## Checklist

- [ ] Created struct matching database table
- [ ] Created repository struct with pool
- [ ] Created constructor `NewXxxRepository(pool)`
- [ ] Implemented Create with RETURNING clause
- [ ] Implemented GetByID with nil for not found
- [ ] Implemented List with proper ordering
- [ ] Added organization scoping where needed
- [ ] Used `defer rows.Close()` after Query
- [ ] Wrapped errors with `fmt.Errorf(": %w", err)`
- [ ] Handled `pgx.ErrNoRows` appropriately
- [ ] Created corresponding migration
