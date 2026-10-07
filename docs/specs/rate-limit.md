# rate-limit

## Goal

When enabled, the four analyze routes count each uncached request against a per-address and a per-browser quota in Redis and refuse requests over it, with the same responses, cookie, and Redis keys as `mlbb-analyzer-service` branch `dev`.

## Non-goals

- No limit on the read routes, and none on an analyze request answered from the cache.
- No change to the analyze responses of a request that passes.
- No limiting when `RATE_LIMIT_ENABLED` is not `true`, and no Redis connection is opened then.

## Behaviour

- `X-Forwarded-For` is trusted as sent: the client address is its first entry, or the connection's peer address when the header is absent. A caller who rotates the header gets a fresh address quota; this is accepted.
- The address is stored only as an HMAC-SHA256 of it keyed with `RATE_LIMIT_SALT`.
- The browser is identified by a UUID in the `RATE_LIMIT_COOKIE_NAME` cookie. A request without a valid one is issued a new one: `HttpOnly`, `Path=/`, with the configured `Max-Age`, `Secure`, and `SameSite`.
- Each route has its own two counters, keyed `rate:analyze:<route>:ip:<hash>` and `rate:analyze:<route>:client:<uuid>`, where `<route>` is `analyze-counter-score`, `analyze-counter-detail`, `analyze-synergy-score`, or `analyze-synergy-detail`.
- The address counter is checked first, then the browser counter. Each check and increment is one atomic Redis script; a counter's window starts at its first request and lasts `RATE_LIMIT_ANALYZE_WINDOW_SECONDS`.
- A score route allows `RATE_LIMIT_ANALYZE_MAX_REQUESTS` per window, and a detail route that many times `RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER`.

| Status | Code | Message | Headers |
|---|---|---|---|
| 429 | `rate_limit_exceeded` | `Too many analyze requests. Please try again later.` | `Retry-After` set to the counter's remaining seconds |
| 503 | `rate_limit_unavailable` | `Rate limit storage is unavailable. Please try again later.` | |

A Redis failure refuses the request with 503; it never lets the request through unlimited.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `RATE_LIMIT_ENABLED` | `false` | `true` turns limiting on |
| `REDIS_URL` | required when enabled | Redis connection URL |
| `RATE_LIMIT_SALT` | required when enabled | HMAC key for addresses |
| `RATE_LIMIT_ANALYZE_MAX_REQUESTS` | `5` | score requests per window |
| `RATE_LIMIT_ANALYZE_WINDOW_SECONDS` | `18000` | window length |
| `RATE_LIMIT_ANALYZE_DETAIL_MULTIPLIER` | `3` | detail allowance as a multiple of the score allowance |
| `RATE_LIMIT_COOKIE_NAME` | `mlbb_analyzer_client_id` | browser cookie name |
| `RATE_LIMIT_COOKIE_MAX_AGE_SECONDS` | `2592000` | cookie lifetime |
| `RATE_LIMIT_COOKIE_SECURE` | `false` | `true` for HTTPS deployments |
| `RATE_LIMIT_COOKIE_SAMESITE` | `lax` | `lax`, `strict`, or `none` |

Enabling limiting without `REDIS_URL` or `RATE_LIMIT_SALT`, or with an unparseable value, stops startup with an error naming the variable.

## Acceptance criteria

- AC-1: Against a real Redis, the sixth uncached score request from one address within the window answers 429 with `Retry-After`, and the sixteenth detail request does the same.
- AC-2: A request from a new address with an exhausted cookie answers 429, and a request with no cookie from an exhausted address answers 429.
- AC-3: A request without a valid cookie receives one with the configured attributes; a request with a valid one receives none.
- AC-4: A cached analyze request consumes no quota: the counters are unchanged after it.
- AC-5: With Redis stopped, an uncached analyze request answers 503 `rate_limit_unavailable`.
- AC-6: With `RATE_LIMIT_ENABLED=false`, no Redis connection is opened and no cookie is set.
- AC-7: Enabling without `RATE_LIMIT_SALT` stops startup naming it.
- AC-8: For the same sequence of requests, the Redis keys, counter values, and TTLs equal those `dev` writes.

## Verification

```bash
go vet ./...
go test ./...                                                     # AC-1 to AC-8 against a local Redis
RATE_LIMIT_ENABLED=true REDIS_URL=redis://127.0.0.1:6379/15 RATE_LIMIT_SALT=x AI_PROVIDERS=mock AI_ANALYSIS_CACHE_TTL_SECONDS=0 make api
for i in 1 2 3 4 5 6; do curl -s -o /dev/null -w '%{http_code}\n' -XPOST -H 'X-Forwarded-For: 10.0.0.1' localhost:8080/api/counters/analyze-score -d '{"targetHeroId":"tigreal"}'; done   # AC-1: five 200s, then 429
redis-cli -n 15 --scan --pattern 'rate:analyze:*'                 # AC-8 key shape
RATE_LIMIT_ENABLED=true REDIS_URL=redis://127.0.0.1:6379/15 go run ./cmd/api; test $? -ne 0   # AC-7
```
