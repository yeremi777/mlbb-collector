//go:build integration

package snapshot_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/snapshot"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

var day = time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)

func rowsPerCombination(t *testing.T, ctx context.Context, tx pgx.Tx) map[snapshot.Combination]int {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT window_days, rank_tier, count(*) FROM raw.hero_rank_snapshots
		WHERE snapshot_date = $1 GROUP BY 1, 2`, day)
	if err != nil {
		t.Fatal(err)
	}
	got := map[snapshot.Combination]int{}
	for rows.Next() {
		var c snapshot.Combination
		var n int
		if err := rows.Scan(&c.Window, &c.RankTier, &n); err != nil {
			t.Fatal(err)
		}
		got[c] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIngestStoresEveryCombination(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, waits := newClient(t, s)

	got, err := snapshot.Ingest(ctx, tx, c, day, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Fetched: 30, Inserted: 3030}); got != want {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
	stored := rowsPerCombination(t, ctx, tx)
	for _, combo := range snapshot.Combinations() {
		if stored[combo] != 101 {
			t.Errorf("%+v: %d rows, want 101", combo, stored[combo])
		}
	}
	if len(*waits) != 29 {
		t.Fatalf("waits %v, want 29", *waits)
	}
	for _, w := range *waits {
		if w != 2*time.Second {
			t.Fatalf("waits %v, want each 2s", *waits)
		}
	}

	var payloads, rates int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE payload = $1::jsonb),
		count(*) FILTER (WHERE win_rate = 0.512345 AND ban_rate = 0.0123 AND appearance_share = round(1.0/101, 6))
		FROM raw.hero_rank_snapshots WHERE main_heroid = 1`, record(1)).Scan(&payloads, &rates); err != nil {
		t.Fatal(err)
	}
	if payloads != 30 || rates != 30 {
		t.Errorf("hero 1: %d rows with the upstream payload and %d with its rates, want 30 each", payloads, rates)
	}
}

func TestIngestFetchesOnlyMissingCombinations(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, _ := newClient(t, s)
	if _, err := snapshot.Ingest(ctx, tx, c, day, false); err != nil {
		t.Fatal(err)
	}

	again, err := snapshot.Ingest(ctx, tx, c, day, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Skipped: 30}); again != want || len(s.requests) != 30 {
		t.Errorf("complete day: summary %+v and %d requests, want %+v and 30", again, len(s.requests), want)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM raw.hero_rank_snapshots WHERE snapshot_date = $1
		AND (window_days = 15 OR (window_days = 30 AND rank_tier IN ('epic','legend','mythic','honor')))`, day); err != nil {
		t.Fatal(err)
	}
	partial, err := snapshot.Ingest(ctx, tx, c, day, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Fetched: 10, Skipped: 20, Inserted: 1010}); partial != want || len(s.requests) != 40 {
		t.Errorf("partial day: summary %+v and %d requests, want %+v and 40", partial, len(s.requests), want)
	}

	forced, err := snapshot.Ingest(ctx, tx, c, day, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Fetched: 30, Inserted: 3030}); forced != want {
		t.Errorf("forced: summary %+v, want %+v", forced, want)
	}
	for combo, n := range rowsPerCombination(t, ctx, tx) {
		if n != 202 {
			t.Errorf("%+v: %d rows after -force, want 202", combo, n)
		}
	}
}

func TestIngestKeepsEarlierCombinationsWhenOneFails(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	good := ok(body(0, 101, records(101)))
	s := &stub{replies: []func(http.ResponseWriter){good, ok(body(0, 0, nil)), good}}
	c, waits := newClient(t, s)

	got, err := snapshot.Ingest(ctx, tx, c, day, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Fetched: 29, Failed: 1, Inserted: 2929}); got != want {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
	stored := rowsPerCombination(t, ctx, tx)
	failed := snapshot.Combinations()[1]
	if _, ok := stored[failed]; ok || len(stored) != 29 {
		t.Errorf("stored %d combinations including %+v: %v; want the other 29", len(stored), failed, stored)
	}
	if len(s.requests) != 30 || len(*waits) != 29 {
		t.Errorf("%d requests and %d waits, want 30 and 29", len(s.requests), len(*waits))
	}
}

func TestIngestCountsAFailedInsertAsAFailedCombination(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	bad := append(records(100), `{"data": {"main_heroid": 999, "main_hero_win_rate": 12345.5, "main_hero_appearance_rate": 0.01, "main_hero_ban_rate": 0.01}}`)
	good := ok(body(0, 101, records(101)))
	s := &stub{replies: []func(http.ResponseWriter){good, ok(body(0, 101, bad)), good}}
	c, _ := newClient(t, s)

	got, err := snapshot.Ingest(ctx, tx, c, day, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := (snapshot.Summary{Fetched: 29, Failed: 1, Inserted: 2929}); got != want {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
	if stored := rowsPerCombination(t, ctx, tx); len(stored) != 29 {
		t.Errorf("stored %d combinations, want 29: a failed insert leaves none of its rows", len(stored))
	}
}

func TestIngestStopsWhenCanceled(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	s := &stub{replies: []func(http.ResponseWriter){ok(body(0, 101, records(101)))}}
	c, _ := newClient(t, s)
	canceled, cancel := context.WithCancel(ctx)
	c.Sleep = func(time.Duration) { cancel() }

	got, err := snapshot.Ingest(canceled, tx, c, day, true)
	if err == nil {
		t.Fatal("want the cancellation")
	}
	if want := (snapshot.Summary{Fetched: 1, Inserted: 101}); got != want || len(s.requests) != 1 {
		t.Errorf("summary %+v and %d requests, want %+v and 1", got, len(s.requests), want)
	}
}
