package patch

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yeremi777/mlbb-collector/internal/database"
)

// Sync upserts the calendar into raw.patches in one transaction and returns
// how many rows it inserted or changed. A row is rewritten only when its
// version or Highlights differ; a Patch no longer in the calendar is kept.
func Sync(ctx context.Context, db database.DB, patches []Patch) (int, error) {
	changed := 0
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, p := range patches {
			batch.Queue(`INSERT INTO raw.patches (release_date, version, highlights) VALUES ($1, $2, $3)
				ON CONFLICT (release_date) DO UPDATE
				SET version = EXCLUDED.version, highlights = EXCLUDED.highlights, updated_at = now()
				WHERE (raw.patches.version, raw.patches.highlights) IS DISTINCT FROM (EXCLUDED.version, EXCLUDED.highlights)`,
				p.ReleaseDate, p.Version, p.Highlights).Exec(func(tag pgconn.CommandTag) error {
				changed += int(tag.RowsAffected())
				return nil
			})
		}
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return 0, fmt.Errorf("sync patches: %w", err)
	}
	return changed, nil
}
