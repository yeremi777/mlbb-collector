# Hero snapshots are append-only

Moonton publishes hero statistics for the current day only and cannot be asked for a past date, so `raw.hero_rank_snapshots` records every fetch as new rows and is never updated or deleted from. Each row keeps the hero's upstream record as `jsonb`, unmodified, beside the parsed Win rate, Appearance share, and Ban rate, and the request names no fields so the server returns its full default projection. The table has no unique key and no foreign key to `public.heroes`. A run fetches only the Rank tier and Window combinations that have no rows for its date, so a re-run finishes a partial day and a complete day costs nothing; `-force` fetches every combination again and appends. Staging resolves repeated fetches to the newest.

## Considered options

- **Unique key per hero, Rank tier, Window, and date, with upsert.** Rejected: a same-day restatement by Moonton would silently replace the earlier answer.
- **Parsed columns only, or a narrowed field list.** Rejected: a field that is not stored can never be recovered. `sub_hero` was first read as synergy data; keeping the payload meant the fix was a new view over stored rows, not a lost history.
- **Foreign key to `public.heroes`.** Rejected: Moonton lists new heroes before the authored dataset carries them.
- **Skip the run when its date has any rows.** Rejected: a run that failed halfway would leave the day permanently incomplete.

## Consequences

The table grows by about 4,000 rows a day. Nothing outside the `staging` views reads it directly.
