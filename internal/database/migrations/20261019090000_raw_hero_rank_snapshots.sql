-- +goose Up
CREATE SCHEMA raw;

CREATE TABLE raw.hero_rank_snapshots (
    id               BIGSERIAL PRIMARY KEY,
    main_heroid      INTEGER NOT NULL,
    rank_tier        TEXT NOT NULL CHECK (rank_tier IN ('all','epic','legend','mythic','honor','glory')),
    window_days      SMALLINT NOT NULL CHECK (window_days IN (1,3,7,15,30)),
    snapshot_date    DATE NOT NULL,
    win_rate         NUMERIC(9,6) NOT NULL,
    appearance_share NUMERIC(9,6) NOT NULL,
    ban_rate         NUMERIC(9,6) NOT NULL,
    payload          JSONB NOT NULL,
    fetched_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX hero_rank_snapshots_lookup_idx
    ON raw.hero_rank_snapshots (main_heroid, rank_tier, window_days, snapshot_date);

-- +goose Down
DROP TABLE raw.hero_rank_snapshots;
DROP SCHEMA raw;
