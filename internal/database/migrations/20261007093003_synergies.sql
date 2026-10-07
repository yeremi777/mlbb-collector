-- +goose Up
CREATE TABLE public.synergies (
    anchor_hero_id  TEXT NOT NULL REFERENCES public.heroes (uid) ON DELETE CASCADE,
    synergy_hero_id TEXT NOT NULL REFERENCES public.heroes (uid) ON DELETE CASCADE,
    reasons         TEXT[] NOT NULL CHECK (cardinality(reasons) >= 1),
    synergy_types   TEXT[] NOT NULL CHECK (cardinality(synergy_types) >= 1),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (anchor_hero_id, synergy_hero_id),
    CHECK (anchor_hero_id <> synergy_hero_id)
);

-- +goose Down
DROP TABLE public.synergies;
