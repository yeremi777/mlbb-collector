package snapshot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database"
)

const requestInterval = 2 * time.Second

// Summary counts a run's combinations and the rows it inserted.
type Summary struct {
	Fetched  int
	Skipped  int
	Failed   int
	Inserted int
}

// Ingest fetches every combination with no rows for date, or every
// combination when force is set, and inserts each in its own transaction.
// Requests are 2s apart. A failed combination is logged and the run moves on;
// an error is returned only when stored rows cannot be read or ctx is done.
func Ingest(ctx context.Context, db database.DB, c *Client, date time.Time, force bool) (Summary, error) {
	stored := map[Combination]bool{}
	if !force {
		var err error
		if stored, err = storedCombinations(ctx, db, date); err != nil {
			return Summary{}, err
		}
	}
	var s Summary
	requested := false
	for _, combo := range Combinations() {
		if stored[combo] {
			s.Skipped++
			continue
		}
		if requested {
			c.Sleep(requestInterval)
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}
		requested = true
		n, err := fetchAndInsert(ctx, db, c, date, combo)
		if err != nil {
			s.Failed++
			slog.Error("snapshot failed", "window_days", combo.Window, "rank_tier", combo.RankTier, "err", err)
			continue
		}
		s.Fetched++
		s.Inserted += n
	}
	return s, nil
}

func fetchAndInsert(ctx context.Context, db database.DB, c *Client, date time.Time, combo Combination) (int, error) {
	records, err := c.Fetch(ctx, combo)
	if err != nil {
		return 0, err
	}
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, r := range records {
			batch.Queue(`INSERT INTO raw.hero_rank_snapshots
				(main_heroid, rank_tier, window_days, snapshot_date, win_rate, appearance_share, ban_rate, payload)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				r.MoontonID, combo.RankTier, combo.Window, date, r.WinRate, r.AppearanceShare, r.BanRate, r.Payload)
		}
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return 0, fmt.Errorf("insert: %w", err)
	}
	return len(records), nil
}

func storedCombinations(ctx context.Context, db database.Querier, date time.Time) (map[Combination]bool, error) {
	rows, err := db.Query(ctx, `SELECT DISTINCT window_days, rank_tier FROM raw.hero_rank_snapshots WHERE snapshot_date = $1`, date)
	if err != nil {
		return nil, fmt.Errorf("read stored combinations: %w", err)
	}
	stored, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Combination, error) {
		var c Combination
		return c, row.Scan(&c.Window, &c.RankTier)
	})
	if err != nil {
		return nil, fmt.Errorf("read stored combinations: %w", err)
	}
	out := make(map[Combination]bool, len(stored))
	for _, c := range stored {
		out[c] = true
	}
	return out, nil
}
