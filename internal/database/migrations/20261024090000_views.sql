-- +goose Up
CREATE SCHEMA staging;
CREATE SCHEMA marts;

-- One row per hero, Rank tier, Window, and date; the newest fetch wins.
CREATE VIEW staging.hero_rank_snapshot_latest AS
SELECT DISTINCT ON (main_heroid, rank_tier, window_days, snapshot_date)
       id,
       main_heroid AS main_hero_id,
       rank_tier, window_days, snapshot_date,
       win_rate, appearance_share, ban_rate, payload, fetched_at
  FROM raw.hero_rank_snapshots
 ORDER BY main_heroid, rank_tier, window_days, snapshot_date, fetched_at DESC, id DESC;

-- The latest Snapshot rows without id and payload.
CREATE VIEW staging.hero_rank_daily AS
SELECT main_hero_id, rank_tier, window_days, snapshot_date,
       win_rate, appearance_share, ban_rate, fetched_at
  FROM staging.hero_rank_snapshot_latest;

-- Measured counters oriented so counter_heroid beats target_heroid; sub_hero_last rows are flipped and negated.
CREATE VIEW staging.hero_counter_daily AS
WITH entries AS (
    SELECT l.main_hero_id, l.rank_tier, l.window_days, l.snapshot_date,
           k.source, s.ord, e.heroid, e.delta
      FROM staging.hero_rank_snapshot_latest l
     CROSS JOIN (VALUES ('sub_hero'), ('sub_hero_last')) AS k(source)
     CROSS JOIN LATERAL jsonb_array_elements(
            CASE WHEN jsonb_typeof(l.payload->'data'->k.source) = 'array'
                 THEN l.payload->'data'->k.source
                 ELSE '[]'::jsonb
            END) WITH ORDINALITY AS s(elem, ord)
     CROSS JOIN LATERAL (
            SELECT CASE WHEN jsonb_typeof(s.elem->'heroid') = 'number' AND s.elem->>'heroid' ~ '^[0-9]{1,9}$'
                        THEN (s.elem->>'heroid')::integer
                   END AS heroid,
                   CASE WHEN jsonb_typeof(s.elem->'increase_win_rate') = 'number'
                        THEN (s.elem->>'increase_win_rate')::numeric
                   END AS delta
     ) e
)
SELECT CASE source WHEN 'sub_hero' THEN main_hero_id ELSE heroid END AS target_heroid,
       CASE source WHEN 'sub_hero' THEN heroid ELSE main_hero_id END AS counter_heroid,
       abs(delta)    AS win_rate_delta,
       source,
       ord::smallint AS source_rank,
       rank_tier, window_days, snapshot_date
  FROM entries
 WHERE heroid IS NOT NULL
   AND ((source = 'sub_hero' AND delta > 0) OR (source = 'sub_hero_last' AND delta < 0));

CREATE VIEW staging.patch_calendar AS
SELECT release_date, version, highlights
  FROM raw.patches;

-- Each hero's newest Snapshot per Rank tier and Window, with movement since the previous date, ranks among the same date, and its Patch.
CREATE VIEW marts.hero_current AS
WITH movement AS (
    SELECT d.main_hero_id, d.rank_tier, d.window_days, d.snapshot_date,
           d.win_rate, d.appearance_share, d.ban_rate,
           lag(d.win_rate)         OVER w AS prev_win_rate,
           lag(d.appearance_share) OVER w AS prev_appearance_share,
           lag(d.ban_rate)         OVER w AS prev_ban_rate,
           lag(d.snapshot_date)    OVER w AS prev_snapshot_date,
           row_number() OVER (PARTITION BY d.main_hero_id, d.rank_tier, d.window_days
                              ORDER BY d.snapshot_date DESC) AS recency
      FROM staging.hero_rank_daily d
    WINDOW w AS (PARTITION BY d.main_hero_id, d.rank_tier, d.window_days ORDER BY d.snapshot_date)
)
SELECT m.main_hero_id,
       h.uid  AS hero_uid,
       h.name AS hero_name,
       m.rank_tier, m.window_days, m.snapshot_date,
       p.version                                            AS patch_as_of,
       (m.snapshot_date - p.release_date)                   AS days_since_patch,
       ((m.snapshot_date - m.window_days) < p.release_date) AS window_crosses_patch,
       m.win_rate, m.appearance_share, m.ban_rate,
       m.win_rate         - m.prev_win_rate         AS win_rate_delta,
       m.appearance_share - m.prev_appearance_share AS appearance_share_delta,
       m.ban_rate         - m.prev_ban_rate         AS ban_rate_delta,
       m.prev_snapshot_date,
       rank() OVER (PARTITION BY m.rank_tier, m.window_days, m.snapshot_date ORDER BY m.win_rate DESC)         AS win_rate_rank,
       rank() OVER (PARTITION BY m.rank_tier, m.window_days, m.snapshot_date ORDER BY m.appearance_share DESC) AS appearance_rank
  FROM movement m
  LEFT JOIN public.heroes h ON h.mlid = m.main_hero_id
  LEFT JOIN LATERAL (
       SELECT pc.version, pc.release_date
         FROM staging.patch_calendar pc
        WHERE pc.release_date <= m.snapshot_date
        ORDER BY pc.release_date DESC
        LIMIT 1
  ) p ON TRUE
 WHERE m.recency = 1;

-- Measured counters at the newest Snapshot date per Target hero, Rank tier, and Window, with both heroes named.
CREATE VIEW marts.hero_counter_current AS
WITH latest AS (
    SELECT c.*,
           dense_rank() OVER (PARTITION BY c.target_heroid, c.rank_tier, c.window_days
                              ORDER BY c.snapshot_date DESC) AS recency
      FROM staging.hero_counter_daily c
)
SELECT l.target_heroid,
       th.uid  AS target_uid,
       th.name AS target_name,
       l.counter_heroid,
       ch.uid  AS counter_uid,
       ch.name AS counter_name,
       l.win_rate_delta, l.source, l.source_rank,
       l.rank_tier, l.window_days, l.snapshot_date
  FROM latest l
  LEFT JOIN public.heroes th ON th.mlid = l.target_heroid
  LEFT JOIN public.heroes ch ON ch.mlid = l.counter_heroid
 WHERE l.recency = 1;

-- +goose Down
DROP VIEW marts.hero_counter_current;
DROP VIEW marts.hero_current;
DROP VIEW staging.patch_calendar;
DROP VIEW staging.hero_counter_daily;
DROP VIEW staging.hero_rank_daily;
DROP VIEW staging.hero_rank_snapshot_latest;
DROP SCHEMA marts;
DROP SCHEMA staging;
