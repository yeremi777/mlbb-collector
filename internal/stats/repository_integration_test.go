//go:build integration

package stats_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/seed"
	"github.com/yeremi777/mlbb-collector/internal/stats"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// snap is one raw.hero_rank_snapshots row with an empty payload.
type snap struct {
	hero             int
	tier             string
	window           int
	date             string
	win, appear, ban float64
}

func insert(t *testing.T, ctx context.Context, tx pgx.Tx, snaps ...snap) {
	t.Helper()
	for _, s := range snaps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO raw.hero_rank_snapshots
				(main_heroid, rank_tier, window_days, snapshot_date, win_rate, appearance_share, ban_rate, payload)
			VALUES ($1, $2, $3, $4, $5, $6, $7, '{}')`,
			s.hero, s.tier, s.window, s.date, s.win, s.appear, s.ban); err != nil {
			t.Fatal(err)
		}
	}
}

// seedHeroes stores heroes as tanks in the roam lane and returns them as stored.
func seedHeroes(t *testing.T, ctx context.Context, tx pgx.Tx, heroes ...hero.Hero) []hero.Hero {
	t.Helper()
	for i := range heroes {
		heroes[i].Roles, heroes[i].Lanes, heroes[i].Images = []string{"tank"}, []string{"roam"}, json.RawMessage(`{"head": "https://example.test/`+heroes[i].UID+`.png"}`)
	}
	if err := seed.Sync(ctx, tx, dataset.Dataset{Heroes: heroes}); err != nil {
		t.Fatal(err)
	}
	return heroes
}

func ptr[T any](v T) *T { return &v }

func TestBoardHoldsTheNewestDateOfItsRankTierAndWindow(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	heroes := seedHeroes(t, ctx, tx,
		hero.Hero{UID: "miya", MLID: 1, Name: "Miya"},
		hero.Hero{UID: "layla", MLID: 2, Name: "Layla"},
		hero.Hero{UID: "tigreal", MLID: 6, Name: "Tigreal"},
		hero.Hero{UID: "alucard", MLID: 7, Name: "Alucard"})
	if _, err := tx.Exec(ctx, `INSERT INTO raw.patches (release_date, version) VALUES ('2026-10-10', '2.1.100')`); err != nil {
		t.Fatal(err)
	}
	insert(t, ctx, tx,
		snap{1, "glory", 7, "2026-10-20", 0.50, 0.010, 0.020},
		snap{1, "glory", 7, "2026-10-30", 0.52, 0.012, 0.010},
		snap{2, "glory", 7, "2026-10-20", 0.60, 0.010, 0.010},  // stale: no 10-30 Snapshot
		snap{6, "glory", 7, "2026-10-30", 0.55, 0.020, 0.030},  // first Snapshot
		snap{7, "glory", 7, "2026-10-30", 0.52, 0.030, 0.001},  // ties miya on win rate
		snap{99, "glory", 7, "2026-10-30", 0.53, 0.001, 0.001}, // not in public.heroes
		snap{1, "glory", 30, "2026-10-31", 0.40, 0.010, 0.010}, // another Window
		snap{1, "epic", 7, "2026-10-01", 0.45, 0.010, 0.010},   // before every Patch
		snap{1, "mythic", 7, "2026-10-12", 0.45, 0.010, 0.010}, // window starts before the Patch
	)
	repo := stats.NewRepository(tx)

	got, err := repo.Board(ctx, stats.Query{RankTier: "glory", WindowDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	want := stats.Board{
		RankTier: "glory", WindowDays: 7, SnapshotDate: ptr("2026-10-30"),
		Patch: &stats.Patch{Version: "2.1.100", DaysSince: 20, WindowCrossesPatch: false},
		Heroes: []stats.Standing{
			{Hero: heroes[2], WinRate: 0.55, AppearanceShare: 0.02, BanRate: 0.03,
				Position: stats.Position{WinRate: 1, Appearance: 2}},
			{Hero: heroes[3], WinRate: 0.52, AppearanceShare: 0.03, BanRate: 0.001,
				Position: stats.Position{WinRate: 3, Appearance: 1}},
			{Hero: heroes[0], WinRate: 0.52, AppearanceShare: 0.012, BanRate: 0.01,
				Change:   &stats.Change{Since: "2026-10-20", WinRate: 0.02, AppearanceShare: 0.002, BanRate: -0.01},
				Position: stats.Position{WinRate: 3, Appearance: 3}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("glory 7:\n got %s\nwant %s", jsonOf(t, got), jsonOf(t, want))
	}

	epic, err := repo.Board(ctx, stats.Query{RankTier: "epic", WindowDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	if epic.SnapshotDate == nil || *epic.SnapshotDate != "2026-10-01" || epic.Patch != nil || len(epic.Heroes) != 1 {
		t.Errorf("epic 7 = %s, want 2026-10-01 with a null patch and one hero", jsonOf(t, epic))
	}

	mythic, err := repo.Board(ctx, stats.Query{RankTier: "mythic", WindowDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	if want := (stats.Patch{Version: "2.1.100", DaysSince: 2, WindowCrossesPatch: true}); mythic.Patch == nil || *mythic.Patch != want {
		t.Errorf("mythic 7 patch = %s, want %+v", jsonOf(t, mythic.Patch), want)
	}
}

func TestBoardIsEmptyWithoutASnapshot(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	repo := stats.NewRepository(tx)
	for _, q := range []stats.Query{{RankTier: "all", WindowDays: 7}, {RankTier: "diamond", WindowDays: 10},
		{RankTier: "all", WindowDays: 40000}, {RankTier: "all", WindowDays: -40000}} {
		got, err := repo.Board(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		want := stats.Board{RankTier: q.RankTier, WindowDays: q.WindowDays, Heroes: []stats.Standing{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%+v:\n got %s\nwant %s", q, jsonOf(t, got), jsonOf(t, want))
		}
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
