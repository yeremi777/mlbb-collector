package hero

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database"
)

// ErrNotFound reports a Hero ID that names no hero.
var ErrNotFound = errors.New("hero not found")

// Repository reads heroes from public.heroes.
type Repository struct{ db database.Querier }

// NewRepository reads through db.
func NewRepository(db database.Querier) Repository { return Repository{db: db} }

// Columns is the column list Scan reads, in order, for a heroes table
// aliased as alias.
func Columns(alias string) string {
	return fmt.Sprintf("%[1]s.uid, %[1]s.mlid, %[1]s.name, %[1]s.roles, %[1]s.lanes, %[1]s.images", alias)
}

// Scan reads one hero from the columns Columns lists.
func Scan(row pgx.Row) (Hero, error) {
	var h Hero
	err := row.Scan(&h.UID, &h.MLID, &h.Name, &h.Roles, &h.Lanes, &h.Images)
	return h, err
}

// List returns every hero in Moonton ID order.
func (r Repository) List(ctx context.Context) ([]Hero, error) {
	rows, err := r.db.Query(ctx, "SELECT "+Columns("h")+" FROM heroes h ORDER BY h.mlid")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Hero, error) { return Scan(row) })
}

// Get returns the hero with the given Hero ID, or ErrNotFound.
func (r Repository) Get(ctx context.Context, uid string) (Hero, error) {
	h, err := Scan(r.db.QueryRow(ctx, "SELECT "+Columns("h")+" FROM heroes h WHERE h.uid = $1", uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return Hero{}, ErrNotFound
	}
	return h, err
}
