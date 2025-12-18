package index

import (
	"bytes"
	"encoding/json"
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

	send := func(state *components.TodoMVC) error {
		c := components.TodosMVCView(state)
		if err := sse.PatchElementTempl(c); err != nil {
			if err := sse.ConsoleError(err); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return err
		}
		return nil
	}

	// send initial state
	if err := send(mvc); err != nil {
		return
	}

	last, err := json.Marshal(mvc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := h.todoService.GetMVCBySessionID(ctx, sessionID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if current == nil {
				continue
			}

			b, err := json.Marshal(current)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if bytes.Equal(b, last) {
				continue
			}
			last = b

			if err := send(current); err != nil {
				return
			}
		}
	}
}

func (h *Handlers) ResetTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.todoService.ResetMVC(mvc)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) CancelEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		if err := sse.ConsoleError(err); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	h.todoService.CancelEditing(mvc)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		if err := sse.ConsoleError(err); err != nil {
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
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		if err := sse.ConsoleError(err); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	i, err := h.parseIndex(w, r)
	if err != nil {
		if err := sse.ConsoleError(err); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	h.todoService.ToggleTodo(mvc, i)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
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

	i, err := h.parseIndex(w, r)
	if err != nil {
		return
	}

	h.todoService.StartEditing(mvc, i)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
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

	i, err := h.parseIndex(w, r)
	if err != nil {
		return
	}

	h.todoService.EditTodo(mvc, i, store.Input)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	i, err := h.parseIndex(w, r)
	if err != nil {
		return
	}

	ctx := r.Context()
	sessionID, mvc, err := h.todoService.GetSessionMVC(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.todoService.DeleteTodo(mvc, i)
	if err := h.todoService.SaveMVC(ctx, sessionID, mvc); err != nil {
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
