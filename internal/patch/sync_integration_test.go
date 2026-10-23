//go:build integration

package patch_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/patch"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

var past = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// backdate sets every updated_at to past, so a later write shows.
func backdate(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	if _, err := tx.Exec(ctx, `UPDATE raw.patches SET updated_at = $1`, past); err != nil {
		t.Fatal(err)
	}
}

// touched returns the release dates whose updated_at is no longer past.
func touched(t *testing.T, ctx context.Context, tx pgx.Tx) []time.Time {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT release_date FROM raw.patches WHERE updated_at <> $1 ORDER BY 1`, past)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSyncStoresTheCalendarAndWritesOnlyChanges(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	calendar := parseFixture(t)

	changed, err := patch.Sync(ctx, tx, calendar)
	if err != nil || changed != 113 {
		t.Fatalf("first sync: changed %d, err %v; want 113", changed, err)
	}
	var version string
	var highlights []string
	if err := tx.QueryRow(ctx, `SELECT version, highlights FROM raw.patches WHERE release_date = '2026-08-04'`).Scan(&version, &highlights); err != nil {
		t.Fatal(err)
	}
	if version != "2.1.95" || len(highlights) != 5 || highlights[1] != "Hero Adjustments" {
		t.Errorf("2026-08-04 = %s %q", version, highlights)
	}

	backdate(t, ctx, tx)
	changed, err = patch.Sync(ctx, tx, calendar)
	if err != nil || changed != 0 {
		t.Fatalf("unchanged sync: changed %d, err %v; want 0", changed, err)
	}
	if got := touched(t, ctx, tx); len(got) != 0 {
		t.Errorf("unchanged sync touched %v", got)
	}

	edited := append([]patch.Patch(nil), calendar...)
	edited[3].Highlights = append(append([]string(nil), edited[3].Highlights...), "Bug Fixes")
	edited[5].Version += "x"
	changed, err = patch.Sync(ctx, tx, edited)
	if err != nil || changed != 2 {
		t.Fatalf("edited sync: changed %d, err %v; want 2", changed, err)
	}
	got := touched(t, ctx, tx)
	if len(got) != 2 || !got[0].Equal(edited[5].ReleaseDate) || !got[1].Equal(edited[3].ReleaseDate) {
		t.Errorf("edited sync touched %v, want %s and %s", got, edited[5].ReleaseDate, edited[3].ReleaseDate)
	}
}

func TestSyncKeepsAPatchNoLongerOnThePage(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	calendar := parseFixture(t)
	if _, err := patch.Sync(ctx, tx, calendar); err != nil {
		t.Fatal(err)
	}
	if _, err := patch.Sync(ctx, tx, calendar[:10]); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM raw.patches`).Scan(&n); err != nil || n != 113 {
		t.Errorf("%d rows (err %v), want 113", n, err)
	}
}
