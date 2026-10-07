-- +goose Up
CREATE TABLE public.counters (
    target_hero_id  TEXT NOT NULL REFERENCES public.heroes (uid) ON DELETE CASCADE,
    counter_hero_id TEXT NOT NULL REFERENCES public.heroes (uid) ON DELETE CASCADE,
    reasons         TEXT[] NOT NULL CHECK (cardinality(reasons) >= 1),
    counter_types   TEXT[] NOT NULL CHECK (cardinality(counter_types) >= 1),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (target_hero_id, counter_hero_id),
    CHECK (target_hero_id <> counter_hero_id)
);

-- +goose Down
DROP TABLE public.counters;
