package stats

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/hero"
)

type fakeStore struct {
	board Board
	err   error
	got   *Query
}

func (f fakeStore) Board(_ context.Context, q Query) (Board, error) {
	if f.got != nil {
		*f.got = q
	}
	return f.board, f.err
}

func get(t *testing.T, store fakeStore, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(store).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHandlerReadsTheRankTierAndWindow(t *testing.T) {
	cases := map[string]Query{
		"/api/stats/heroes":                        {RankTier: "all", WindowDays: 7},
		"/api/stats/heroes?rank=glory&window=30":   {RankTier: "glory", WindowDays: 30},
		"/api/stats/heroes?rank=%20GLORY%20":       {RankTier: "glory", WindowDays: 7},
		"/api/stats/heroes?rank=&window=":          {RankTier: "all", WindowDays: 7},
		"/api/stats/heroes?window=abc":             {RankTier: "all", WindowDays: 7},
		"/api/stats/heroes?rank=diamond&window=10": {RankTier: "diamond", WindowDays: 10},
	}
	for target, want := range cases {
		var got Query
		if rec := get(t, fakeStore{got: &got}, target); rec.Code != http.StatusOK || got != want {
			t.Errorf("%s: got %d %+v, want 200 %+v", target, rec.Code, got, want)
		}
	}
}

var (
	tigreal = hero.Hero{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"},
		Images: json.RawMessage(`{"head": "https://example.test/t.png"}`)}
	miya = hero.Hero{UID: "miya", MLID: 1, Name: "Miya", Roles: []string{"marksman"}, Lanes: []string{"gold"}, Images: json.RawMessage(`{}`)}
)

func TestHandlerWritesTheBoard(t *testing.T) {
	date, since := "2026-10-30", "2026-10-20"
	board := Board{
		RankTier: "glory", WindowDays: 7, SnapshotDate: &date,
		Patch: &Patch{Version: "1.9.20", DaysSince: 12},
		Heroes: []Standing{
			{Hero: tigreal, WinRate: 0.512, AppearanceShare: 0.008, BanRate: 0.031,
				Change:   &Change{Since: since, WinRate: 0.004, AppearanceShare: -0.001, BanRate: 0.002},
				Position: Position{WinRate: 1, Appearance: 88}},
			{Hero: miya, WinRate: 0.5, AppearanceShare: 0.01, BanRate: 0.002, Position: Position{WinRate: 2, Appearance: 3}},
		},
	}
	rec := get(t, fakeStore{board: board}, "/api/stats/heroes?rank=glory")
	want := `{"rankTier":"glory","windowDays":7,"snapshotDate":"2026-10-30",` +
		`"patch":{"version":"1.9.20","daysSince":12,"windowCrossesPatch":false},"heroes":[` +
		`{"hero":{"uid":"tigreal","mlid":"6","name":"Tigreal","roles":["tank"],"lanes":["roam"],"images":{"head":"https://example.test/t.png"}},` +
		`"winRate":0.512,"appearanceShare":0.008,"banRate":0.031,` +
		`"change":{"since":"2026-10-20","winRate":0.004,"appearanceShare":-0.001,"banRate":0.002},"position":{"winRate":1,"appearance":88}},` +
		`{"hero":{"uid":"miya","mlid":"1","name":"Miya","roles":["marksman"],"lanes":["gold"],"images":{}},` +
		`"winRate":0.5,"appearanceShare":0.01,"banRate":0.002,"change":null,"position":{"winRate":2,"appearance":3}}]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("got %d %q\nwant 200 %q", rec.Code, rec.Body.String(), want)
	}
}

func TestHandlerHidesStoreFailures(t *testing.T) {
	rec := get(t, fakeStore{err: errors.New("connection refused at 10.0.0.5")}, "/api/stats/heroes")
	want := `{"error":{"code":"internal_error","message":"Internal server error."}}` + "\n"
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != want {
		t.Errorf("got %d %q, want 500 %q", rec.Code, rec.Body.String(), want)
	}
}
