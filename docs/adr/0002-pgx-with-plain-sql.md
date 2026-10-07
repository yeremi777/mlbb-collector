# pgx with plain SQL

The Go service standard this project follows uses GORM. Here repositories use `pgx/v5` with SQL written by hand, because the work leans on Postgres features an ORM hides: Snapshots store the upstream record as `jsonb` exactly as received, seeding and ingest write through `pgx.Batch`, every upsert carries an `IS DISTINCT FROM` guard so unchanged rows are not rewritten, and the `staging` and `marts` layers are views. GORM was rejected because each of those needs a raw-SQL escape hatch anyway, and because every column would be declared twice, in the goose migration and on a model struct, with nothing keeping the two in step. sqlc was rejected because a code generator costs more than it saves at about twenty queries. Goose remains the migration tool.

## Consequences

SQL errors surface at run time, not at compile time, so every repository query needs a test against a real Postgres.
