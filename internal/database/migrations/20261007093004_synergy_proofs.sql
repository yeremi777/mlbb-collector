-- +goose Up
CREATE TABLE public.synergy_proofs (
    id              TEXT PRIMARY KEY,
    anchor_hero_id  TEXT NOT NULL,
    synergy_hero_id TEXT NOT NULL,
    category        TEXT NOT NULL CHECK (category IN (
        'skill-interaction','crowd-control-chain','engage-follow-up','setup-combo',
        'damage-amplification','protection','peel','frontline-enabler','mobility-enabler',
        'vision-setup','healing-sustain','shielding','poke-siege','pickoff-combo',
        'teamfight-combo','objective-control','laning-synergy','game-phase',
        'positioning-requirement','cooldown-window','execution-difficulty')),
    priority        TEXT NOT NULL CHECK (priority IN ('primary','secondary','condition')),
    impact          TEXT NOT NULL CHECK (impact IN ('high','medium','low')),
    summary         TEXT NOT NULL CHECK (summary <> ''),
    works_best_when TEXT[] NOT NULL CHECK (cardinality(works_best_when) >= 1),
    failure_cases   TEXT[] NOT NULL CHECK (cardinality(failure_cases) >= 1),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (anchor_hero_id, synergy_hero_id)
        REFERENCES public.synergies (anchor_hero_id, synergy_hero_id) ON DELETE CASCADE
);
CREATE INDEX synergy_proofs_matchup_idx ON public.synergy_proofs (anchor_hero_id, synergy_hero_id);

-- +goose Down
DROP TABLE public.synergy_proofs;
