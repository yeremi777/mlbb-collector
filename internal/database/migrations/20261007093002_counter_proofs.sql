-- +goose Up
CREATE TABLE public.counter_proofs (
    id              TEXT PRIMARY KEY,
    target_hero_id  TEXT NOT NULL,
    counter_hero_id TEXT NOT NULL,
    category        TEXT NOT NULL CHECK (category IN (
        'skill-interaction','crowd-control-counter','damage-type-advantage','item-power-spike',
        'mobility-advantage','range-advantage','kiting','sustain-anti-sustain',
        'positioning-requirement','cooldown-window','vision-awareness','teamfight-role-counter',
        'game-phase','execution-difficulty')),
    priority        TEXT NOT NULL CHECK (priority IN ('primary','secondary','condition')),
    impact          TEXT NOT NULL CHECK (impact IN ('high','medium','low')),
    summary         TEXT NOT NULL CHECK (summary <> ''),
    works_best_when TEXT[] NOT NULL CHECK (cardinality(works_best_when) >= 1),
    failure_cases   TEXT[] NOT NULL CHECK (cardinality(failure_cases) >= 1),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (target_hero_id, counter_hero_id)
        REFERENCES public.counters (target_hero_id, counter_hero_id) ON DELETE CASCADE
);
CREATE INDEX counter_proofs_matchup_idx ON public.counter_proofs (target_hero_id, counter_hero_id);

-- +goose Down
DROP TABLE public.counter_proofs;
