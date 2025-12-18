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
	return r.getBy(ctx, "node_key", nodeKey)
}

func (r *HostRepository) GetByID(ctx context.Context, id uuid.UUID) (*Host, error) {
	return r.getBy(ctx, "id", id)
}

func (r *HostRepository) getBy(ctx context.Context, column string, value any) (*Host, error) {
	var h Host
	query := fmt.Sprintf(`
		SELECT id, host_identifier, node_key, os_version, osquery_info, system_info, platform_info,
		       last_enrollment_at, last_config_at, last_logger_at, last_distributed_at, created_at, updated_at
		FROM hosts WHERE %s = $1
	`, column)
	err := r.pool.QueryRow(ctx, query, value).Scan(
		&h.ID, &h.HostIdentifier, &h.NodeKey, &h.OSVersion, &h.OsqueryInfo, &h.SystemInfo, &h.PlatformInfo,
		&h.LastEnrollmentAt, &h.LastConfigAt, &h.LastLoggerAt, &h.LastDistributedAt, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting host by %s: %w", column, err)
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

func (r *HostRepository) List(ctx context.Context) ([]*Host, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, host_identifier, node_key, os_version, osquery_info, system_info, platform_info,
		       last_enrollment_at, last_config_at, last_logger_at, last_distributed_at, created_at, updated_at
		FROM hosts
		ORDER BY last_logger_at DESC NULLS LAST
	`)
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	defer rows.Close()

	var hosts []*Host
	for rows.Next() {
		var h Host
		err := rows.Scan(
			&h.ID, &h.HostIdentifier, &h.NodeKey, &h.OSVersion, &h.OsqueryInfo, &h.SystemInfo, &h.PlatformInfo,
			&h.LastEnrollmentAt, &h.LastConfigAt, &h.LastLoggerAt, &h.LastDistributedAt, &h.CreatedAt, &h.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning host: %w", err)
		}
		hosts = append(hosts, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}
	return hosts, nil
}

func (r *HostRepository) SaveResultLogs(ctx context.Context, hostID uuid.UUID, name, action string, columns json.RawMessage, timestamp time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO osquery_results (host_id, name, action, columns, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, hostID, name, action, columns, timestamp)
	return err
}

func (r *HostRepository) SaveStatusLogs(ctx context.Context, hostID uuid.UUID, line int, message string, severity int, filename string, createdAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO osquery_status_logs (host_id, line, message, severity, filename, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, hostID, line, message, severity, filename, createdAt)
	return err
}

func (r *HostRepository) GetConfigForHost(ctx context.Context, nodeKey string) (json.RawMessage, error) {
	var config json.RawMessage
	err := r.pool.QueryRow(ctx, `
		SELECT c.config 
		FROM osquery_configs c
		JOIN hosts h ON h.config_id = c.id
		WHERE h.node_key = $1
	`, nodeKey).Scan(&config)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Return default config
			err = r.pool.QueryRow(ctx, `SELECT config FROM osquery_configs WHERE name = 'default'`).Scan(&config)
			if err != nil {
				return nil, err
			}
			return config, nil
		}
		return nil, err
	}
	return config, nil
}

func (r *HostRepository) GetPendingQueries(ctx context.Context, hostID uuid.UUID) (map[string]string, error) {
	// Atomically fetch pending queries and mark them sent.
	rows, err := r.pool.Query(ctx, `
		UPDATE distributed_query_targets t
		SET status = 'sent', updated_at = NOW()
		FROM distributed_queries q
		WHERE t.query_id = q.id
			AND t.host_id = $1
			AND t.status = 'pending'
		RETURNING t.query_id, q.query
	`, hostID)
	if err != nil {
		return nil, fmt.Errorf("getting pending queries: %w", err)
	}
	defer rows.Close()

	queries := make(map[string]string)
	for rows.Next() {
		var id uuid.UUID
		var query string
		if err := rows.Scan(&id, &query); err != nil {
			return nil, fmt.Errorf("scanning pending query: %w", err)
		}
		queries[id.String()] = query
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pending queries: %w", err)
	}

	return queries, nil
}

func (r *HostRepository) SaveQueryResults(ctx context.Context, hostID uuid.UUID, queryID uuid.UUID, status string, results json.RawMessage, errorText *string) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE distributed_query_targets
		SET status = $1, results = $2, error = $3, updated_at = NOW()
		WHERE query_id = $4 AND host_id = $5
	`, status, results, errorText, queryID, hostID)
	if err != nil {
		return fmt.Errorf("saving query results: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("saving query results: no target row")
	}
	return nil
}

type QueryResult struct {
	QueryID   uuid.UUID
	Query     string
	Status    string
	Results   json.RawMessage
	UpdatedAt time.Time
}

func (r *HostRepository) GetRecentResults(ctx context.Context, hostID uuid.UUID) ([]QueryResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.query, t.status, t.results, t.updated_at
		FROM distributed_queries q
		JOIN distributed_query_targets t ON t.query_id = q.id
		WHERE t.host_id = $1
		ORDER BY t.updated_at DESC
		LIMIT 10
	`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var res QueryResult
		if err := rows.Scan(&res.QueryID, &res.Query, &res.Status, &res.Results, &res.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning query result: %w", err)
		}
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getting recent results: %w", err)
	}
	return results, nil
}

func (r *HostRepository) QueueQuery(ctx context.Context, query string, hostIDs []uuid.UUID) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var queryID uuid.UUID
	err = tx.QueryRow(ctx, "INSERT INTO distributed_queries (query) VALUES ($1) RETURNING id", query).Scan(&queryID)
	if err != nil {
		return uuid.Nil, err
	}

	for _, hostID := range hostIDs {
		_, err = tx.Exec(ctx, "INSERT INTO distributed_query_targets (query_id, host_id) VALUES ($1, $2)", queryID, hostID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	return queryID, tx.Commit(ctx)
}
