package synergy

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

type synergyStore interface {
	ForAnchor(ctx context.Context, anchor string) ([]WithHero, error)
}

// Handler serves the Synergies of a hero.
type Handler struct {
	heroes    heroStore
	synergies synergyStore
}

// NewHandler serves Synergies from synergies, checking heroes exist in heroes.
func NewHandler(heroes heroStore, synergies synergyStore) Handler {
	return Handler{heroes: heroes, synergies: synergies}
}

// Register adds the Synergy route to mux.
func (h Handler) Register(mux httpx.Mux) {
	mux.HandleFunc("GET /api/heroes/{heroId}/synergies", h.list)
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	anchor := r.PathValue("heroId")
	if _, err := h.heroes.Get(r.Context(), anchor); errors.Is(err, hero.ErrNotFound) {
		hero.WriteNotFound(w)
		return
	} else if err != nil {
		httpx.InternalError(w, "get hero", err)
		return
	}
	synergies, err := h.synergies.ForAnchor(r.Context(), anchor)
	if err != nil {
		httpx.InternalError(w, "list synergies", err)
		return
	}
	if len(synergies) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "synergy_data_not_found", "Synergy data was not found for the anchor hero.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, synergies)
}
