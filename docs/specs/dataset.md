# dataset

## Goal

The authored Heroes, Counters, and Synergies in `data/` are validated as one unit and seeded into the `public` tables below, which then match the files exactly ([ADR-0001](../adr/0001-authored-dataset-is-the-source-of-truth.md)).

## Non-goals

- No HTTP API, analysis, ingest, or `raw`, `staging`, or `marts` schema. Later specs own them.
- The authored content is not edited. The files are copied from `mlbb-analyzer-service` branch `dev`, `data/static/`, unchanged.
- No index files (`counters.json`, `synergies.json`), no `data/static/raw/hero-meta-final.json`, and no README inside `data/`.
- No `score` or `scoreHint` anywhere in the dataset ([ADR-0003](../adr/0003-scores-are-ai-generated-and-never-stored.md)).

## Files

| Path | Holds |
|---|---|
| `data/heroes.json` | An array of every Hero, in Moonton ID order |
| `data/counters/<hero-id>.json` | An array of the Counters whose Target hero is `<hero-id>` |
| `data/synergies/<hero-id>.json` | An array of the Synergies whose Anchor hero is `<hero-id>` |

A Hero with no Counters or no Synergies has no file in that directory.

### Hero

| Key | JSON type | Rule |
|---|---|---|
| `uid` | string | the Hero ID; non-blank, unique |
| `mlid` | string | the Moonton ID as decimal digits; a positive integer, unique |
| `name` | string | non-blank |
| `roles` | string array | one or more of `tank`, `fighter`, `assassin`, `mage`, `marksman`, `support`, no repeats |
| `lanes` | string array | zero or more of `gold`, `exp`, `mid`, `roam`, `jungle`, no repeats |
| `images` | object | optional; image URLs by variant, currently `head`; stored as given |

### Counter and Synergy

| Counter key | Synergy key | JSON type | Rule |
|---|---|---|---|
| `targetHeroId` | `anchorHeroId` | string | equals the file name without `.json` |
| `counterHeroId` | `synergyHeroId` | string | a Hero ID other than the first hero |
| `reasons` | `reasons` | string array | one or more, each non-blank |
| `counterTypes` | `synergyTypes` | string array | one or more, each non-blank; free-form tags |
| `proof` | `proof` | Proof array | one or more |

### Proof

| Key | JSON type | Rule |
|---|---|---|
| `id` | string | non-blank; unique across all Counter Proofs, and separately across all Synergy Proofs |
| `category` | string | one of the categories for its kind, below |
| `priority` | string | `primary`, `secondary`, or `condition` |
| `impact` | string | `high`, `medium`, or `low` |
| `summary` | string | non-blank |
| `worksBestWhen` | string array | one or more, each non-blank |
| `failureCases` | string array | one or more, each non-blank |

Counter Proof categories: `skill-interaction`, `crowd-control-counter`, `damage-type-advantage`, `item-power-spike`, `mobility-advantage`, `range-advantage`, `kiting`, `sustain-anti-sustain`, `positioning-requirement`, `cooldown-window`, `vision-awareness`, `teamfight-role-counter`, `game-phase`, `execution-difficulty`.

Synergy Proof categories: `skill-interaction`, `crowd-control-chain`, `engage-follow-up`, `setup-combo`, `damage-amplification`, `protection`, `peel`, `frontline-enabler`, `mobility-enabler`, `vision-setup`, `healing-sustain`, `shielding`, `poke-siege`, `pickoff-combo`, `teamfight-combo`, `objective-control`, `laning-synergy`, `game-phase`, `positioning-requirement`, `cooldown-window`, `execution-difficulty`.

### Loading fails

The dataset is loaded whole and rejected whole, with an error naming the file, the row index, and the key, when any of these holds:

- a file is not valid JSON, or is not an array;
- `heroes.json` is empty, or a Counter or Synergy file is empty;
- a key in the tables above is missing, or has the wrong JSON type;
- an object carries a key not in the tables above, including `score` and `scoreHint` (`images` is the one open object);
- a value breaks its rule above;
- a Counter or Synergy names a Hero ID that `heroes.json` does not carry;
- two Counters share a Target hero and Counter hero, or two Synergies share an Anchor hero and Synergy hero.

## Data model

Every table has `created_at` and `updated_at` (`timestamptz NOT NULL DEFAULT now()`). Every text array is `text[] NOT NULL`. Columns keep the dataset's names for the Hero ID (`uid`) and Moonton ID (`mlid`).

### public.heroes

| Column | Type | Rule |
|---|---|---|
| `uid` | `text` PK | |
| `mlid` | `integer NOT NULL UNIQUE` | |
| `name` | `text NOT NULL` | |
| `roles` | `text[]` | `cardinality >= 1`, contained in the six roles |
| `lanes` | `text[]` | contained in the five lanes |
| `images` | `jsonb NOT NULL DEFAULT '{}'` | |

### public.counters, public.synergies

| Column | Type | Rule |
|---|---|---|
| `target_hero_id` / `anchor_hero_id` | `text NOT NULL` | FK `heroes(uid) ON DELETE CASCADE` |
| `counter_hero_id` / `synergy_hero_id` | `text NOT NULL` | FK `heroes(uid) ON DELETE CASCADE`; differs from the first hero |
| `reasons` | `text[]` | `cardinality >= 1` |
| `counter_types` / `synergy_types` | `text[]` | `cardinality >= 1` |

Primary key is the hero pair.

### public.counter_proofs, public.synergy_proofs

| Column | Type | Rule |
|---|---|---|
| `id` | `text` PK | |
| the hero pair | `text NOT NULL` × 2 | FK to its Counter or Synergy `ON DELETE CASCADE`, indexed |
| `category` | `text NOT NULL` | `CHECK IN` the categories for its kind |
| `priority` | `text NOT NULL` | `CHECK IN ('primary','secondary','condition')` |
| `impact` | `text NOT NULL` | `CHECK IN ('high','medium','low')` |
| `summary` | `text NOT NULL` | `<> ''` |
| `works_best_when`, `failure_cases` | `text[]` | `cardinality >= 1` |

## Seeding

- `make seed` loads the dataset first and connects to the database named by the `DB_*` settings (as in `api`) only after the whole dataset is accepted.
- One transaction covers all five tables. Within it, rows absent from the files are deleted first, then every row is upserted.
- An upsert changes a row, and its `updated_at`, only when a column value differs from the stored one.
- The seed prints the row count it synced per table.

## Acceptance criteria

- AC-1: Every file under `data/` is byte-identical to its counterpart under `dev`'s `data/static/`, and `data/` holds nothing else.
- AC-2: Loading the dataset yields 133 Heroes, 660 Counters with 660 Proofs, and 660 Synergies with 660 Proofs.
- AC-3: Unit tests prove each case under Loading fails rejects a fixture dataset with an error naming the file, row index, and key.
- AC-4: Migrations create the five tables with the columns, checks, keys, and indexes above. `make migrate-reset`, which rolls every migration back, then `make migrate-up`, succeeds.
- AC-5: Seeding an empty database leaves heroes 133, counters 660, counter_proofs 660, synergies 660, synergy_proofs 660.
- AC-6: Seeding a second time leaves every count and `max(updated_at)` of every table unchanged.
- AC-7: Seeding from a copy of `data/` with one Counter removed deletes that Counter and its Proof; seeding from a copy with one Hero removed deletes that Hero and every Counter and Synergy naming it.
- AC-8: Seeding from an invalid dataset exits non-zero without connecting to the database. A seed that fails inside the transaction leaves every table as it was.
- AC-9: Integration tests run against a throwaway Postgres (`make test-db-up`) whose schema they rebuild from the migrations, never the `.env` database, and refuse a database whose name does not start with `test`. Each test runs inside a transaction that is rolled back.

## Verification

```bash
set -a; . ./.env; set +a; export PGHOST=$DB_HOST PGPORT=$DB_PORT PGDATABASE=$DB_NAME PGUSER=$DB_USERNAME PGPASSWORD=$DB_PASSWORD PGSSLMODE=$DB_SSLMODE   # psql below reads these
git -C ~/Documents/Local/mlbb-analyzer-service archive dev data/static | tar -x -C "$TMPDIR" && diff -r -x counters.json -x synergies.json -x raw -x README.md "$TMPDIR/data/static" data   # AC-1
go vet ./...
go test ./...                                                     # AC-2, AC-3
make test-db-up && make test-integration                            # AC-9
make migrate-reset && make migrate-up                             # AC-4, run by the user
make seed && make seed                                            # AC-5, AC-6, run by the user
psql -c "SELECT 'heroes', count(*), max(updated_at) FROM heroes UNION ALL SELECT 'counters', count(*), max(updated_at) FROM counters UNION ALL SELECT 'counter_proofs', count(*), max(updated_at) FROM counter_proofs UNION ALL SELECT 'synergies', count(*), max(updated_at) FROM synergies UNION ALL SELECT 'synergy_proofs', count(*), max(updated_at) FROM synergy_proofs"   # AC-5, AC-6
go run ./cmd/seed -data "$TMPDIR/data-minus-one"                  # AC-7, run by the user against a disposable database
go run ./cmd/seed -data "$TMPDIR/data-invalid"; test $? -ne 0     # AC-8
```
