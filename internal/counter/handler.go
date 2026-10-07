package counter

import (
	"context"
	"errors"
	"net/http"

	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/httpx"
)

type heroStore interface {
	Get(ctx context.Context, uid string) (hero.Hero, error)
}

type counterStore interface {
	ForTarget(ctx context.Context, target string) ([]WithHero, error)
}

// Handler serves the Counters of a hero.
type Handler struct {
	heroes   heroStore
	counters counterStore
}

// NewHandler serves Counters from counters, checking heroes exist in heroes.
func NewHandler(heroes heroStore, counters counterStore) Handler {
	return Handler{heroes: heroes, counters: counters}
}

// Register adds the Counter route to mux.
func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/heroes/{heroId}/counters", h.list)
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("heroId")
	if _, err := h.heroes.Get(r.Context(), target); errors.Is(err, hero.ErrNotFound) {
		hero.WriteNotFound(w)
		return
	} else if err != nil {
		httpx.InternalError(w, "get hero", err)
		return
	}
	counters, err := h.counters.ForTarget(r.Context(), target)
	if err != nil {
		httpx.InternalError(w, "list counters", err)
		return
	}
	if len(counters) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "counter_data_not_found", "Counter data was not found for the target hero.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, counters)
}
