package counter

import (
	"net/http"
	"sync/atomic"

	"queryops/features/counter/pages"

	"github.com/Jeffail/gabs/v2"
	"github.com/alexedwards/scs/v2"
	"github.com/starfederation/datastar-go/datastar"
)

const countKey = "counter_count"

type Handlers struct {
	globalCounter  atomic.Uint32
	sessionManager *scs.SessionManager
}

func NewHandlers(sessionManager *scs.SessionManager) *Handlers {
	return &Handlers{
		sessionManager: sessionManager,
	}
}

func (h *Handlers) CounterPage(w http.ResponseWriter, r *http.Request) {
	if err := pages.CounterPage().Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) CounterData(w http.ResponseWriter, r *http.Request) {
	userCount := h.getUserValue(r)

	store := pages.CounterSignals{
		Global: h.globalCounter.Load(),
		User:   userCount,
	}

	if err := datastar.NewSSE(w, r).PatchElementTempl(pages.Counter(store)); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) IncrementGlobal(w http.ResponseWriter, r *http.Request) {
	update := gabs.New()
	h.updateGlobal(update)

	if err := datastar.NewSSE(w, r).MarshalAndPatchSignals(update); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) IncrementUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	val := h.sessionManager.GetInt(ctx, countKey)
	val++
	h.sessionManager.Put(ctx, countKey, val)

	update := gabs.New()
	h.updateGlobal(update)
	if _, err := update.Set(uint32(val), "user"); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := datastar.NewSSE(w, r).MarshalAndPatchSignals(update); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *Handlers) getUserValue(r *http.Request) uint32 {
	return uint32(h.sessionManager.GetInt(r.Context(), countKey))
}

func (h *Handlers) updateGlobal(store *gabs.Container) {
	_, _ = store.Set(h.globalCounter.Add(1), "global")
}
