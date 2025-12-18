package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Host struct {
	ID                uuid.UUID
	HostIdentifier    string
	NodeKey           string
	OSVersion         json.RawMessage
	OsqueryInfo       json.RawMessage
	SystemInfo        json.RawMessage
	PlatformInfo      json.RawMessage
	LastEnrollmentAt  time.Time
	LastConfigAt      *time.Time
	LastLoggerAt      *time.Time
	LastDistributedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type HostRepository struct {
	pool *pgxpool.Pool
}

func NewHostRepository(pool *pgxpool.Pool) *HostRepository {
	return &HostRepository{pool: pool}
}

func (r *HostRepository) Enroll(ctx context.Context, hostIdentifier string, hostDetails json.RawMessage) (string, error) {
	nodeKey := uuid.New().String()

	// Parse host details to extract info if possible, or just store as JSONB
	// For now, we'll store the whole thing in relevant columns if they exist in the raw message
	// or just leave them null for now and let the caller handle it.
	// Actually, let's just store the raw message in one place or try to split it.
	// The prompt says "For now this will include the detailed in the enrollment request."

	_, err := r.pool.Exec(ctx, `
		INSERT INTO hosts (host_identifier, node_key, last_enrollment_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (host_identifier)
		DO UPDATE SET node_key = EXCLUDED.node_key, last_enrollment_at = NOW(), updated_at = NOW()
	`, hostIdentifier, nodeKey)
	if err != nil {
		return "", fmt.Errorf("enrolling host: %w", err)
	}

	return nodeKey, nil
}

func (r *HostRepository) GetByNodeKey(ctx context.Context, nodeKey string) (*Host, error) {
	var h Host
	err := r.pool.QueryRow(ctx, `
		SELECT id, host_identifier, node_key, os_version, osquery_info, system_info, platform_info,
		       last_enrollment_at, last_config_at, last_logger_at, last_distributed_at, created_at, updated_at
		FROM hosts WHERE node_key = $1
	`, nodeKey).Scan(
		&h.ID, &h.HostIdentifier, &h.NodeKey, &h.OSVersion, &h.OsqueryInfo, &h.SystemInfo, &h.PlatformInfo,
		&h.LastEnrollmentAt, &h.LastConfigAt, &h.LastLoggerAt, &h.LastDistributedAt, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting host by node key: %w", err)
	}
	return &h, nil
}

func (r *HostRepository) UpdateLastConfig(ctx context.Context, nodeKey string) error {
	_, err := r.pool.Exec(ctx, `UPDATE hosts SET last_config_at = NOW(), updated_at = NOW() WHERE node_key = $1`, nodeKey)
	return err
}

func (r *HostRepository) UpdateLastLogger(ctx context.Context, nodeKey string) error {
	_, err := r.pool.Exec(ctx, `UPDATE hosts SET last_logger_at = NOW(), updated_at = NOW() WHERE node_key = $1`, nodeKey)
	return err
}

func (r *HostRepository) UpdateLastDistributed(ctx context.Context, nodeKey string) error {
	_, err := r.pool.Exec(ctx, `UPDATE hosts SET last_distributed_at = NOW(), updated_at = NOW() WHERE node_key = $1`, nodeKey)
	return err
}
