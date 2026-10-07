package synergy

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

type fakeSynergies struct {
	byAnchor map[string][]WithHero
	err      error
}

func (f fakeSynergies) ForAnchor(_ context.Context, anchor string) ([]WithHero, error) {
	return f.byAnchor[anchor], f.err
}

func get(t *testing.T, heroes fakeHeroes, synergies fakeSynergies, anchor string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(heroes, synergies).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, anchor, nil))
	return rec
}

func TestHandlerListsSynergies(t *testing.T) {
	synergies := fakeSynergies{byAnchor: map[string][]WithHero{"tigreal": {{
		AnchorHeroID: "tigreal",
		SynergyHero:  hero.Hero{UID: "diggie", MLID: 48, Name: "Diggie", Roles: []string{"support"}, Lanes: []string{"roam"}, Images: []byte(`{}`)},
		Reasons:      []string{"Diggie answers the engage."},
		SynergyTypes: []string{"anti-cc"},
		Proof: []Proof{{ID: "diggie-vs-tigreal", Category: "crowd-control-chain", Priority: "primary", Impact: "high",
			Summary: "Cleanses the engage.", WorksBestWhen: []string{"Ultimate held."}, FailureCases: []string{"Ultimate baited."}}},
	}}}}
	rec := get(t, fakeHeroes{}, synergies, "/api/heroes/tigreal/synergies")
	want := `[{"anchorHeroId":"tigreal","synergyHero":{"uid":"diggie","mlid":"48","name":"Diggie","roles":["support"],"lanes":["roam"],"images":{}},` +
		`"reasons":["Diggie answers the engage."],"synergyTypes":["anti-cc"],` +
		`"proof":[{"id":"diggie-vs-tigreal","category":"crowd-control-chain","priority":"primary","impact":"high",` +
		`"summary":"Cleanses the engage.","worksBestWhen":["Ultimate held."],"failureCases":["Ultimate baited."]}]}]` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("got %d %s\nwant 200 %s", rec.Code, rec.Body.String(), want)
	}
}

func TestHandlerAnswersMissingData(t *testing.T) {
	cases := []struct {
		name      string
		heroes    fakeHeroes
		synergies fakeSynergies
		anchor    string
		status    int
		want      string
	}{
		{"unknown hero", fakeHeroes{}, fakeSynergies{}, "/api/heroes/nope/synergies", http.StatusNotFound,
			`{"error":{"code":"hero_not_found","message":"Hero was not found in the dataset."}}`},
		{"hero without synergies", fakeHeroes{}, fakeSynergies{}, "/api/heroes/hirara/synergies", http.StatusNotFound,
			`{"error":{"code":"synergy_data_not_found","message":"Synergy data was not found for the anchor hero."}}`},
		{"hero lookup fails", fakeHeroes{err: errors.New("db down")}, fakeSynergies{}, "/api/heroes/tigreal/synergies", http.StatusInternalServerError,
			`{"error":{"code":"internal_error","message":"Internal server error."}}`},
		{"synergy lookup fails", fakeHeroes{}, fakeSynergies{err: errors.New("db down")}, "/api/heroes/tigreal/synergies", http.StatusInternalServerError,
			`{"error":{"code":"internal_error","message":"Internal server error."}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, c.heroes, c.synergies, c.anchor)
			if rec.Code != c.status || rec.Body.String() != c.want+"\n" {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body.String(), c.status, c.want)
			}
		})
	}
}
