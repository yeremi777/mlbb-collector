package analysis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/config"
	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

// datasetStores serve the heroes, Counters, and Synergies of data/ the way
// the repositories do.
type datasetStores struct {
	heroes    map[string]hero.Hero
	counters  map[string][]counter.WithHero
	synergies map[string][]synergy.WithHero
}

func newDatasetStores(t *testing.T) datasetStores {
	t.Helper()
	target, counters, synergies := tigreal(t)
	s := datasetStores{heroes: map[string]hero.Hero{}, counters: map[string][]counter.WithHero{}, synergies: map[string][]synergy.WithHero{}}
	ds, err := dataset.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range ds.Heroes {
		s.heroes[h.UID] = h
	}
	for _, m := range counters {
		c := counter.WithHero{TargetHeroID: target.UID, CounterHero: m.Partner, Reasons: m.Reasons, CounterTypes: m.Types}
		for _, p := range m.Proof {
			c.Proof = append(c.Proof, counter.Proof(p))
		}
		s.counters[target.UID] = append(s.counters[target.UID], c)
	}
	for _, m := range synergies {
		y := synergy.WithHero{AnchorHeroID: target.UID, SynergyHero: m.Partner, Reasons: m.Reasons, SynergyTypes: m.Types}
		for _, p := range m.Proof {
			y.Proof = append(y.Proof, synergy.Proof(p))
		}
		s.synergies[target.UID] = append(s.synergies[target.UID], y)
	}
	return s
}

func (s datasetStores) Get(_ context.Context, uid string) (hero.Hero, error) {
	h, ok := s.heroes[uid]
	if !ok {
		return hero.Hero{}, hero.ErrNotFound
	}
	return h, nil
}

func (s datasetStores) ForTarget(_ context.Context, target string) ([]counter.WithHero, error) {
	return s.counters[target], nil
}

func (s datasetStores) ForAnchor(_ context.Context, anchor string) ([]synergy.WithHero, error) {
	return s.synergies[anchor], nil
}

func (s datasetStores) post(t *testing.T, analyzer *Analyzer, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(s, s, s, analyzer, nil).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func mustLoadAI(t *testing.T, vars map[string]string) config.AI {
	t.Helper()
	cfg, err := config.LoadAI(func(key string) string { return vars[key] })
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestMockDerivesItsAnswerFromTheMatchups(t *testing.T) {
	ms := []Matchup{
		{Partner: hero.Hero{UID: "akai"}, Reasons: []string{"Akai holds the line."}},
		{Partner: hero.Hero{UID: "diggie"}, Reasons: []string{"Diggie cleanses.", "Diggie revives."},
			Proof: []Proof{{ID: "p1", WorksBestWhen: []string{"Ultimate held."}, FailureCases: []string{"Ultimate baited."}}, {ID: "p2"}}},
	}
	a := New(Mock{}, testConfig)
	ranked, err := a.ScoreCounters(context.Background(), hero.Hero{UID: "tigreal"}, ms, "en")
	if want := []Ranked{{1, "diggie", 80, 70}, {2, "akai", 60, 60}}; err != nil || !reflect.DeepEqual(ranked, want) {
		t.Errorf("counters: got %+v, %v; want %+v", ranked, err, want)
	}
	ranked, err = a.ScoreSynergies(context.Background(), hero.Hero{UID: "tigreal"}, ms, "en")
	if want := []Ranked{{1, "diggie", 80, 70}, {2, "akai", 60, 60}}; err != nil || !reflect.DeepEqual(ranked, want) {
		t.Errorf("synergies: got %+v, %v; want %+v", ranked, err, want)
	}
	for name, detail := range map[string]func(context.Context, hero.Hero, Matchup, string) (Detail, error){
		"counter": a.CounterDetail, "synergy": a.SynergyDetail,
	} {
		got, err := detail(context.Background(), hero.Hero{UID: "tigreal"}, ms[1], "en")
		want := Detail{Score: 80, Confidence: 70, Summary: "Diggie cleanses.", Strengths: []string{"Diggie cleanses.", "Diggie revives."},
			Conditions: []string{"Ultimate held."}, FailureCases: []string{"Ultimate baited."}, EvidenceIDs: []string{"p1", "p2"}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s detail: got %+v, %v\nwant %+v", name, got, err, want)
		}
	}
}

func TestMockAnswersEveryRouteForTigrealTheSameEachTime(t *testing.T) {
	stores := newDatasetStores(t)
	analyzer := NewFromConfig(mustLoadAI(t, map[string]string{"AI_PROVIDERS": "mock", "AI_ANALYSIS_CACHE_TTL_SECONDS": "0"}))
	for path, body := range map[string]string{
		"/api/counters/analyze-score":   `{"targetHeroId":"tigreal"}`,
		"/api/counters/analyze-detail":  `{"targetHeroId":"tigreal","counterHeroId":"diggie","language":"id"}`,
		"/api/synergies/analyze-score":  `{"anchorHeroId":"tigreal","language":"en"}`,
		"/api/synergies/analyze-detail": `{"anchorHeroId":"tigreal","synergyHeroId":"pharsa"}`,
	} {
		first, again := stores.post(t, analyzer, path, body), stores.post(t, analyzer, path, body)
		if first.Code != 200 || again.Code != 200 || first.Body.String() != again.Body.String() {
			t.Errorf("%s: got %d %s then %d %s; want the same 200 twice", path, first.Code, first.Body, again.Code, again.Body)
		}
	}
}

func TestNewFromConfigSkipsAProviderWithoutAKey(t *testing.T) {
	if got := NewFromConfig(mustLoadAI(t, map[string]string{"AI_PROVIDERS": "openrouter,opencode_zen"})); got != nil {
		t.Errorf("no key: got an analyzer, want none")
	}
	if got := NewFromConfig(mustLoadAI(t, map[string]string{})); got != nil {
		t.Errorf("no provider listed: got an analyzer, want none")
	}

	analyzer := NewFromConfig(mustLoadAI(t, map[string]string{"AI_PROVIDERS": "openrouter,mock", "AI_TIMEOUT_SECONDS": "5"}))
	if analyzer == nil {
		t.Fatal("openrouter without a key, then mock: got no analyzer, want mock")
	}
	rec := newDatasetStores(t).post(t, analyzer, "/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`)
	if rec.Code != 200 {
		t.Errorf("got %d %s; want mock's answer", rec.Code, rec.Body)
	}
}
