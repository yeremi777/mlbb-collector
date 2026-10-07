package counter

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Repository reads Counters from public.counters and public.counter_proofs.
type Repository struct{ db database.Querier }

// NewRepository reads through db.
func NewRepository(db database.Querier) Repository { return Repository{db: db} }

// ForTarget returns the Counters of the target hero ordered by Counter hero
// ID, each with its Proofs ordered by Proof ID.
func (r Repository) ForTarget(ctx context.Context, target string) ([]WithHero, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.reasons, c.counter_types, `+hero.Columns("h")+`
		  FROM counters c JOIN heroes h ON h.uid = c.counter_hero_id
		 WHERE c.target_hero_id = $1
		 ORDER BY c.counter_hero_id`, target)
	if err != nil {
		return nil, err
	}
	counters, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (WithHero, error) {
		c := WithHero{TargetHeroID: target, Proof: []Proof{}}
		h := &c.CounterHero
		err := row.Scan(&c.Reasons, &c.CounterTypes, &h.UID, &h.MLID, &h.Name, &h.Roles, &h.Lanes, &h.Images)
		return c, err
	})
	if err != nil || len(counters) == 0 {
		return counters, err
	}

	byPartner := make(map[string]int, len(counters))
	for i, c := range counters {
		byPartner[c.CounterHero.UID] = i
	}
	rows, err = r.db.Query(ctx, `
		SELECT counter_hero_id, id, category, priority, impact, summary, works_best_when, failure_cases
		  FROM counter_proofs
		 WHERE target_hero_id = $1
		 ORDER BY id`, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var partner string
		var p Proof
		if err := rows.Scan(&partner, &p.ID, &p.Category, &p.Priority, &p.Impact, &p.Summary, &p.WorksBestWhen, &p.FailureCases); err != nil {
			return nil, err
		}
		i := byPartner[partner]
		counters[i].Proof = append(counters[i].Proof, p)
	}
	return counters, rows.Err()
}
