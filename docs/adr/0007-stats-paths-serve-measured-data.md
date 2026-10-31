# Stats paths serve measured data

Paths under `/api/stats` serve data measured by Moonton, and paths under `/api/heroes` serve the authored dataset, so the hero standing board is `/api/stats/heroes` and a hero's Measured counters belong under `/api/stats/heroes/{heroId}`, never beside its Counters at `/api/heroes/{heroId}/counters`. `/api/heroes/stats` was rejected because its literal segment sits where `{heroId}` goes and leaves a Hero ID `stats` unreachable, and `/api/hero-stats` because flat names give later per-hero and Measured counter routes no shared root.

## Consequences

The `stats` Swagger tag covers every measured route, not only heroes. A Measured counter and a Counter never share a path, as they never share a Score.
