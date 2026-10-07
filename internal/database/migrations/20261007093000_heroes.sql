-- +goose Up
CREATE TABLE public.heroes (
    uid        TEXT PRIMARY KEY,
    mlid       INTEGER NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    roles      TEXT[] NOT NULL CHECK (cardinality(roles) >= 1
                   AND roles <@ ARRAY['tank','fighter','assassin','mage','marksman','support']),
    lanes      TEXT[] NOT NULL CHECK (lanes <@ ARRAY['gold','exp','mid','roam','jungle']),
    images     JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE public.heroes;
