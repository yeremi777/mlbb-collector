//go:build integration

package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// snap is one raw.hero_rank_snapshots row; zero appear and ban take 0.1, empty fetched takes the transaction's now(), and empty payload takes {}.
type snap struct {
	hero    int
	tier    string
	window  int
	date    string
	win     float64
	appear  float64
	ban     float64
	fetched string
	payload string
}

func insertSnap(t *testing.T, ctx context.Context, tx pgx.Tx, s snap) {
	t.Helper()
	if s.tier == "" {
		s.tier = "all"
	}
	if s.window == 0 {
		s.window = 1
	}
	if s.appear == 0 {
		s.appear = 0.1
	}
	if s.ban == 0 {
		s.ban = 0.1
	}
	if s.payload == "" {
		s.payload = "{}"
	}
	var fetched any
	if s.fetched != "" {
		fetched = s.fetched
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO raw.hero_rank_snapshots
			(main_heroid, rank_tier, window_days, snapshot_date, win_rate, appearance_share, ban_rate, payload, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, COALESCE($9::timestamptz, now()))`,
		s.hero, s.tier, s.window, s.date, s.win, s.appear, s.ban, s.payload, fetched)
	if err != nil {
		t.Fatal(err)
	}
}

func queryAll[T any](t *testing.T, ctx context.Context, tx pgx.Tx, sql string) []T {
	t.Helper()
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[T])
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSnapshotLatestKeepsTheNewestFetchOfEachKey(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-20", win: 0.50, fetched: "2026-10-20 10:00+00"})
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-20", win: 0.40, fetched: "2026-10-20 09:00+00"})
	insertSnap(t, ctx, tx, snap{hero: 2, date: "2026-10-20", win: 0.45, fetched: "2026-10-20 10:00+00"})
	insertSnap(t, ctx, tx, snap{hero: 2, date: "2026-10-20", win: 0.46, fetched: "2026-10-20 10:00+00"})
	insertSnap(t, ctx, tx, snap{hero: 3, date: "2026-10-20", win: 0.30})

	type row struct {
		Hero int
		Win  float64
	}
	for _, view := range []string{"hero_rank_snapshot_latest", "hero_rank_daily"} {
		got := queryAll[row](t, ctx, tx, fmt.Sprintf(`SELECT main_hero_id, win_rate::float8 FROM staging.%s ORDER BY 1`, view))
		want := []row{{1, 0.50}, {2, 0.46}, {3, 0.30}}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s = %v, want %v", view, got, want)
		}
	}

	columns := queryAll[struct{ Name string }](t, ctx, tx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'staging' AND table_name = 'hero_rank_daily' ORDER BY ordinal_position`)
	for _, c := range columns {
		if c.Name == "id" || c.Name == "payload" {
			t.Errorf("hero_rank_daily carries %s", c.Name)
		}
	}
}

func TestCounterDailyOrientsEachMeasuredCounterToItsCounterHero(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-20", payload: `{"data": {
		"sub_hero":      [{"heroid": 2, "increase_win_rate": 0.05}, {"heroid": 3, "increase_win_rate": -0.01},
		                  {"heroid": 4, "increase_win_rate": 0.03}, {"heroid": 7, "increase_win_rate": 0}],
		"sub_hero_last": [{"heroid": 5, "increase_win_rate": -0.04}, {"heroid": 6, "increase_win_rate": 0.02}]}}`})

	type row struct {
		Target, Counter int
		Delta           float64
		Source          string
		Rank            int
	}
	got := queryAll[row](t, ctx, tx, `
		SELECT target_heroid, counter_heroid, win_rate_delta::float8, source, source_rank::int
		FROM staging.hero_counter_daily ORDER BY source, source_rank`)
	want := []row{{1, 2, 0.05, "sub_hero", 1}, {1, 4, 0.03, "sub_hero", 3}, {5, 1, 0.04, "sub_hero_last", 1}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestCounterDailySkipsWhatThePayloadDoesNotSay(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	for hero, payload := range map[int]string{
		10: `{"data": {"sub_hero": null, "sub_hero_last": 7}}`,
		11: `{"data": {"sub_hero": "x"}}`,
		12: `{}`,
		13: `{"data": 5}`,
		14: `{"data": {"sub_hero": [
			{"heroid": "x", "increase_win_rate": 0.1}, {"heroid": 20, "increase_win_rate": null},
			{"increase_win_rate": 0.2}, {"heroid": 21, "increase_win_rate": "0.3"},
			{"heroid": 3.5, "increase_win_rate": 0.1}, {"heroid": 99999999999, "increase_win_rate": 0.1},
			{"heroid": 23, "increase_win_rate": 1e300}, 7, null,
			{"heroid": 22, "increase_win_rate": 0.07}]}}`,
	} {
		insertSnap(t, ctx, tx, snap{hero: hero, date: "2026-10-20", payload: payload})
	}

	type row struct {
		Target, Counter int
		Rank            int
	}
	got := queryAll[row](t, ctx, tx, `SELECT target_heroid, counter_heroid, source_rank::int FROM staging.hero_counter_daily ORDER BY 1, 2`)
	want := []row{{14, 22, 10}, {14, 23, 7}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestPatchCalendarListsEveryPatch(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO raw.patches (release_date, version, highlights)
		VALUES ('2026-10-07', '2.1.99', '{"Hero Adjustments"}'), ('2026-10-10', '2.1.100', '{}')`); err != nil {
		t.Fatal(err)
	}
	type row struct {
		Date       string
		Version    string
		Highlights []string
	}
	got := queryAll[row](t, ctx, tx, `SELECT release_date::text, version, highlights FROM staging.patch_calendar ORDER BY 1`)
	want := []row{{"2026-10-07", "2.1.99", []string{"Hero Adjustments"}}, {"2026-10-10", "2.1.100", []string{}}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func insertHero(t *testing.T, ctx context.Context, tx pgx.Tx, uid string, mlid int, name string) {
	t.Helper()
	if _, err := tx.Exec(ctx, `INSERT INTO public.heroes (uid, mlid, name, roles, lanes) VALUES ($1, $2, $3, '{tank}', '{roam}')`, uid, mlid, name); err != nil {
		t.Fatal(err)
	}
}

func TestHeroCurrentHoldsOneRowPerHeroTierAndWindowAtTheNewestDate(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertHero(t, ctx, tx, "tigreal", 1, "Tigreal")
	for _, tier := range []string{"all", "epic", "legend", "mythic", "honor", "glory"} {
		for _, window := range []int{1, 3, 7, 15, 30} {
			for _, date := range []string{"2026-10-19", "2026-10-20"} {
				for _, hero := range []int{1, 133} {
					insertSnap(t, ctx, tx, snap{hero: hero, tier: tier, window: window, date: date, win: 0.5})
				}
			}
		}
	}

	type row struct {
		Hero            int
		UID, Name, Date string
		Count           int
	}
	got := queryAll[row](t, ctx, tx, `
		SELECT main_hero_id, COALESCE(hero_uid, 'NULL'), COALESCE(hero_name, 'NULL'), snapshot_date::text, count(*)::int
		FROM marts.hero_current GROUP BY 1, 2, 3, 4 ORDER BY 1`)
	want := []row{{1, "tigreal", "Tigreal", "2026-10-20", 30}, {133, "NULL", "NULL", "2026-10-20", 30}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestHeroCurrentMeasuresMovementAgainstThePreviousAvailableDate(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-15", win: 0.50, appear: 0.10, ban: 0.20})
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-19", win: 0.53, appear: 0.12, ban: 0.25})
	insertSnap(t, ctx, tx, snap{hero: 2, date: "2026-10-19", win: 0.48})

	type row struct {
		Hero                   int
		Win, Appear, Ban, Prev string
	}
	got := queryAll[row](t, ctx, tx, `
		SELECT main_hero_id, COALESCE(win_rate_delta::text, 'NULL'), COALESCE(appearance_share_delta::text, 'NULL'),
		       COALESCE(ban_rate_delta::text, 'NULL'), COALESCE(prev_snapshot_date::text, 'NULL')
		FROM marts.hero_current ORDER BY 1`)
	want := []row{{1, "0.030000", "0.020000", "0.050000", "2026-10-15"}, {2, "NULL", "NULL", "NULL", "NULL"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestHeroCurrentNamesThePatchAMeasurementBelongsTo(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if _, err := tx.Exec(ctx, `INSERT INTO raw.patches (release_date, version) VALUES ('2026-10-10', '2.1.100'), ('2026-10-07', '2.1.99')`); err != nil {
		t.Fatal(err)
	}
	for _, window := range []int{1, 3, 7, 15, 30} {
		insertSnap(t, ctx, tx, snap{hero: 1, window: window, date: "2026-10-09", win: 0.5})
	}
	insertSnap(t, ctx, tx, snap{hero: 2, date: "2026-09-01", win: 0.5})

	type row struct {
		Hero, Window         int
		Patch, Days, Crosses string
	}
	got := queryAll[row](t, ctx, tx, `
		SELECT main_hero_id, window_days, COALESCE(patch_as_of, 'NULL'), COALESCE(days_since_patch::text, 'NULL'),
		       COALESCE(window_crosses_patch::text, 'NULL')
		FROM marts.hero_current ORDER BY 1, 2`)
	want := []row{
		{1, 1, "2.1.99", "2", "false"}, {1, 3, "2.1.99", "2", "true"}, {1, 7, "2.1.99", "2", "true"},
		{1, 15, "2.1.99", "2", "true"}, {1, 30, "2.1.99", "2", "true"},
		{2, 1, "NULL", "NULL", "NULL"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestHeroCurrentRanksOnlyRowsOfTheSameDate(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-20", win: 0.60, appear: 0.10})
	insertSnap(t, ctx, tx, snap{hero: 2, date: "2026-10-19", win: 0.70, appear: 0.30})
	insertSnap(t, ctx, tx, snap{hero: 3, date: "2026-10-20", win: 0.55, appear: 0.20})

	type row struct{ Hero, WinRank, AppearanceRank int }
	got := queryAll[row](t, ctx, tx, `SELECT main_hero_id, win_rate_rank::int, appearance_rank::int FROM marts.hero_current ORDER BY 1`)
	want := []row{{1, 1, 2}, {2, 1, 1}, {3, 2, 1}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ranks = %v, want %v", got, want)
	}
}

func TestCounterCurrentHoldsTheNewestDatePerTarget(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	insertHero(t, ctx, tx, "tigreal", 1, "Tigreal")
	insertHero(t, ctx, tx, "diggie", 3, "Diggie")
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-19", payload: `{"data": {"sub_hero": [{"heroid": 2, "increase_win_rate": 0.05}]}}`})
	insertSnap(t, ctx, tx, snap{hero: 1, date: "2026-10-20", payload: `{"data": {"sub_hero": [{"heroid": 3, "increase_win_rate": 0.04}]}}`})
	insertSnap(t, ctx, tx, snap{hero: 9, date: "2026-10-20", payload: `{"data": {"sub_hero_last": [{"heroid": 1, "increase_win_rate": -0.02}]}}`})
	insertSnap(t, ctx, tx, snap{hero: 9, date: "2026-10-18", payload: `{"data": {"sub_hero_last": [{"heroid": 1, "increase_win_rate": -0.09}]}}`})

	type row struct{ Target, Counter, Delta, Date string }
	got := queryAll[row](t, ctx, tx, `
		SELECT target_name, COALESCE(counter_name, 'NULL'), win_rate_delta::text, snapshot_date::text
		FROM marts.hero_counter_current ORDER BY counter_heroid`)
	want := []row{{"Tigreal", "Diggie", "0.04", "2026-10-20"}, {"Tigreal", "NULL", "0.02", "2026-10-20"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}
