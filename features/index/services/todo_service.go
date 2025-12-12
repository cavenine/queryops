package services

import (
	"context"
	"fmt"
	"net/http"

	"queryops/features/index/components"

	"github.com/delaneyj/toolbelt"
	"github.com/gorilla/sessions"
	"github.com/samber/lo"
)

type TodoService struct {
	repo  *TodoRepository
	store sessions.Store
}

func NewTodoService(repo *TodoRepository, store sessions.Store) *TodoService {
	return &TodoService{
		repo:  repo,
		store: store,
	}
}

func (s *TodoService) GetSessionMVC(w http.ResponseWriter, r *http.Request) (string, *components.TodoMVC, error) {
	ctx := r.Context()
	sessionID, err := s.upsertSessionID(r, w)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get session id: %w", err)
	}

	mvc, err := s.repo.GetMVC(ctx, sessionID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get todo mvc: %w", err)
	}

	if mvc == nil {
		mvc = &components.TodoMVC{}
		s.resetMVC(mvc)

		if err := s.saveMVC(ctx, sessionID, mvc); err != nil {
			return "", nil, fmt.Errorf("failed to save mvc: %w", err)
		}
	}

	return sessionID, mvc, nil
}

func (s *TodoService) SaveMVC(ctx context.Context, sessionID string, mvc *components.TodoMVC) error {
	return s.saveMVC(ctx, sessionID, mvc)
}

// GetMVCBySessionID loads the current MVC state for a session without modifying it.
func (s *TodoService) GetMVCBySessionID(ctx context.Context, sessionID string) (*components.TodoMVC, error) {
	mvc, err := s.repo.GetMVC(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get todo mvc: %w", err)
	}
	return mvc, nil
}

func (s *TodoService) ResetMVC(mvc *components.TodoMVC) {
	s.resetMVC(mvc)
}

func (s *TodoService) ToggleTodo(mvc *components.TodoMVC, index int) {
	if index < 0 {
		setCompletedTo := false
		for _, todo := range mvc.Todos {
			if !todo.Completed {
				setCompletedTo = true
				break
			}
		}
		for _, todo := range mvc.Todos {
			todo.Completed = setCompletedTo
		}
	} else if index < len(mvc.Todos) {
		todo := mvc.Todos[index]
		todo.Completed = !todo.Completed
	}
}

func (s *TodoService) EditTodo(mvc *components.TodoMVC, index int, text string) {
	if index >= 0 && index < len(mvc.Todos) {
		mvc.Todos[index].Text = text
	} else if index < 0 {
		mvc.Todos = append(mvc.Todos, &components.Todo{
			Text:      text,
			Completed: false,
		})
	}
	mvc.EditingIdx = -1
}

func (s *TodoService) DeleteTodo(mvc *components.TodoMVC, index int) {
	if index >= 0 && index < len(mvc.Todos) {
		mvc.Todos = append(mvc.Todos[:index], mvc.Todos[index+1:]...)
	} else if index < 0 {
		mvc.Todos = lo.Filter(mvc.Todos, func(todo *components.Todo, i int) bool {
			return !todo.Completed
		})
	}
}

func (s *TodoService) SetMode(mvc *components.TodoMVC, mode components.TodoViewMode) {
	mvc.Mode = mode
}

func (s *TodoService) StartEditing(mvc *components.TodoMVC, index int) {
	mvc.EditingIdx = index
}

func (s *TodoService) CancelEditing(mvc *components.TodoMVC) {
	mvc.EditingIdx = -1
}

func (s *TodoService) saveMVC(ctx context.Context, sessionID string, mvc *components.TodoMVC) error {
	if err := s.repo.SaveMVC(ctx, sessionID, mvc); err != nil {
		return fmt.Errorf("failed to persist mvc: %w", err)
	}
	return nil
}

func (s *TodoService) resetMVC(mvc *components.TodoMVC) {
	mvc.Mode = components.TodoViewModeAll
	mvc.Todos = []*components.Todo{
		{Text: "Learn any backend language", Completed: true},
		{Text: "Learn Datastar", Completed: false},
		{Text: "Create Hypermedia", Completed: false},
		{Text: "???", Completed: false},
		{Text: "Profit", Completed: false},
	}
	mvc.EditingIdx = -1
}

func (s *TodoService) upsertSessionID(r *http.Request, w http.ResponseWriter) (string, error) {
	sess, err := s.store.Get(r, "connections")
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	id, ok := sess.Values["id"].(string)

	if !ok {
		id = toolbelt.NextEncodedID()
		sess.Values["id"] = id
		if err := sess.Save(r, w); err != nil {
			return "", fmt.Errorf("failed to save session: %w", err)
		}
	}

	return id, nil
}
