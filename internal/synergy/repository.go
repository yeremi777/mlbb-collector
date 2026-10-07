package synergy

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Repository reads Synergies from public.synergies and public.synergy_proofs.
type Repository struct{ db database.Querier }

// NewRepository reads through db.
func NewRepository(db database.Querier) Repository { return Repository{db: db} }

// ForAnchor returns the Synergies of the anchor hero ordered by Synergy hero
// ID, each with its Proofs ordered by Proof ID.
func (r Repository) ForAnchor(ctx context.Context, anchor string) ([]WithHero, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.reasons, c.synergy_types, `+hero.Columns("h")+`
		  FROM synergies c JOIN heroes h ON h.uid = c.synergy_hero_id
		 WHERE c.anchor_hero_id = $1
		 ORDER BY c.synergy_hero_id`, anchor)
	if err != nil {
		return nil, err
	}
	synergies, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (WithHero, error) {
		c := WithHero{AnchorHeroID: anchor, Proof: []Proof{}}
		h := &c.SynergyHero
		err := row.Scan(&c.Reasons, &c.SynergyTypes, &h.UID, &h.MLID, &h.Name, &h.Roles, &h.Lanes, &h.Images)
		return c, err
	})
	if err != nil || len(synergies) == 0 {
		return synergies, err
	}

	byPartner := make(map[string]int, len(synergies))
	for i, c := range synergies {
		byPartner[c.SynergyHero.UID] = i
	}
	rows, err = r.db.Query(ctx, `
		SELECT synergy_hero_id, id, category, priority, impact, summary, works_best_when, failure_cases
		  FROM synergy_proofs
		 WHERE anchor_hero_id = $1
		 ORDER BY id`, anchor)
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
		synergies[i].Proof = append(synergies[i].Proof, p)
	}
	return synergies, rows.Err()
}
