# Authored dataset is the source of truth

Heroes, Counters, and Synergies are curated as JSON files in `data/` and reviewed as diffs in git, and the seeder makes the `public` tables match those files exactly: changed rows are updated, missing rows are inserted, rows absent from the files are deleted, and the whole seed runs in one transaction. The database is never edited by hand, because the next seed overwrites any such edit; a fix to the data goes into the files.

## Considered options

- **Database as the editable source.** Rejected: an edit would leave no reviewable diff, and Proof is only trustworthy when someone can see who changed it and why.
- **Seeder that only inserts and updates.** Rejected: a Counter removed from the files would keep being served.

## Consequences

Removing a Hero from `heroes.json` deletes every Counter and Synergy that names it. The seeder refuses an empty source, so a broken or truncated file fails the seed instead of emptying the tables.
