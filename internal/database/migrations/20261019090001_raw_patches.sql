-- +goose Up
CREATE TABLE raw.patches (
    release_date DATE PRIMARY KEY,
    version      TEXT NOT NULL,
    highlights   TEXT[] NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE raw.patches;
