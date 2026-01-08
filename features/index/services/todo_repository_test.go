package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cavenine/queryops/features/index/components"
	"github.com/cavenine/queryops/features/index/services"
	"github.com/cavenine/queryops/internal/testdb"
)

func TestTodoRepository_GetMVC(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	repo := services.NewTodoRepository(tdb.Pool)

	sessionID := "test-session-1"
	mvc := &components.TodoMVC{
		Todos: []*components.Todo{
			{Text: "Test todo", Completed: false},
		},
	}

	data, err := json.Marshal(mvc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if _, err := tdb.Pool.Exec(ctx, `INSERT INTO todos (session_id, state) VALUES ($1, $2)`, sessionID, data); err != nil {
		t.Fatalf("seed todos: %v", err)
	}

	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		err       error
	}{
		{
			name:      "found",
			sessionID: sessionID,
			wantErr:   false,
		},
		{
			name:      "not found",
			sessionID: "nonexistent-session",
			wantErr:   true,
			err:       services.ErrTodoNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetMVC(ctx, tt.sessionID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetMVC() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, tt.err) {
					t.Fatalf("error = %v, want %v", err, tt.err)
				}
				return
			}
			if len(got.Todos) != 1 {
				t.Fatalf("len(Todos) = %d, want 1", len(got.Todos))
			}
			if got.Todos[0].Text != "Test todo" {
				t.Fatalf("Text = %q, want %q", got.Todos[0].Text, "Test todo")
			}
		})
	}
}

func TestTodoRepository_SaveMVC(t *testing.T) {
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	repo := services.NewTodoRepository(tdb.Pool)

	sessionID := "test-session-2"
	mvc := &components.TodoMVC{
		Todos: []*components.Todo{
			{Text: "First todo", Completed: false},
			{Text: "Second todo", Completed: true},
		},
	}

	t.Run("insert", func(t *testing.T) {
		if err := repo.SaveMVC(ctx, sessionID, mvc); err != nil {
			t.Fatalf("SaveMVC(insert) error = %v", err)
		}

		got, err := repo.GetMVC(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetMVC() error = %v", err)
		}
		if len(got.Todos) != 2 {
			t.Fatalf("len(Todos) = %d, want 2", len(got.Todos))
		}
	})

	t.Run("update", func(t *testing.T) {
		updatedMVC := &components.TodoMVC{
			Todos: []*components.Todo{
				{Text: "Updated todo", Completed: true},
			},
		}

		if err := repo.SaveMVC(ctx, sessionID, updatedMVC); err != nil {
			t.Fatalf("SaveMVC(update) error = %v", err)
		}

		got, err := repo.GetMVC(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetMVC() error = %v", err)
		}
		if len(got.Todos) != 1 {
			t.Fatalf("len(Todos) after update = %d, want 1", len(got.Todos))
		}
		if got.Todos[0].Text != "Updated todo" {
			t.Fatalf("Text = %q, want %q", got.Todos[0].Text, "Updated todo")
		}
	})
}
