# analysis

## Goal

Four analyze routes return an AI-produced Score and Confidence for every Counter or Synergy of one hero, or a written explanation of one, built only from the stored Reasons and Proof ([ADR-0003](../adr/0003-scores-are-ai-generated-and-never-stored.md)), with the same request and response contract as `mlbb-analyzer-service` branch `dev`.

## Non-goals

- No rate limiting. `rate-limit` owns it, and the check order below leaves its place.
- No measured data, Patch, or Snapshot in any prompt. Scores draw on the authored dataset only.
- No Score, Confidence, or explanation is written to Postgres or any durable store.
- No change to a request key, response key, or `*_not_found` error code the frontend reads.
- No prompt wording change. The messages sent to a provider equal `dev`'s for the same input.

## Routes

| Method and path | Request | 200 body |
|---|---|---|
| `POST /api/counters/analyze-score` | `{"targetHeroId","language"}` | `{"targetHeroId","source":"ai","recommendations":[{"rank","counterHeroId","score","confidence"}]}` |
| `POST /api/counters/analyze-detail` | `{"targetHeroId","counterHeroId","language"}` | `{"targetHeroId","counterHeroId","source":"ai","score","confidence","summary","strengths","conditions","failureCases","evidenceIds"}` |
| `POST /api/synergies/analyze-score` | `{"anchorHeroId","language"}` | `{"anchorHeroId","source":"ai","recommendations":[{"rank","synergyHeroId","score","confidence"}]}` |
| `POST /api/synergies/analyze-detail` | `{"anchorHeroId","synergyHeroId","language"}` | `{"anchorHeroId","synergyHeroId","source":"ai","score","confidence","summary","strengths","conditions","failureCases","evidenceIds"}` |

`language` is `en` or `id`, and empty means `en`. With `id`, the explanatory prose is Indonesian while ids, hero names, and enum values stay as given.

### Check order

Each route checks, in this order, and answers with the first failure:

1. The body is valid JSON: else 422 `invalid_request`.
2. An AI provider is configured: else 504 `ai_provider_not_configured`.
3. `language` is allowed: else 422 `invalid_request`.
4. The Target or Anchor hero exists: else 404 `target_hero_not_found` or `anchor_hero_not_found`.
5. That hero has Counters or Synergies: else 404 `counter_data_not_found` or `synergy_data_not_found`.
6. Detail only: the Counter or Synergy hero exists, else 404 `counter_hero_not_found` or `synergy_hero_not_found`; and the pair is a stored Counter or Synergy, else 404 `counter_matchup_not_found` or `synergy_matchup_not_found`.
7. Rate limiting, owned by `rate-limit`. A request whose result is cached passes it without consuming quota.
8. The cache answers, or the provider is asked.

### Errors

| Status | Code | When |
|---|---|---|
| 422 | `invalid_request` | message `Request body is not valid JSON.` or `language must be 'en' or 'id'.` |
| 404 | `*_not_found` | as in Check order; the hero messages are `Hero was not found in the dataset.`, `Counter hero was not found in the dataset.`, `Synergy hero was not found in the dataset.`; the data and matchup messages read `Counter data was not found for the target hero.`, `Counter matchup was not found for the target hero.`, and their Synergy and anchor equivalents |
| 502 | `ai_provider_error` | every provider failed, or the model's answer failed validation; the message is `dev`'s for the same failure |
| 504 | `ai_provider_not_configured` | message `AI provider is not configured. Set the required API key in .env.` |
| 504 | `ai_provider_timeout` | the request's AI deadline passed; message `AI provider did not answer in time.` |

## Scoring

- One provider request scores every Counter or Synergy of the hero.
- The answer is valid only when `recommendations` holds each expected Counter hero or Synergy hero ID exactly once, each with an integer `score` and `confidence` from 0 to 100, and nothing else.
- Recommendations are ranked by Score descending, then Confidence descending, then hero ID ascending, and numbered from 1.

## Detail

- The answer is valid only when `score` and `confidence` are integers from 0 to 100, `summary` is non-blank, `strengths` has at least one entry, and every `evidenceIds` entry is a Proof ID of that Counter or Synergy. A missing `conditions`, `failureCases`, or `evidenceIds` becomes an empty array.
- An answer with an unknown evidence ID fails at once with 502. Any other invalid answer is sent back once with a repair instruction, and a second invalid answer fails with 502.

## Providers

- `AI_PROVIDERS` lists provider names in fall-through order: `openrouter`, `opencode_zen`, `mock`. A name outside that list stops startup with an error naming it.
- `openrouter` and `opencode_zen` speak OpenAI-compatible `POST /chat/completions` with `response_format: json_object` and temperature 0.2. A listed provider without its API key or model is skipped with a warning; an API key starting with `<` is a placeholder and counts as missing. No usable provider leaves the routes answering `ai_provider_not_configured`.
- One provider is asked directly; two or more are asked as a chain, whose failure message joins each provider's, as `dev` does.
- A transport error, a 5xx, or a 402, 408, 409, 425, or 429 from a provider falls through to the next one. Any other failure stops the chain.
- A model answer wrapped in a markdown code fence is unwrapped before parsing.
- `mock` makes no network call and returns a valid, deterministic answer derived from the request's Counters or Synergies, for local development and tests.

## Deadline and cache

- `AI_TIMEOUT_SECONDS` bounds the whole provider work of one request: every provider tried and the detail repair. When it passes, the request answers `ai_provider_timeout`.
- Valid results are cached in memory per kind, hero, partner (detail only), and language, for `AI_ANALYSIS_CACHE_TTL_SECONDS`, holding at most `AI_ANALYSIS_CACHE_MAX_ENTRIES` entries and evicting the least recently used. A zero value disables the cache. A failure is never cached.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `AI_PROVIDERS` | empty | provider names, comma-separated, in fall-through order; empty leaves the routes answering `ai_provider_not_configured`, and `.env.example` sets `mock` |
| `AI_TIMEOUT_SECONDS` | `60` | deadline for one request's provider work |
| `AI_ANALYSIS_CACHE_TTL_SECONDS` | `600` | cache lifetime |
| `AI_ANALYSIS_CACHE_MAX_ENTRIES` | `256` | cache size |
| `OPENROUTER_API_KEY`, `OPENROUTER_MODEL`, `OPENROUTER_SERVER_URL`, `OPENROUTER_APP_TITLE`, `OPENROUTER_HTTP_REFERER` | model `openrouter/free`, URL `https://openrouter.ai/api/v1`, title `MLBB Analyzer Service`, referer `http://127.0.0.1:8000` | OpenRouter; title and referer are sent as `X-OpenRouter-Title` and `HTTP-Referer` |
| `OPENCODE_ZEN_API_KEY`, `OPENCODE_ZEN_MODEL`, `OPENCODE_ZEN_SERVER_URL` | URL `https://opencode.ai/zen/v1` | OpenCode Zen; a leading `opencode/` on the model is dropped |

## Acceptance criteria

- AC-1: For each of the four routes, the messages sent to the provider for `tigreal` (and `tigreal`/`diggie` for counter detail, `tigreal`/`pharsa` for synergy detail) in `en` and `id`, and the repair messages after an invalid counter detail, equal golden files produced from `dev`'s prompt builders.
- AC-2: Against a stub chat-completions server returning a fixed answer, each route's status and body bytes, and the request bodies the stub receives, equal golden files captured from `dev`'s analyzer and response encoding against the same stub and input, with no database. The cases cover a valid answer for each route, a detail repair, an invalid scoring answer, a provider error, and a two-provider chain that fails.
- AC-3: Handler tests prove each step of Check order answers its stated status and code, and that a request failing step N never reaches step N+1.
- AC-4: Unit tests prove each Scoring and Detail validation rule rejects an answer that breaks it, the ranking order including both tie-breaks, and that the repair is sent once and only for a non-evidence failure.
- AC-5: Unit tests prove fall-through on each retryable status and on a transport error, a stop on a non-retryable status, and `ai_provider_timeout` when a stub provider outlasts the deadline, including during the repair.
- AC-6: Unit tests prove a cache hit makes no provider call, expiry and least-recently-used eviction, that a failure is not cached, and that a zero setting disables the cache.
- AC-7: With `AI_PROVIDERS=mock` and no API key, all four routes answer 200 for `tigreal`, and twice the same request gives the same body.
- AC-8: `AI_PROVIDERS=openai` stops startup with an error naming `openai`.
- AC-9: `docs/openapi.yaml` describes the four routes, their bodies, and every error above, and the route test from `api` passes with the analyze routes no longer excepted.

## Verification

```bash
go vet ./...
go test ./...                                                       # AC-1 to AC-6, AC-9
AI_PROVIDERS=mock make api                                          # then:
curl -s -XPOST 127.0.0.1:8080/api/counters/analyze-score -d '{"targetHeroId":"tigreal"}'           # AC-7
curl -s -XPOST 127.0.0.1:8080/api/synergies/analyze-detail -d '{"anchorHeroId":"tigreal","synergyHeroId":"'"$(curl -s 127.0.0.1:8080/api/heroes/tigreal/synergies | jq -r '.[0].synergyHero.uid')"'","language":"id"}'   # AC-7
AI_PROVIDERS=openai go run ./cmd/api; test $? -ne 0                 # AC-8
```
