# api

## Goal

`cmd/api` serves Heroes, Counters, and Synergies from the `public` tables over HTTP, answering every route below with the same status, headers, and body bytes as `mlbb-analyzer-service` branch `dev`, so the `mlbb-analyzer` frontend switches to it without a change.

## Non-goals

- No analyze routes, AI provider, or rate limiting. `analysis` and `rate-limit` own them.
- No write route, authentication, or route the frontend does not call.
- No change to a response shape, key name, status code, or error code the frontend reads.
- No dataset loading at request time. The API reads only Postgres.

## Routes

| Method and path | 200 body | Errors |
|---|---|---|
| `GET /health` | `{"status":"ok"}` | |
| `GET /api/heroes` | Hero page | |
| `GET /api/heroes/{heroId}` | Hero | 404 `hero_not_found` |
| `GET /api/heroes/{heroId}/counters` | array of Counter | 404 `hero_not_found`, 404 `counter_data_not_found` |
| `GET /api/heroes/{heroId}/synergies` | array of Synergy | 404 `hero_not_found`, 404 `synergy_data_not_found` |
| `GET /docs` | Scalar page rendering `docs/openapi.yaml` | |
| `GET /docs/openapi.yaml` | the spec file, `application/yaml` | |

`docs/openapi.yaml` is the contract for every route and body except the two `/docs` routes that serve it. It is written by hand, and a test fails when a registered route other than those two is missing from it, or a path in it is not registered.

### Bodies

- **Hero:** `{"uid","mlid","name","roles","lanes","images"}`. `mlid` is a JSON string of decimal digits. `images` is the object stored for the Hero.
- **Hero page:** `{"items","page","size","total","pages"}`, where `items` is an array of Hero.
- **Counter:** `{"targetHeroId","counterHero","reasons","counterTypes","proof"}`, where `counterHero` is the Counter hero as a Hero.
- **Synergy:** `{"anchorHeroId","synergyHero","reasons","synergyTypes","proof"}`, where `synergyHero` is the Synergy hero as a Hero.
- **Proof:** `{"id","category","priority","impact","summary","worksBestWhen","failureCases"}`.
- **Error:** `{"error":{"code","message"}}`.

Every JSON body is written by `encoding/json`'s `Encoder` with its defaults, so it ends in a newline and escapes `<`, `>`, and `&`.

### Hero list

- `search` keeps Heroes whose name contains it, case-insensitively, after trimming spaces.
- `role` and `lane` keep Heroes with that exact Role or Lane, case-insensitively, after trimming spaces.
- Filters combine with AND. An empty parameter is no filter.
- `page` is 1-based, default 1; a value below 1 or not an integer means 1.
- `size` defaults to 10; a value below 1 or not an integer means 10, and a value above 100 means 100.
- Heroes are ordered by Moonton ID. `total` counts the filtered Heroes, and `pages` is `ceil(total / size)`, or 0 when `total` is 0.
- A page past the last returns an empty `items` with the true `total` and `pages`.

### Counters and Synergies

- Counters are ordered by Counter hero ID, and Synergies by Synergy hero ID. Proofs within each are ordered by Proof ID.
- A Hero ID with no Hero answers `hero_not_found`. A Hero with no Counters answers `counter_data_not_found`, and one with no Synergies answers `synergy_data_not_found`.

### Errors

| Status | Code | Message |
|---|---|---|
| 404 | `hero_not_found` | `Hero was not found in the dataset.` |
| 404 | `counter_data_not_found` | `Counter data was not found for the target hero.` |
| 404 | `synergy_data_not_found` | `Synergy data was not found for the anchor hero.` |
| 404 | `not_found` | `Route was not found.` |
| 405 | `method_not_allowed` | `Method is not allowed on this route.` |
| 500 | `internal_error` | `Internal server error.` |

A 500 logs the underlying error. Its body never contains it.

## CORS

A request whose `Origin` is `http://localhost:3000` or one of the comma-separated `FRONTEND_ORIGIN` values gets `Access-Control-Allow-Origin` set to that origin, `Access-Control-Allow-Credentials: true`, and `Vary: Origin`. An `OPTIONS` preflight from such an origin answers 204 with `Access-Control-Allow-Methods: GET, POST, OPTIONS` and echoes `Access-Control-Request-Headers`. Any other origin gets no CORS headers.

## Configuration

Read once at startup into one typed config, from the environment and an optional `.env`. A missing required value or an unparseable one stops startup with an error naming the variable.

| Variable | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | required | Postgres connection URL, used verbatim |
| `API_HOST` | `127.0.0.1` | listen address |
| `API_PORT` | `8080` | listen port |
| `FRONTEND_ORIGIN` | empty | extra CORS origins, comma-separated |

## Server

- Startup pings Postgres and fails when it cannot reach it.
- Read-header timeout 5s, read timeout 10s, idle timeout 60s. The write timeout is `AI_TIMEOUT_SECONDS` (default 20, owned by `analysis`) plus 10s, so an analyze response is never cut off.
- `SIGINT` or `SIGTERM` stops accepting connections and lets in-flight requests finish for up to 25s.

## Acceptance criteria

- AC-1: For every route in the table and for these requests, the status, `Content-Type`, and body bytes equal `dev`'s against the same seeded database: `/api/heroes`, `?search=ali`, `?role=TANK`, `?lane=roam&page=2&size=5`, `?page=0&size=500`, `?page=99`, `?size=abc`, `/api/heroes/tigreal`, `/api/heroes/x.borg`, `/api/heroes/nope`, the counters and synergies of `tigreal`, `x.borg`, `hirara`, and `nope`.
- AC-2: A request from an allowed origin, a preflight from an allowed origin, and a request from another origin get the CORS headers stated above, and match `dev`'s.
- AC-3: An unknown path answers 404 `not_found` and a wrong method on a known path answers 405 `method_not_allowed`, both in the error body shape.
- AC-4: A handler test forces a repository error and gets 500 `internal_error` with no detail from the error in the body.
- AC-5: The route test passes: every registered route except `/docs` and `/docs/openapi.yaml` is in `docs/openapi.yaml`, and every path in it other than the four analyze routes `analysis` owns is registered.
- AC-6: `GET /docs` renders the spec in Scalar.
- AC-7: Startup without `DATABASE_URL`, or with `API_PORT=abc`, exits non-zero naming the variable. Startup with an unreachable database exits non-zero.
- AC-8: After `SIGTERM` during a request that is still running, the request completes and the process exits 0.

## Verification

```bash
go vet ./...
go test ./...                                              # AC-3, AC-4, AC-5
make api                                                   # this API on :8080
API_PORT=8081 go run ./cmd/api                             # dev's API, from the mlbb-analyzer-service checkout on branch dev, same DATABASE_URL
for p in /health /api/heroes '/api/heroes?search=ali' '/api/heroes?role=TANK' '/api/heroes?lane=roam&page=2&size=5' '/api/heroes?page=0&size=500' '/api/heroes?page=99' '/api/heroes?size=abc' /api/heroes/tigreal /api/heroes/x.borg /api/heroes/nope /api/heroes/tigreal/counters /api/heroes/x.borg/counters /api/heroes/hirara/counters /api/heroes/nope/counters /api/heroes/tigreal/synergies /api/heroes/x.borg/synergies /api/heroes/hirara/synergies /api/heroes/nope/synergies; do diff <(curl -si "localhost:8080$p" | grep -iv '^date:') <(curl -si "localhost:8081$p" | grep -iv '^date:') >/dev/null || echo "DIFF $p"; done   # AC-1, prints nothing
for o in http://localhost:3000 https://evil.example; do diff <(curl -si -H "Origin: $o" localhost:8080/health | grep -i '^access-control\|^vary') <(curl -si -H "Origin: $o" localhost:8081/health | grep -i '^access-control\|^vary') || echo "DIFF $o"; done   # AC-2
curl -si -X OPTIONS -H 'Origin: http://localhost:3000' -H 'Access-Control-Request-Headers: content-type' localhost:8080/api/heroes   # AC-2, 204
curl -s localhost:8080/nope; curl -s -X DELETE localhost:8080/api/heroes   # AC-3
open http://localhost:8080/docs                            # AC-6
env -u DATABASE_URL go run ./cmd/api; API_PORT=abc go run ./cmd/api   # AC-7
```
