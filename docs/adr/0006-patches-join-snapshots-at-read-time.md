# Patches join Snapshots at read time

A Snapshot's Patch is resolved in the `marts` views by date, as the latest Patch released on or before the Snapshot's date, and is never written into `raw.hero_rank_snapshots`. Stamping the Patch at ingest was rejected because the calendar comes from a different source on a different failure path: a late or failed calendar fetch would stamp the wrong Patch into append-only rows for good, while a read-time join is corrected by the next successful fetch.

## Consequences

`ingest all` fetches the calendar before statistics, but a calendar failure does not stop the statistics run. A Snapshot dated before the earliest known Patch has no Patch.
