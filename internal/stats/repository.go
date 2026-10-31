package stats

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Repository reads Boards from marts.hero_current and public.heroes.
type Repository struct{ db database.Querier }

// NewRepository reads through db.
func NewRepository(db database.Querier) Repository { return Repository{db: db} }

// Board returns the Board of q's Rank tier and Window, empty when it has no
// Snapshot. A hero absent from public.heroes is left out.
func (r Repository) Board(ctx context.Context, q Query) (Board, error) {
	board := Board{RankTier: q.RankTier, WindowDays: q.WindowDays, Heroes: []Standing{}}
	var (
		date, version *string
		daysSince     *int
		crosses       *bool
	)
	err := r.db.QueryRow(ctx, `
		SELECT snapshot_date::text, patch_as_of, days_since_patch, window_crosses_patch
		  FROM marts.hero_current
		 WHERE rank_tier = $1 AND window_days = $2::bigint
		 ORDER BY snapshot_date DESC
		 LIMIT 1`, q.RankTier, q.WindowDays).Scan(&date, &version, &daysSince, &crosses)
	if errors.Is(err, pgx.ErrNoRows) {
		return board, nil
	}
	if err != nil {
		return Board{}, err
	}
	board.SnapshotDate = date
	if version != nil {
		board.Patch = &Patch{Version: *version, DaysSince: *daysSince, WindowCrossesPatch: *crosses}
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+hero.Columns("h")+`,
		       c.win_rate::float8, c.appearance_share::float8, c.ban_rate::float8,
		       c.prev_snapshot_date::text, c.win_rate_delta::float8, c.appearance_share_delta::float8, c.ban_rate_delta::float8,
		       c.win_rate_rank::int, c.appearance_rank::int
		  FROM marts.hero_current c JOIN heroes h ON h.mlid = c.main_hero_id
		 WHERE c.rank_tier = $1 AND c.window_days = $2::bigint AND c.snapshot_date = $3::date
		 ORDER BY c.win_rate_rank, h.uid`, q.RankTier, q.WindowDays, *date)
	if err != nil {
		return Board{}, err
	}
	board.Heroes, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Standing, error) {
		var (
			s                               Standing
			since                           *string
			deltaWin, deltaAppear, deltaBan *float64
		)
		h := &s.Hero
		err := row.Scan(&h.UID, &h.MLID, &h.Name, &h.Roles, &h.Lanes, &h.Images,
			&s.WinRate, &s.AppearanceShare, &s.BanRate, &since, &deltaWin, &deltaAppear, &deltaBan,
			&s.Position.WinRate, &s.Position.Appearance)
		if since != nil {
			s.Change = &Change{Since: *since, WinRate: *deltaWin, AppearanceShare: *deltaAppear, BanRate: *deltaBan}
		}
		return s, err
	})
	if err != nil {
		return Board{}, err
	}
	return board, nil
}
