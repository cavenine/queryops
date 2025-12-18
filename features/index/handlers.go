package index

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/cavenine/queryops/features/index/components"
	"github.com/cavenine/queryops/features/index/pages"
	"github.com/cavenine/queryops/features/index/services"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
)

type Handlers struct {
	todoService *services.TodoService
}

func NewHandlers(todoService *services.TodoService) *Handlers {
	return &Handlers{
		todoService: todoService,
	}
}

func (h *Handlers) IndexPage(w http.ResponseWriter, r *http.Request) {
	if err := pages.IndexPage("QueryOps").Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) TodosSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)

	// send initial state
	if err = h.sendTodos(sse, mvc); err != nil {
		return
	}

	var data []byte
	data, err = json.Marshal(mvc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.runTodoSSELoop(ctx, sse, sessionID, data)
}

func (h *Handlers) runTodoSSELoop(ctx context.Context, sse *datastar.ServerSentEventGenerator, sessionID string, last []byte) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := h.todoService.GetMVCBySessionID(ctx, sessionID)
			if err != nil {
				if errors.Is(err, services.ErrTodoNotFound) {
					continue
				}
				slog.ErrorContext(ctx, "failed to get todo mvc", "error", err)
				return
			}

			var b []byte
			b, err = json.Marshal(current)
			if err != nil {
				slog.ErrorContext(ctx, "failed to marshal current mvc", "error", err)
				return
			}
			if bytes.Equal(b, last) {
				continue
			}
			last = b

			if err = h.sendTodos(sse, current); err != nil {
				return
			}
		}
	}
}

func (h *Handlers) sendTodos(sse *datastar.ServerSentEventGenerator, mvc *components.TodoMVC) error {
	c := components.TodosMVCView(mvc)
	if err := sse.PatchElementTempl(c); err != nil {
		if consoleErr := sse.ConsoleError(err); consoleErr != nil {
			// Best effort
			slog.Error("failed to send console error", "error", consoleErr)
		}
		return err
	}
	return nil
}

func (h *Handlers) ResetTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.todoService.ResetMVC(mvc)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) CancelEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		if consoleErr := sse.ConsoleError(err); consoleErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	h.todoService.CancelEditing(mvc)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		if consoleErr := sse.ConsoleError(err); consoleErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
}

func (h *Handlers) SetMode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	modeStr := chi.URLParam(r, "mode")
	modeRaw, err := strconv.Atoi(modeStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mode := components.TodoViewMode(modeRaw)
	if mode < components.TodoViewModeAll || mode > components.TodoViewModeCompleted {
		http.Error(w, "invalid mode", http.StatusBadRequest)
		return
	}

	h.todoService.SetMode(mvc, mode)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		if consoleErr := sse.ConsoleError(err); consoleErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	var i int
	i, err = h.parseIndex(w, r)
	if err != nil {
		if consoleErr := sse.ConsoleError(err); consoleErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	h.todoService.ToggleTodo(mvc, i)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) StartEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var i int
	i, err = h.parseIndex(w, r)
	if err != nil {
		return
	}

	h.todoService.StartEditing(mvc, i)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) SaveEdit(w http.ResponseWriter, r *http.Request) {
	type Store struct {
		Input string `json:"input"`
	}
	store := &Store{}

	if err := datastar.ReadSignals(r, store); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if store.Input == "" {
		return
	}

	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var i int
	i, err = h.parseIndex(w, r)
	if err != nil {
		return
	}

	h.todoService.EditTodo(mvc, i, store.Input)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	var i int
	var err error
	i, err = h.parseIndex(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()
	var sessionID string
	var mvc *components.TodoMVC
	sessionID, mvc, err = h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.todoService.DeleteTodo(mvc, i)
	if err = h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) parseIndex(w http.ResponseWriter, r *http.Request) (int, error) {
	idx := chi.URLParam(r, "idx")
	i, err := strconv.Atoi(idx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return 0, err
	}
	return i, nil
}
