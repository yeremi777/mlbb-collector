# stats

## Goal

`GET /api/stats/heroes` serves the meta board: every Hero's standing from `marts.hero_current` for one Rank tier and Window, at the newest Snapshot date of that pair, with its Patch, its change since the previous Snapshot, and its position.

## Non-goals

- No `date` parameter and no history. `marts.hero_current` holds only each hero's newest Snapshot date.
- No migration, and no change to `raw`, `staging`, `marts`, or `public`.
- No per-hero route, role or lane filter, pagination, or Measured counter.
- No rate limit, authentication, or cache. The rate limit stays on the analyze routes only.
- No change to any existing route, body, or error code.

## Route

| Method and path | 200 body | Errors |
|---|---|---|
| `GET /api/stats/heroes` | Board | 500 `internal_error` |

Paths under `/api/stats` serve Moonton's measured data, and paths under `/api/heroes` serve the authored dataset, so a Measured counter and a Counter never share a path.

### Query

- `rank` is the Rank tier, default `all`. It is trimmed and lowercased.
- `window` is the Window in days, default `7`. A value that is blank or not an integer means `7`.
- A Rank tier or Window that names no data, such as `rank=diamond` or `window=10`, answers 200 with an empty Board. No query value answers 400.

### Board

```json
{
  "rankTier": "glory",
  "windowDays": 7,
  "snapshotDate": "2026-10-30",
  "patch": { "version": "1.9.20", "daysSince": 12, "windowCrossesPatch": false },
  "heroes": [
    {
      "hero": { "uid": "tigreal", "mlid": "6", "name": "Tigreal", "roles": ["tank"], "lanes": ["roam"], "images": { "head": "https://..." } },
      "winRate": 0.512, "appearanceShare": 0.008, "banRate": 0.031,
      "change": { "since": "2026-10-20", "winRate": 0.004, "appearanceShare": -0.001, "banRate": 0.002 },
      "position": { "winRate": 1, "appearance": 88 }
    }
  ]
}
```

- `rankTier` and `windowDays` echo the query as read.
- `snapshotDate` is the newest `snapshot_date` in `marts.hero_current` for the Rank tier and Window. Only rows of that date are on the Board.
- `patch` is `patch_as_of`, `days_since_patch`, and `window_crosses_patch` of that date, or `null` when the date predates every known Patch.
- `heroes` holds each row with a Hero ID, ordered by `position.winRate`, then Hero ID. A row with a NULL Hero ID is left out, and the positions keep the view's ranks.
- `hero` is the Hero in full, as `counterHero` is on a Counter. Rates and deltas are fractions, as stored.
- `position` is the hero's place by win rate and by Appearance share among the Board's Snapshot date.
- `change` is `null` when the hero has no earlier Snapshot for the Rank tier and Window; `since` is `prev_snapshot_date`.
- An empty Board is `{"rankTier", "windowDays", "snapshotDate": null, "patch": null, "heroes": []}`.

## Acceptance criteria

- AC-1: With fixture Snapshots on two dates, `GET /api/stats/heroes?rank=glory&window=7` returns only the newer date's heroes, with that date's Patch, ordered by win rate position then Hero ID, each `change` measured from the older date.
- AC-2: A hero whose newest Snapshot is older than the Board's date is not on the Board.
- AC-3: A hero absent from `public.heroes` is not on the Board.
- AC-4: A hero with one Snapshot has `change` `null`. A Snapshot dated before every Patch gives `patch` `null`.
- AC-5: No `rank` or `window` reads `all` and `7`. A blank or non-integer `window` reads `7`. `rank=GLORY ` reads `glory`.
- AC-6: An empty database, `rank=diamond`, and `window=10` each answer 200 with the empty Board, echoing the Rank tier and Window.
- AC-7: A handler test forces a repository error and gets 500 `internal_error` with no detail from the error in the body.
- AC-8: The route test passes: `GET /api/stats/heroes` is registered and documented in `docs/openapi.yaml`, under the `stats` tag, in the wording of the existing operations.

## Verification

```bash
set -a; . <(grep -E '^(DB|APP)_' .env); set +a; export PGHOST=$DB_HOST PGPORT=$DB_PORT PGDATABASE=$DB_NAME PGUSER=$DB_USERNAME PGPASSWORD=$DB_PASSWORD PGSSLMODE=$DB_SSLMODE   # psql below reads these
make vet
go test ./...                                              # AC-5, AC-6, AC-7, AC-8
make test-integration                                      # AC-1 to AC-4, AC-6 on test_mlbb_collector
make api                                                   # demo: the API on APP_PORT
curl -s "$APP_URL/api/stats/heroes?rank=glory&window=7" | jq '{snapshotDate, patch, n: (.heroes | length), top: .heroes[0]}'   # demo
psql -tAc "SELECT count(*) FROM marts.hero_current WHERE rank_tier = 'glory' AND window_days = 7 AND hero_uid IS NOT NULL AND snapshot_date = (SELECT max(snapshot_date) FROM marts.hero_current WHERE rank_tier = 'glory' AND window_days = 7)"   # equals n above
curl -s "$APP_URL/api/stats/heroes?rank=diamond"                  # AC-6, empty Board
```
