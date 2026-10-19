//go:build integration

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/patch"
	"github.com/yeremi777/mlbb-collector/internal/snapshot"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

var day = time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)

func noWait(time.Duration) {}

// moonton serves a full roster to every request and counts them.
func moonton(t *testing.T) (*snapshot.Client, *atomic.Int32) {
	t.Helper()
	recs := make([]string, 101)
	for i := range recs {
		recs[i] = fmt.Sprintf(`{"data": {"main_heroid": %d, "main_hero_win_rate": 0.5, "main_hero_appearance_rate": 0.0099, "main_hero_ban_rate": 0.01}}`, i+1)
	}
	body := `{"code": 0, "data": {"total": 101, "records": [` + strings.Join(recs, ",") + `]}}`
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	c := snapshot.NewClient(srv.URL, "probe/1.0")
	c.Sleep = noWait
	return c, &n
}

// liquipedia serves the committed Portal:Patches fixture and records how
// many Moonton requests preceded it.
func liquipedia(t *testing.T, moontonRequests *atomic.Int32, before *int32) *patch.Client {
	t.Helper()
	html, err := os.ReadFile("../../internal/patch/testdata/patches.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*before = moontonRequests.Load()
		_, _ = fmt.Fprintf(w, `{"parse": {"text": {"*": %q}}}`, html)
	}))
	t.Cleanup(srv.Close)
	c := patch.NewClient(srv.URL, "probe/1.0")
	c.Sleep = noWait
	return c
}

func TestAllStoresTheCalendarThenStatistics(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	stats, requests := moonton(t)
	before := int32(-1)

	if code := ingest(ctx, tx, command{name: "all"}, day, stats, liquipedia(t, requests, &before)); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	var patches, snapshots int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM raw.patches), (SELECT count(*) FROM raw.hero_rank_snapshots)`).Scan(&patches, &snapshots); err != nil {
		t.Fatal(err)
	}
	if patches != 113 || snapshots != 3030 || requests.Load() != 30 {
		t.Errorf("%d patches, %d snapshot rows, %d requests; want 113, 3030, 30", patches, snapshots, requests.Load())
	}
	if before != 0 {
		t.Errorf("the calendar was fetched after %d Moonton requests, want before any", before)
	}
}

func TestAllStoresStatisticsWhenTheCalendarFails(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	stats, _ := moonton(t)
	unreachable := patch.NewClient("http://127.0.0.1:1", "probe/1.0")
	unreachable.Sleep = noWait

	if code := ingest(ctx, tx, command{name: "all"}, day, stats, unreachable); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	var snapshots int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM raw.hero_rank_snapshots WHERE snapshot_date = $1`, day).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if snapshots != 3030 {
		t.Errorf("%d snapshot rows, want 3030", snapshots)
	}
}

func TestStatsExitsOneWhenACombinationFails(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"code": 0, "data": {"total": 0, "records": null}}`)
	}))
	t.Cleanup(srv.Close)
	stats := snapshot.NewClient(srv.URL, "probe/1.0")
	stats.Sleep = noWait

	if code := ingest(ctx, tx, command{name: "stats"}, day, stats, nil); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}
