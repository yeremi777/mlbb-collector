# views

## Goal

The `staging` views turn the raw zone into one clean row per fact, and the `marts` views give each hero's current standing per Rank tier and Window, with its Patch ([ADR-0006](../adr/0006-patches-join-snapshots-at-read-time.md)) and its Measured counters.

## Non-goals

- No table, materialized view, or scheduled refresh. Every object here is a plain view.
- No HTTP route and no Go reader. A view's first consumer brings its own spec.
- No change to `raw` or `public`.
- No Measured counter mixed into a Counter, or into any Score.

## Rules

- Nothing outside `staging` reads `raw`.
- `raw` keeps upstream spelling (`main_heroid`); `staging` and `marts` use `main_hero_id`, `target_heroid`, and `counter_heroid`.
- A `marts` view joins `public.heroes` with a `LEFT JOIN` on `mlid`, so a hero Moonton lists before the authored dataset carries it appears with a NULL Hero ID and name.
- A value read from `payload` that is missing, null, or not the expected JSON type yields no row, never an error for the whole view.

## staging

| View | One row per | Columns and rules |
|---|---|---|
| `hero_rank_snapshot_latest` | hero, Rank tier, Window, date | every `raw.hero_rank_snapshots` column with `main_heroid` as `main_hero_id`; the newest `fetched_at` wins, then the highest `id` |
| `hero_rank_daily` | hero, Rank tier, Window, date | `hero_rank_snapshot_latest` without `id` and `payload` |
| `hero_counter_daily` | Measured counter, source, Rank tier, Window, date | `target_heroid`, `counter_heroid`, `win_rate_delta`, `source`, `source_rank`, `rank_tier`, `window_days`, `snapshot_date` |
| `patch_calendar` | Patch | `release_date`, `version`, `highlights` from `raw.patches` |

`hero_counter_daily` reads `payload->'data'`:

- Each `sub_hero` entry with a positive `increase_win_rate` is a row where the entry's `heroid` is the Counter hero and the Snapshot's hero is the Target hero, with `source` `sub_hero`.
- Each `sub_hero_last` entry with a negative `increase_win_rate` is a row where the Snapshot's hero is the Counter hero and the entry's `heroid` is the Target hero, with the delta negated and `source` `sub_hero_last`.
- `source_rank` is the entry's 1-based position in its array. Every row has `win_rate_delta > 0`.

## marts

### hero_current

One row per hero, Rank tier, and Window, from the hero's newest Snapshot date in `hero_rank_daily`.

| Column | Value |
|---|---|
| `main_hero_id`, `hero_uid`, `hero_name` | the hero; `hero_uid` and `hero_name` NULL when not in `public.heroes` |
| `rank_tier`, `window_days`, `snapshot_date` | |
| `snapshot_age_days` | `current_date - snapshot_date` |
| `patch_as_of` | the version of the latest Patch released on or before `snapshot_date` |
| `days_since_patch` | `snapshot_date` minus that Patch's release date |
| `window_crosses_patch` | true when `snapshot_date - window_days` is before that Patch's release date |
| `win_rate`, `appearance_share`, `ban_rate` | |
| `win_rate_delta`, `appearance_share_delta`, `ban_rate_delta`, `prev_snapshot_date` | change from the hero's previous Snapshot date for the same Rank tier and Window; NULL when there is none |
| `win_rate_rank`, `appearance_rank` | `rank()` by descending value among rows with the same Rank tier, Window, and `snapshot_date` |

The three Patch columns are NULL for a Snapshot dated before the earliest known Patch.

### hero_counter_current

The rows of `hero_counter_daily` at the newest Snapshot date per Target hero, Rank tier, and Window, with `target_uid`, `target_name`, `counter_uid`, and `counter_name` joined from `public.heroes`.

## Acceptance criteria

- AC-1: Migrations create the six views; `make migrate-reset && make migrate-up` succeeds.
- AC-2: With one day forced twice, `hero_rank_snapshot_latest` holds one row per hero, Rank tier, Window, and date, carrying the second fetch's values.
- AC-3: `hero_counter_daily` has no row with `win_rate_delta <= 0`, and a `sub_hero_last` row appears with its hero ids swapped relative to the payload.
- AC-4: A Snapshot whose `payload` has `sub_hero` null, or a scalar, yields no `hero_counter_daily` rows for that hero and no error.
- AC-5: `hero_current` has 30 rows per hero in the newest Snapshot; a hero absent from `public.heroes` appears with NULL `hero_uid`.
- AC-6: With Patches released on D and D-3, a Snapshot dated D-1 has the D-3 Patch as `patch_as_of`, `days_since_patch` 2, and `window_crosses_patch` true for the 3-day Window and above, false for the 1-day Window. A Snapshot dated before every Patch has the three Patch columns NULL.
- AC-7: Ranks in `hero_current` never compare rows of different Snapshot dates.
- AC-8: View tests insert fixture rows into `raw` and `public` inside a rolled-back transaction and assert AC-2 to AC-7.

## Verification

```bash
set -a; . ./.env; set +a; export PGHOST=$DB_HOST PGPORT=$DB_PORT PGDATABASE=$DB_NAME PGUSER=$DB_USERNAME PGPASSWORD=$DB_PASSWORD PGSSLMODE=$DB_SSLMODE   # psql below reads these
go vet ./...
go test ./...                                                      # AC-2 to AC-8
make migrate-reset && make migrate-up                              # AC-1, run by the user
psql -tAc "SELECT count(*) FROM staging.hero_counter_daily WHERE win_rate_delta <= 0"   # AC-3, returns 0
psql -tAc "SELECT main_hero_id, count(*) FROM marts.hero_current WHERE snapshot_date = (SELECT max(snapshot_date) FROM marts.hero_current) GROUP BY 1 HAVING count(*) <> 30"   # AC-5, returns nothing
```
