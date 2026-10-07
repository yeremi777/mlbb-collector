package counter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/hero"
)

type fakeHeroes struct{ err error }

func (f fakeHeroes) Get(_ context.Context, uid string) (hero.Hero, error) {
	if f.err != nil {
		return hero.Hero{}, f.err
	}
	if uid == "nope" {
		return hero.Hero{}, hero.ErrNotFound
	}
	return hero.Hero{UID: uid}, nil
}

type fakeCounters struct {
	byTarget map[string][]WithHero
	err      error
}

func (f fakeCounters) ForTarget(_ context.Context, target string) ([]WithHero, error) {
	return f.byTarget[target], f.err
}

func get(t *testing.T, heroes fakeHeroes, counters fakeCounters, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(heroes, counters).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHandlerListsCounters(t *testing.T) {
	counters := fakeCounters{byTarget: map[string][]WithHero{"tigreal": {{
		TargetHeroID: "tigreal",
		CounterHero:  hero.Hero{UID: "diggie", MLID: 48, Name: "Diggie", Roles: []string{"support"}, Lanes: []string{"roam"}, Images: []byte(`{}`)},
		Reasons:      []string{"Diggie answers the engage."},
		CounterTypes: []string{"anti-cc"},
		Proof: []Proof{{ID: "diggie-vs-tigreal", Category: "crowd-control-counter", Priority: "primary", Impact: "high",
			Summary: "Cleanses the engage.", WorksBestWhen: []string{"Ultimate held."}, FailureCases: []string{"Ultimate baited."}}},
	}}}}
	rec := get(t, fakeHeroes{}, counters, "/api/heroes/tigreal/counters")
	want := `[{"targetHeroId":"tigreal","counterHero":{"uid":"diggie","mlid":"48","name":"Diggie","roles":["support"],"lanes":["roam"],"images":{}},` +
		`"reasons":["Diggie answers the engage."],"counterTypes":["anti-cc"],` +
		`"proof":[{"id":"diggie-vs-tigreal","category":"crowd-control-counter","priority":"primary","impact":"high",` +
		`"summary":"Cleanses the engage.","worksBestWhen":["Ultimate held."],"failureCases":["Ultimate baited."]}]}]` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("got %d %s\nwant 200 %s", rec.Code, rec.Body.String(), want)
	}
}

func TestHandlerAnswersMissingData(t *testing.T) {
	cases := []struct {
		name     string
		heroes   fakeHeroes
		counters fakeCounters
		target   string
		status   int
		want     string
	}{
		{"unknown hero", fakeHeroes{}, fakeCounters{}, "/api/heroes/nope/counters", http.StatusNotFound,
			`{"error":{"code":"hero_not_found","message":"Hero was not found in the dataset."}}`},
		{"hero without counters", fakeHeroes{}, fakeCounters{}, "/api/heroes/hirara/counters", http.StatusNotFound,
			`{"error":{"code":"counter_data_not_found","message":"Counter data was not found for the target hero."}}`},
		{"hero lookup fails", fakeHeroes{err: errors.New("db down")}, fakeCounters{}, "/api/heroes/tigreal/counters", http.StatusInternalServerError,
			`{"error":{"code":"internal_error","message":"Internal server error."}}`},
		{"counter lookup fails", fakeHeroes{}, fakeCounters{err: errors.New("db down")}, "/api/heroes/tigreal/counters", http.StatusInternalServerError,
			`{"error":{"code":"internal_error","message":"Internal server error."}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, c.heroes, c.counters, c.target)
			if rec.Code != c.status || rec.Body.String() != c.want+"\n" {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body.String(), c.status, c.want)
			}
		})
	}
}
