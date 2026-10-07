// Package seed makes the public tables match an authored dataset exactly
// (ADR-0001): rows the dataset no longer holds are deleted, and every row it
// holds is upserted, rewriting a row only when one of its values changed.
package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

// Sync writes ds into the public tables within tx. The caller owns the
// transaction, so a failure anywhere leaves every table as it was once the
// caller rolls back.
func Sync(ctx context.Context, tx pgx.Tx, ds dataset.Dataset) error {
	if err := syncHeroes(ctx, tx, ds.Heroes); err != nil {
		return fmt.Errorf("heroes: %w", err)
	}
	if err := syncCounters(ctx, tx, ds.Counters); err != nil {
		return fmt.Errorf("counters: %w", err)
	}
	if err := syncSynergies(ctx, tx, ds.Synergies); err != nil {
		return fmt.Errorf("synergies: %w", err)
	}
	return nil
}

const (
	deleteHeroes = `DELETE FROM heroes WHERE uid <> ALL($1)`
	upsertHero   = `
		INSERT INTO heroes (uid, mlid, name, roles, lanes, images)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		ON CONFLICT (uid) DO UPDATE
		SET mlid = EXCLUDED.mlid, name = EXCLUDED.name, roles = EXCLUDED.roles,
		    lanes = EXCLUDED.lanes, images = EXCLUDED.images, updated_at = now()
		WHERE (heroes.mlid, heroes.name, heroes.roles, heroes.lanes, heroes.images)
		      IS DISTINCT FROM
		      (EXCLUDED.mlid, EXCLUDED.name, EXCLUDED.roles, EXCLUDED.lanes, EXCLUDED.images)`
)

func syncHeroes(ctx context.Context, tx pgx.Tx, heroes []hero.Hero) error {
	uids := make([]string, len(heroes))
	batch := &pgx.Batch{}
	for i, h := range heroes {
		uids[i] = h.UID
		batch.Queue(upsertHero, h.UID, h.MLID, h.Name, h.Roles, h.Lanes, string(h.Images))
	}
	// Deleting first lets a departed hero's Moonton ID pass to a new hero.
	if _, err := tx.Exec(ctx, deleteHeroes, uids); err != nil {
		return err
	}
	return tx.SendBatch(ctx, batch).Close()
}

const (
	deleteCounterProofs = `DELETE FROM counter_proofs WHERE id <> ALL($1)`
	deleteCounters      = `
		DELETE FROM counters
		WHERE (target_hero_id, counter_hero_id) NOT IN (SELECT * FROM unnest($1::text[], $2::text[]))`
	upsertCounter = `
		INSERT INTO counters (target_hero_id, counter_hero_id, reasons, counter_types)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (target_hero_id, counter_hero_id) DO UPDATE
		SET reasons = EXCLUDED.reasons, counter_types = EXCLUDED.counter_types, updated_at = now()
		WHERE (counters.reasons, counters.counter_types)
		      IS DISTINCT FROM (EXCLUDED.reasons, EXCLUDED.counter_types)`
	upsertCounterProof = `
		INSERT INTO counter_proofs (id, target_hero_id, counter_hero_id, category, priority, impact,
		                            summary, works_best_when, failure_cases)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE
		SET target_hero_id = EXCLUDED.target_hero_id, counter_hero_id = EXCLUDED.counter_hero_id,
		    category = EXCLUDED.category, priority = EXCLUDED.priority, impact = EXCLUDED.impact,
		    summary = EXCLUDED.summary, works_best_when = EXCLUDED.works_best_when,
		    failure_cases = EXCLUDED.failure_cases, updated_at = now()
		WHERE (counter_proofs.target_hero_id, counter_proofs.counter_hero_id, counter_proofs.category,
		       counter_proofs.priority, counter_proofs.impact, counter_proofs.summary,
		       counter_proofs.works_best_when, counter_proofs.failure_cases)
		      IS DISTINCT FROM
		      (EXCLUDED.target_hero_id, EXCLUDED.counter_hero_id, EXCLUDED.category,
		       EXCLUDED.priority, EXCLUDED.impact, EXCLUDED.summary,
		       EXCLUDED.works_best_when, EXCLUDED.failure_cases)`
)

func syncCounters(ctx context.Context, tx pgx.Tx, counters []counter.Counter) error {
	targets := make([]string, len(counters))
	partners := make([]string, len(counters))
	proofIDs := []string{} // never nil: <> ALL(NULL) would delete nothing
	batch := &pgx.Batch{}
	for i, c := range counters {
		targets[i], partners[i] = c.TargetHeroID, c.CounterHeroID
		batch.Queue(upsertCounter, c.TargetHeroID, c.CounterHeroID, c.Reasons, c.CounterTypes)
		for _, p := range c.Proof {
			proofIDs = append(proofIDs, p.ID)
			batch.Queue(upsertCounterProof, p.ID, c.TargetHeroID, c.CounterHeroID, p.Category,
				p.Priority, p.Impact, p.Summary, p.WorksBestWhen, p.FailureCases)
		}
	}
	// Proofs go first: deleting a departed Counter cascades to its Proofs, and
	// a Proof that moved to another Counter is upserted again below.
	if _, err := tx.Exec(ctx, deleteCounterProofs, proofIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteCounters, targets, partners); err != nil {
		return err
	}
	return tx.SendBatch(ctx, batch).Close()
}

const (
	deleteSynergyProofs = `DELETE FROM synergy_proofs WHERE id <> ALL($1)`
	deleteSynergies     = `
		DELETE FROM synergies
		WHERE (anchor_hero_id, synergy_hero_id) NOT IN (SELECT * FROM unnest($1::text[], $2::text[]))`
	upsertSynergy = `
		INSERT INTO synergies (anchor_hero_id, synergy_hero_id, reasons, synergy_types)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (anchor_hero_id, synergy_hero_id) DO UPDATE
		SET reasons = EXCLUDED.reasons, synergy_types = EXCLUDED.synergy_types, updated_at = now()
		WHERE (synergies.reasons, synergies.synergy_types)
		      IS DISTINCT FROM (EXCLUDED.reasons, EXCLUDED.synergy_types)`
	upsertSynergyProof = `
		INSERT INTO synergy_proofs (id, anchor_hero_id, synergy_hero_id, category, priority, impact,
		                            summary, works_best_when, failure_cases)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE
		SET anchor_hero_id = EXCLUDED.anchor_hero_id, synergy_hero_id = EXCLUDED.synergy_hero_id,
		    category = EXCLUDED.category, priority = EXCLUDED.priority, impact = EXCLUDED.impact,
		    summary = EXCLUDED.summary, works_best_when = EXCLUDED.works_best_when,
		    failure_cases = EXCLUDED.failure_cases, updated_at = now()
		WHERE (synergy_proofs.anchor_hero_id, synergy_proofs.synergy_hero_id, synergy_proofs.category,
		       synergy_proofs.priority, synergy_proofs.impact, synergy_proofs.summary,
		       synergy_proofs.works_best_when, synergy_proofs.failure_cases)
		      IS DISTINCT FROM
		      (EXCLUDED.anchor_hero_id, EXCLUDED.synergy_hero_id, EXCLUDED.category,
		       EXCLUDED.priority, EXCLUDED.impact, EXCLUDED.summary,
		       EXCLUDED.works_best_when, EXCLUDED.failure_cases)`
)

func syncSynergies(ctx context.Context, tx pgx.Tx, synergies []synergy.Synergy) error {
	anchors := make([]string, len(synergies))
	partners := make([]string, len(synergies))
	proofIDs := []string{} // never nil: <> ALL(NULL) would delete nothing
	batch := &pgx.Batch{}
	for i, s := range synergies {
		anchors[i], partners[i] = s.AnchorHeroID, s.SynergyHeroID
		batch.Queue(upsertSynergy, s.AnchorHeroID, s.SynergyHeroID, s.Reasons, s.SynergyTypes)
		for _, p := range s.Proof {
			proofIDs = append(proofIDs, p.ID)
			batch.Queue(upsertSynergyProof, p.ID, s.AnchorHeroID, s.SynergyHeroID, p.Category,
				p.Priority, p.Impact, p.Summary, p.WorksBestWhen, p.FailureCases)
		}
	}
	if _, err := tx.Exec(ctx, deleteSynergyProofs, proofIDs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteSynergies, anchors, partners); err != nil {
		return err
	}
	return tx.SendBatch(ctx, batch).Close()
}
