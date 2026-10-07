package hero

import (
	"context"
	"errors"
	"net/http"

	"github.com/yeremi777/mlbb-collector/internal/httpx"
)

type store interface {
	List(ctx context.Context) ([]Hero, error)
	Get(ctx context.Context, uid string) (Hero, error)
}

// Handler serves the hero catalog.
type Handler struct{ heroes store }

// NewHandler serves heroes from the given store.
func NewHandler(heroes store) Handler { return Handler{heroes: heroes} }

// Register adds the hero routes to mux.
func (h Handler) Register(mux httpx.Mux) {
	mux.HandleFunc("GET /api/heroes", h.list)
	mux.HandleFunc("GET /api/heroes/{heroId}", h.get)
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	heroes, err := h.heroes.List(r.Context())
	if err != nil {
		httpx.InternalError(w, "list heroes", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, List(heroes, ParseListQuery(r.URL.Query())))
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	found, err := h.heroes.Get(r.Context(), r.PathValue("heroId"))
	switch {
	case errors.Is(err, ErrNotFound):
		WriteNotFound(w)
	case err != nil:
		httpx.InternalError(w, "get hero", err)
	default:
		httpx.WriteJSON(w, http.StatusOK, found)
	}
}

// WriteNotFound answers 404 hero_not_found.
func WriteNotFound(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "hero_not_found", "Hero was not found in the dataset.")
}
