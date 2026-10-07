# MLBB Collector

Go backend for an MLBB counter-pick analyzer.

The service seeds a hand-authored hero, counter, and synergy dataset from `data/` into Postgres and serves it over a REST API for the frontend hero selector, counter reveal, and AI analysis flow. Scores are produced by an AI provider at request time from the authored reasons and proof, and are never stored.

## Features

- Hero list API with search, role, lane, and pagination filters
- Hero detail API
- Hero counter and synergy matchup APIs, with each matchup's proof
- AI counter scoring and detail (`POST /api/counters/analyze-score`, `POST /api/counters/analyze-detail`)
- AI synergy scoring and detail (`POST /api/synergies/analyze-score`, `POST /api/synergies/analyze-detail`)
- AI providers in a fall-through chain (OpenRouter, OpenCode Zen, and an offline `mock`), with an in-memory result cache
- Optional Redis-backed rate limiting of uncached analyze requests, per client address and per browser
- Seeding (`make seed`) that makes the public tables match `data/` exactly
- Timestamped SQL migrations via goose
- Swagger UI at `/docs`

## Tech Stack

- Go 1.26
- PostgreSQL
- pgx/v5 with hand-written SQL, no ORM
- goose for migrations
- Redis through go-redis, only when rate limiting is enabled

## Getting Started

Create `.env` from the example and point `DB_*` at your local Postgres. `AI_PROVIDERS=mock` answers the analyze routes offline, with no API key.

```bash
cp .env.example .env
```

Install goose, create the database, apply the migrations, and seed the dataset:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
createdb -h 127.0.0.1 -U postgres mlbb_collector
make migrate-up
make seed
```

Run the API:

```bash
make api
```

The API listens on `APP_PORT` and is reached at `APP_URL`, `http://127.0.0.1:8080` in the example. Swagger UI is at `APP_URL/docs`. A variable given to `make` overrides `.env`:

```bash
make api APP_PORT=8090 APP_URL=http://127.0.0.1:8090
```

To limit analyze requests, set `RATE_LIMIT_ENABLED=true`, `RATE_LIMIT_SALT`, and `REDIS_HOST` (with `REDIS_PORT` and `REDIS_DB`) in `.env`. `make help` lists every target.

## Project Layout

```
cmd/api              REST API binary: the routes the frontend consumes
cmd/seed             seeder binary: data/ -> Postgres
internal/hero        heroes: repository, service, handlers
internal/counter     counters and their proof: repository, handlers
internal/synergy     synergies and their proof: repository, handlers
internal/analysis    AI scoring and detail: prompts, validation, cache, analyze handlers
internal/ai          OpenAI-compatible chat providers and the fall-through chain
internal/ratelimit   Redis-backed analyze rate limiting
internal/dataset     reads and validates the authored dataset
internal/seed        writes the dataset to the public tables
internal/database    Postgres connections and goose migrations
internal/httpx       JSON responses, errors, routing, CORS, docs page
internal/config      environment configuration
docs                 OpenAPI contract, specs, ADRs, authoring guides
data                 hand-authored heroes, counters, and synergies (source of truth in git)
```

## Testing

```bash
make test
```

Integration tests run against `test_mlbb_collector` on the `.env` Postgres server, rebuilt from the migrations and emptied after every run, and against database 15 (`TEST_REDIS_DB`) on the `.env` Redis server. Create the test database once, then:

```bash
createdb -h 127.0.0.1 -U postgres test_mlbb_collector
make test-integration
```

## Documentation

- `CONTEXT.md`: the domain glossary
- `docs/adr/`: architecture decisions
- `docs/specs/`: one spec per feature, with acceptance criteria
- `docs/authoring/`: how counter and synergy proof is written
- `docs/openapi.yaml`: the API contract the Swagger UI serves

## Notice

This is a fan-made project. Not affiliated with or endorsed by Moonton.
