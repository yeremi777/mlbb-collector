package analysis

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

// fakeHeroes knows tigreal, diggie, pharsa, and hirara, a hero with no
// Counter or Synergy; err fails every lookup.
type fakeHeroes struct{ err error }

func (f fakeHeroes) Get(_ context.Context, uid string) (hero.Hero, error) {
	if f.err != nil {
		return hero.Hero{}, f.err
	}
	switch uid {
	case "tigreal", "diggie", "pharsa", "hirara":
		return hero.Hero{UID: uid}, nil
	}
	return hero.Hero{}, hero.ErrNotFound
}

// fakeMatchups gives tigreal the Counter diggie and the Synergy pharsa.
type fakeMatchups struct{ err error }

func (f fakeMatchups) ForTarget(_ context.Context, target string) ([]counter.WithHero, error) {
	if f.err != nil || target != "tigreal" {
		return nil, f.err
	}
	return []counter.WithHero{{TargetHeroID: "tigreal", CounterHero: hero.Hero{UID: "diggie"},
		Proof: []counter.Proof{{ID: "diggie-proof"}}}}, nil
}

func (f fakeMatchups) ForAnchor(_ context.Context, anchor string) ([]synergy.WithHero, error) {
	if f.err != nil || anchor != "tigreal" {
		return nil, f.err
	}
	return []synergy.WithHero{{AnchorHeroID: "tigreal", SynergyHero: hero.Hero{UID: "pharsa"},
		Proof: []synergy.Proof{{ID: "pharsa-proof"}}}}, nil
}

type route struct {
	heroes   fakeHeroes
	matchups fakeMatchups
	provider *scriptedProvider // nil: no provider configured
}

func (rt route) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var analyzer *Analyzer
	if rt.provider != nil {
		analyzer = New(rt.provider, testConfig)
	}
	mux := http.NewServeMux()
	NewHandler(rt.heroes, rt.matchups, rt.matchups, analyzer).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func errorBody(code, message string) string {
	return `{"error":{"code":"` + code + `","message":"` + message + `"}}` + "\n"
}

func TestEachCheckAnswersBeforeTheNext(t *testing.T) {
	const (
		counterScore  = "/api/counters/analyze-score"
		counterDetail = "/api/counters/analyze-detail"
		synergyScore  = "/api/synergies/analyze-score"
		synergyDetail = "/api/synergies/analyze-detail"
	)
	var (
		invalidJSON   = errorBody("invalid_request", "Request body is not valid JSON.")
		badLanguage   = errorBody("invalid_request", "language must be 'en' or 'id'.")
		notConfigured = errorBody("ai_provider_not_configured", "AI provider is not configured. Set the required API key in .env.")
		internal      = errorBody("internal_error", "Internal server error.")
	)
	type check struct {
		name, path, body string
		noProvider       bool
		heroes           fakeHeroes
		matchups         fakeMatchups
		status           int
		want             string
	}
	var checks []check
	// Every request below also breaks every later check, so its answer proves
	// the earlier one ran first.
	for _, path := range []string{counterScore, counterDetail, synergyScore, synergyDetail} {
		checks = append(checks,
			check{"not JSON", path, `{`, true, fakeHeroes{}, fakeMatchups{}, 422, invalidJSON},
			check{"a JSON value of the wrong type", path, `[]`, true, fakeHeroes{}, fakeMatchups{}, 422, invalidJSON},
			check{"no provider", path, `{"targetHeroId":"nope","anchorHeroId":"nope","language":"fr"}`, true, fakeHeroes{}, fakeMatchups{}, 504, notConfigured},
			check{"language", path, `{"targetHeroId":"nope","anchorHeroId":"nope","language":"fr"}`, false, fakeHeroes{}, fakeMatchups{}, 422, badLanguage},
		)
	}
	checks = append(checks,
		check{"unknown target", counterScore, `{"targetHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("target_hero_not_found", "Hero was not found in the dataset.")},
		check{"target lookup fails", counterScore, `{"targetHeroId":"tigreal"}`, false, fakeHeroes{err: errors.New("db down")}, fakeMatchups{}, 500, internal},
		check{"target without counters", counterScore, `{"targetHeroId":"hirara"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("counter_data_not_found", "Counter data was not found for the target hero.")},
		check{"counter lookup fails", counterScore, `{"targetHeroId":"tigreal"}`, false, fakeHeroes{}, fakeMatchups{err: errors.New("db down")}, 500, internal},
		check{"unknown anchor", synergyScore, `{"anchorHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("anchor_hero_not_found", "Hero was not found in the dataset.")},
		check{"anchor without synergies", synergyScore, `{"anchorHeroId":"hirara"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("synergy_data_not_found", "Synergy data was not found for the anchor hero.")},
		check{"synergy lookup fails", synergyScore, `{"anchorHeroId":"tigreal"}`, false, fakeHeroes{}, fakeMatchups{err: errors.New("db down")}, 500, internal},

		check{"detail: unknown target", counterDetail, `{"targetHeroId":"nope","counterHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("target_hero_not_found", "Hero was not found in the dataset.")},
		check{"detail: target without counters", counterDetail, `{"targetHeroId":"hirara","counterHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("counter_data_not_found", "Counter data was not found for the target hero.")},
		check{"detail: unknown counter hero", counterDetail, `{"targetHeroId":"tigreal","counterHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("counter_hero_not_found", "Counter hero was not found in the dataset.")},
		check{"detail: not a counter", counterDetail, `{"targetHeroId":"tigreal","counterHeroId":"pharsa"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("counter_matchup_not_found", "Counter matchup was not found for the target hero.")},
		check{"detail: unknown anchor", synergyDetail, `{"anchorHeroId":"nope","synergyHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("anchor_hero_not_found", "Hero was not found in the dataset.")},
		check{"detail: anchor without synergies", synergyDetail, `{"anchorHeroId":"hirara","synergyHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("synergy_data_not_found", "Synergy data was not found for the anchor hero.")},
		check{"detail: unknown synergy hero", synergyDetail, `{"anchorHeroId":"tigreal","synergyHeroId":"nope"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("synergy_hero_not_found", "Synergy hero was not found in the dataset.")},
		check{"detail: not a synergy", synergyDetail, `{"anchorHeroId":"tigreal","synergyHeroId":"diggie"}`, false, fakeHeroes{}, fakeMatchups{}, 404,
			errorBody("synergy_matchup_not_found", "Synergy matchup was not found for the anchor hero.")},
	)
	for _, c := range checks {
		t.Run(c.path+" "+c.name, func(t *testing.T) {
			rt := route{heroes: c.heroes, matchups: c.matchups}
			if !c.noProvider {
				rt.provider = &scriptedProvider{}
			}
			rec := rt.post(t, c.path, c.body)
			if rec.Code != c.status || rec.Body.String() != c.want {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body.String(), c.status, c.want)
			}
			if rt.provider != nil && len(rt.provider.received) != 0 {
				t.Errorf("the provider was asked %d times, want none", len(rt.provider.received))
			}
		})
	}
}

func TestTheRoutesAnswerTheAnalysis(t *testing.T) {
	for _, tt := range []struct {
		path, body, answer string
		status             int
		want               string
	}{
		{"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`,
			`{"recommendations":[{"counterHeroId":"diggie","score":90,"confidence":80}]}`, 200,
			`{"targetHeroId":"tigreal","source":"ai","recommendations":[{"rank":1,"counterHeroId":"diggie","score":90,"confidence":80}]}`},
		{"/api/synergies/analyze-score", `{"anchorHeroId":"tigreal","language":"id"}`,
			`{"recommendations":[{"synergyHeroId":"pharsa","score":95,"confidence":85}]}`, 200,
			`{"anchorHeroId":"tigreal","source":"ai","recommendations":[{"rank":1,"synergyHeroId":"pharsa","score":95,"confidence":85}]}`},
		{"/api/counters/analyze-detail", `{"targetHeroId":"tigreal","counterHeroId":"diggie","language":"en"}`,
			`{"score":90,"confidence":80,"summary":"s","strengths":["x"],"evidenceIds":["diggie-proof"]}`, 200,
			`{"targetHeroId":"tigreal","counterHeroId":"diggie","source":"ai","score":90,"confidence":80,"summary":"s",` +
				`"strengths":["x"],"conditions":[],"failureCases":[],"evidenceIds":["diggie-proof"]}`},
		{"/api/synergies/analyze-detail", `{"anchorHeroId":"tigreal","synergyHeroId":"pharsa"}`,
			`{"score":95,"confidence":85,"summary":"s","strengths":["x"],"conditions":["c"],"failureCases":["f"],"evidenceIds":[]}`, 200,
			`{"anchorHeroId":"tigreal","synergyHeroId":"pharsa","source":"ai","score":95,"confidence":85,"summary":"s",` +
				`"strengths":["x"],"conditions":["c"],"failureCases":["f"],"evidenceIds":[]}`},
		{"/api/counters/analyze-score", `{"targetHeroId":"tigreal"}`, `{}`, 502,
			strings.TrimSuffix(errorBody("ai_provider_error", "Invalid scoring response from model: recommendations must be an array"), "\n")},
	} {
		t.Run(tt.path, func(t *testing.T) {
			rt := route{provider: &scriptedProvider{answers: []string{tt.answer}}}
			rec := rt.post(t, tt.path, tt.body)
			if rec.Code != tt.status || rec.Body.String() != tt.want+"\n" {
				t.Errorf("got %d %s\nwant %d %s", rec.Code, rec.Body.String(), tt.status, tt.want)
			}
		})
	}
}

func TestATimeoutAnswers504(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(fakeHeroes{}, fakeMatchups{}, fakeMatchups{}, New(&stalledProvider{}, Config{Timeout: 1})).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/counters/analyze-score", strings.NewReader(`{"targetHeroId":"tigreal"}`)))
	if want := errorBody("ai_provider_timeout", "AI provider did not answer in time."); rec.Code != 504 || rec.Body.String() != want {
		t.Errorf("got %d %s, want 504 %s", rec.Code, rec.Body.String(), want)
	}
}
