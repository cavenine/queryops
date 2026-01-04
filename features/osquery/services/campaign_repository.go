package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Campaign struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           *string   `json:"name,omitempty"`
	Description    *string   `json:"description,omitempty"`
	Query          string    `json:"query"`
	CreatedBy      *int      `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Status         string    `json:"status"`
	TargetCount    int       `json:"target_count"`
	ResultCount    int       `json:"result_count"`
}

type CampaignTarget struct {
	CampaignID     uuid.UUID       `json:"campaign_id"`
	HostID         uuid.UUID       `json:"host_id"`
	HostIdentifier string          `json:"host_identifier"`
	Status         string          `json:"status"`
	SentAt         *time.Time      `json:"sent_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Results        json.RawMessage `json:"results,omitempty"`
	Error          *string         `json:"error,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (r *HostRepository) GetCampaignByIDAndOrganization(ctx context.Context, campaignID uuid.UUID, organizationID uuid.UUID) (*Campaign, error) {
	var c Campaign

	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, name, description, query, created_by, created_at, updated_at, status, target_count, result_count
		FROM campaigns
		WHERE id = $1 AND organization_id = $2
	`, campaignID, organizationID).Scan(
		&c.ID,
		&c.OrganizationID,
		&c.Name,
		&c.Description,
		&c.Query,
		&c.CreatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.Status,
		&c.TargetCount,
		&c.ResultCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting campaign: %w", err)
	}

	return &c, nil
}

func (r *HostRepository) ListCampaignsByOrganization(ctx context.Context, organizationID uuid.UUID, limit int) ([]*Campaign, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, organization_id, name, description, query, created_by, created_at, updated_at, status, target_count, result_count
		FROM campaigns
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(
			&c.ID,
			&c.OrganizationID,
			&c.Name,
			&c.Description,
			&c.Query,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.Status,
			&c.TargetCount,
			&c.ResultCount,
		); err != nil {
			return nil, fmt.Errorf("scanning campaign: %w", err)
		}
		campaigns = append(campaigns, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing campaigns: %w", err)
	}

	return campaigns, nil
}

func (r *HostRepository) GetCampaignTargets(ctx context.Context, campaignID uuid.UUID) ([]*CampaignTarget, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.campaign_id, t.host_id, h.host_identifier, t.status, t.sent_at, t.completed_at, t.results, t.error, t.updated_at
		FROM campaign_targets t
		JOIN hosts h ON h.id = t.host_id
		WHERE t.campaign_id = $1
		ORDER BY h.host_identifier ASC
	`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("getting campaign targets: %w", err)
	}
	defer rows.Close()

	var targets []*CampaignTarget
	for rows.Next() {
		var t CampaignTarget
		if err := rows.Scan(
			&t.CampaignID,
			&t.HostID,
			&t.HostIdentifier,
			&t.Status,
			&t.SentAt,
			&t.CompletedAt,
			&t.Results,
			&t.Error,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning campaign target: %w", err)
		}
		targets = append(targets, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getting campaign targets: %w", err)
	}

	return targets, nil
}
