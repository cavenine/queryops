// Package services provides data access and business logic for the index feature.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cavenine/queryops/features/index/components"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TodoRepository persists TodoMVC state per session in Postgres.
type TodoRepository struct {
	pool *pgxpool.Pool
}

// ErrTodoNotFound is returned when todo state for a session cannot be found.
var ErrTodoNotFound = errors.New("todo state not found")

// NewTodoRepository creates a new TodoRepository with the given connection pool.
func NewTodoRepository(pool *pgxpool.Pool) *TodoRepository {
	return &TodoRepository{pool: pool}
}

// GetMVC retrieves the TodoMVC state for the given session ID.
// It returns ErrTodoNotFound if no state exists for the session.
func (r *TodoRepository) GetMVC(ctx context.Context, sessionID string) (*components.TodoMVC, error) {
	row := r.pool.QueryRow(ctx, `SELECT state FROM todos WHERE session_id = $1`, sessionID)

	var data []byte
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTodoNotFound
		}
		return nil, fmt.Errorf("querying todo state: %w", err)
	}

	mvc := &components.TodoMVC{}
	if err := json.Unmarshal(data, mvc); err != nil {
		return nil, fmt.Errorf("unmarshalling todo state: %w", err)
	}

	return mvc, nil
}

// SaveMVC inserts or updates the TodoMVC state for the given session ID.
func (r *TodoRepository) SaveMVC(ctx context.Context, sessionID string, mvc *components.TodoMVC) error {
	data, err := json.Marshal(mvc)
	if err != nil {
		return fmt.Errorf("marshalling todo state: %w", err)
	}

	// Upsert: insert new row or update existing state if session already exists
	_, err = r.pool.Exec(ctx, `
INSERT INTO todos (session_id, state, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (session_id)
DO UPDATE SET state = EXCLUDED.state, updated_at = NOW()
`, sessionID, data)
	if err != nil {
		return fmt.Errorf("upserting todo state: %w", err)
	}

	return nil
}
