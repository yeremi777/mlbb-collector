# Patch calendar is keyed on release date

`raw.patches` holds one row per Patch, keyed on its release date, and the ingest upserts the whole calendar from Liquipedia on every run, changing a row only when its version or Highlights differ from what is stored. An unchanged page therefore writes nothing, and a row's `updated_at` records when Liquipedia last changed it. The release date is the key because it is unique across every Patch on the page, while versions are not: patches before 2025 reused a version for successive updates, 38 of 75 versions repeat, and 1.9.42 shipped five times.

## Considered options

- **Append every fetch, as hero snapshots do.** Rejected: the full page is re-read daily, so the table grew by 113 rows a day to record that nothing changed.
- **Key on release date and version, keeping a corrected version as a second row.** Rejected: it needs a deduplicating view to protect an event that has never happened in the page's history, and it kept history for the version while overwriting the Highlights anyway.

## Consequences

A Highlights or version edit on Liquipedia replaces the stored text. A vandalised edit stays until the page is repaired and the next run picks up the repair; the previous text is not kept.
