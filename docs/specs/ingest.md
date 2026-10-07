# ingest

## Goal

`cmd/ingest` lands each day's hero statistics from Moonton in `raw.hero_rank_snapshots` ([ADR-0004](../adr/0004-hero-snapshots-are-append-only.md)) and keeps `raw.patches` equal to Liquipedia's patch calendar ([ADR-0005](../adr/0005-patch-calendar-is-keyed-on-release-date.md)), runnable by hand and on a schedule.

## Non-goals

- No `staging` or `marts` view. `views` owns them.
- No Patch written into a Snapshot ([ADR-0006](../adr/0006-patches-join-snapshots-at-read-time.md)), and no foreign key from `raw` to `public`.
- No HTTP route. Nothing ingested is served by the API.
- No source beyond Moonton's rank-statistics endpoint and Liquipedia's `Portal:Patches`.
- No backfill. Moonton serves the current day only.

## Commands

| Command | Does |
|---|---|
| `ingest stats [-date YYYY-MM-DD] [-force]` | fetches hero statistics |
| `ingest patches` | fetches the patch calendar |
| `ingest all [-date YYYY-MM-DD] [-force]` | `patches`, then `stats`; a `patches` failure is logged and `stats` still runs |

- `-date` labels the Snapshot with that date instead of today in `INGEST_TIMEZONE`, for a run that started after midnight on behalf of the previous day.
- `-force` fetches every combination even when the date already has rows.
- Exit 0 when everything run succeeded, 1 when any combination or the calendar failed, and 2 on a bad command or flag, with the usage text on stderr.
- Every run logs one structured summary line per feed: date, combinations fetched, skipped, and failed, rows inserted, Patches fetched and changed.

## Hero statistics

### Upstream

- `POST https://api.gms.moontontech.com/api/gms/source/2669606/<endpoint>`, no authentication, with `Origin` and `Referer` set to `https://www.mobilelegends.com`.
- One endpoint per Window: 1 → `2756567`, 3 → `2756568`, 7 → `2756569`, 15 → `2756565`, 30 → `2756570`.
- `bigrank` per Rank tier: all → `101`, epic → `5`, legend → `6`, mythic → `7`, honor → `8`, glory → `9`.
- The body is `pageSize` 200, `pageIndex` 1, filters `bigrank` and `match_type` 0, sorted by `main_hero_win_rate` descending, and `"fields": []` so the server returns its full default projection.
- An error arrives as HTTP 200 with `code` non-zero or `records` null. Success is judged on the body, never on the status.

### Run

- A run covers the 30 combinations of five Windows and six Rank tiers, Windows ascending, then Rank tiers in the order above.
- Without `-force`, a combination that already has rows for the date is skipped, and a run whose date is complete makes no request.
- Requests are sequential, 2s apart, including after a failure.
- A response is accepted only when `code` is 0, `data.total` is above 100, and `data.records` holds exactly `data.total` records.
  - `code` non-zero, empty `records`, or `total` 100 or below fails the combination at once.
  - A transport error, a non-200 status, or a record count different from `total` is retried once after 1s.
- Each accepted combination is inserted in its own transaction, so a later failure keeps it.
- A failed combination is logged and the run moves on; it exits 1 after the last combination.

### raw.hero_rank_snapshots

| Column | Type | Value |
|---|---|---|
| `id` | `bigserial` PK | |
| `main_heroid` | `integer NOT NULL` | the record's Moonton ID |
| `rank_tier` | `text NOT NULL` | `CHECK IN ('all','epic','legend','mythic','honor','glory')` |
| `window_days` | `smallint NOT NULL` | `CHECK IN (1,3,7,15,30)` |
| `snapshot_date` | `date NOT NULL` | the run's date |
| `win_rate` | `numeric(9,6) NOT NULL` | `main_hero_win_rate` |
| `appearance_share` | `numeric(9,6) NOT NULL` | `main_hero_appearance_rate` |
| `ban_rate` | `numeric(9,6) NOT NULL` | `main_hero_ban_rate` |
| `payload` | `jsonb NOT NULL` | the record exactly as received, including `sub_hero` and `sub_hero_last` |
| `fetched_at` | `timestamptz NOT NULL DEFAULT now()` | |

Index on `(main_heroid, rank_tier, window_days, snapshot_date)`. Rows are only ever inserted.

## Patch calendar

### Upstream

- `GET https://liquipedia.net/mobilelegends/api.php?action=parse&page=Portal:Patches&format=json&prop=text`, one request per run.
- A Patch is a table row whose first cell reads `Patch <version>` and whose second cell is a date such as `January 2, 2006`. Every other row is skipped.
- Highlights are the row's third-cell list items, one entry each with whitespace collapsed; nested items fold into their parent; a plain-text cell is one entry; an empty or missing cell is none.
- A 429 or 5xx, a transport error, or a body with no tables is retried once, after 30s or the `Retry-After` value if longer. A MediaWiki error, another status, or a page with tables but no Patch rows fails at once.
- The whole calendar is fetched and parsed before the database is touched, so a failure leaves `raw.patches` as it was.

### raw.patches

| Column | Type | Value |
|---|---|---|
| `release_date` | `date` PK | |
| `version` | `text NOT NULL` | |
| `highlights` | `text[] NOT NULL DEFAULT '{}'` | |
| `created_at`, `updated_at` | `timestamptz NOT NULL DEFAULT now()` | `updated_at` changes only when `version` or `highlights` does |

The calendar is upserted in one transaction. A Patch no longer on the page is kept.

## User-Agent

Both clients send `INGEST_USER_AGENT`, default `mlbb-collector/1.0 (+github.com/yeremi777/mlbb-collector; kuroganehunter99@gmail.com)`. Liquipedia's terms require contact details in it.

## Schedule

`deploy/cron/crontab` holds one marked block: `CRON_TZ=Asia/Jakarta` and `ingest all` at 07:00 and 13:00, run from the checkout with `.env` loaded into its environment, appending to `log/ingest.log`. `make ingest-install` builds `bin/ingest` and writes the block into the user's crontab with this checkout's path, replacing an earlier copy of the block; `make ingest-status` prints the installed block.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `DB_*` | | as in `api` |
| `INGEST_TIMEZONE` | `Asia/Jakarta` | the zone whose today is a run's date |
| `INGEST_USER_AGENT` | as above | sent to both upstreams |
| `MOONTON_BASE_URL` | `https://api.gms.moontontech.com/api/gms/source/2669606` | for pointing a run at a stub |
| `LIQUIPEDIA_BASE_URL` | `https://liquipedia.net/mobilelegends/api.php` | for pointing a run at a stub |

A non-default base URL is logged as a warning on every run.

## Acceptance criteria

- AC-1: Migrations create both tables with the columns, checks, and index above; `make migrate-reset && make migrate-up` succeeds.
- AC-2: `make ingest` exits 0 and leaves 30 combinations × the upstream roster rows for today, and the sum of `appearance_share` for `all`/1-day is within 0.01 of 1.
- AC-3: A second `make ingest` the same day makes no Moonton request and inserts nothing; with `-force` it appends a second full set.
- AC-4: With 10 of 30 combinations deleted for today, `ingest stats` fetches exactly those 10.
- AC-5: Unit tests against a stub server prove: `records: null`, `code` non-zero, and `total` 100 fail without retry; a short read and a 500 are retried once; a failure in the middle keeps earlier combinations and exits 1 after the rest; requests are 2s apart.
- AC-6: `payload` equals the upstream record byte-for-byte as JSON, including `sub_hero`.
- AC-7: `ingest patches` twice in a row reports zero changed on the second run and leaves every `updated_at` unchanged. A fixture with one edited Highlights entry changes exactly that row.
- AC-8: Parser tests on the committed `Portal:Patches` fixture yield 113 Patches, newest first, with the reused-version and letter-suffix cases, and reject a page with tables but no Patch rows as non-retryable.
- AC-9: `LIQUIPEDIA_BASE_URL=http://127.0.0.1:1 ingest all` logs the calendar failure, still stores statistics, and exits 1.
- AC-10: Both upstreams receive the configured User-Agent; Liquipedia receives one `action=parse` request per run.
- AC-11: `make ingest-install` twice leaves exactly one block in `crontab -l`, and `make ingest-status` prints it.

## Verification

```bash
set -a; . ./.env; set +a; export PGHOST=$DB_HOST PGPORT=$DB_PORT PGDATABASE=$DB_NAME PGUSER=$DB_USERNAME PGPASSWORD=$DB_PASSWORD PGSSLMODE=$DB_SSLMODE   # psql below reads these
go vet ./...
go test ./...                                                      # AC-5, AC-8, AC-10
make test-integration                                              # AC-5 to AC-7 against test_mlbb_collector
make migrate-reset && make migrate-up                              # AC-1, run by the user
make ingest && make ingest                                         # AC-2, AC-3, run by the user
psql -c "SELECT rank_tier, window_days, count(*) FROM raw.hero_rank_snapshots WHERE snapshot_date = current_date GROUP BY 1, 2 ORDER BY 1, 2"   # AC-2
psql -tAc "SELECT sum(appearance_share) FROM raw.hero_rank_snapshots WHERE rank_tier = 'all' AND window_days = 1 AND snapshot_date = current_date"   # AC-2
psql -c "DELETE FROM raw.hero_rank_snapshots WHERE snapshot_date = current_date AND (window_days = 15 OR (window_days = 30 AND rank_tier IN ('epic','legend','mythic','honor')))"   # AC-4 setup, run by the user against a disposable database
go run ./cmd/ingest stats                                          # AC-4, logs 10 fetched, 20 skipped
LIQUIPEDIA_BASE_URL=http://127.0.0.1:1 go run ./cmd/ingest all; test $? -eq 1   # AC-9
make ingest-install && make ingest-install && make ingest-status   # AC-11
```
