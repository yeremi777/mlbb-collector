package stats

import (
	"context"
	"net/http"

	"github.com/yeremi777/mlbb-collector/internal/httpx"
)

type store interface {
	Board(ctx context.Context, q Query) (Board, error)
}

// Handler serves the meta board.
type Handler struct{ boards store }

// NewHandler serves Boards from the given store.
func NewHandler(boards store) Handler { return Handler{boards: boards} }

// Register adds the stats route to mux.
func (h Handler) Register(mux httpx.Mux) {
	mux.HandleFunc("GET /api/stats/heroes", h.board)
}

func (h Handler) board(w http.ResponseWriter, r *http.Request) {
	board, err := h.boards.Board(r.Context(), ParseQuery(r.URL.Query()))
	if err != nil {
		httpx.InternalError(w, "get stats board", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, board)
}
