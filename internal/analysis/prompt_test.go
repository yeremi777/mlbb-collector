package analysis

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/ai"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// tigreal returns Tigreal with its Counters and Synergies from data/, ordered
// as the repositories return them: by partner hero ID, each Proof by ID.
func tigreal(t *testing.T) (target hero.Hero, counters, synergies []Matchup) {
	t.Helper()
	ds, err := dataset.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	heroes := make(map[string]hero.Hero, len(ds.Heroes))
	for _, h := range ds.Heroes {
		heroes[h.UID] = h
	}
	for _, c := range ds.Counters {
		if c.TargetHeroID != "tigreal" {
			continue
		}
		m := Matchup{Partner: heroes[c.CounterHeroID], Reasons: c.Reasons, Types: c.CounterTypes}
		for _, p := range c.Proof {
			m.Proof = append(m.Proof, Proof(p))
		}
		counters = append(counters, m)
	}
	for _, s := range ds.Synergies {
		if s.AnchorHeroID != "tigreal" {
			continue
		}
		m := Matchup{Partner: heroes[s.SynergyHeroID], Reasons: s.Reasons, Types: s.SynergyTypes}
		for _, p := range s.Proof {
			m.Proof = append(m.Proof, Proof(p))
		}
		synergies = append(synergies, m)
	}
	for _, ms := range [][]Matchup{counters, synergies} {
		slices.SortFunc(ms, func(a, b Matchup) int { return cmp.Compare(a.Partner.UID, b.Partner.UID) })
		for _, m := range ms {
			slices.SortFunc(m.Proof, func(a, b Proof) int { return cmp.Compare(a.ID, b.ID) })
		}
	}
	return heroes["tigreal"], counters, synergies
}

func partner(t *testing.T, ms []Matchup, uid string) Matchup {
	t.Helper()
	i := slices.IndexFunc(ms, func(m Matchup) bool { return m.Partner.UID == uid })
	if i < 0 {
		t.Fatalf("no matchup with %s", uid)
	}
	return ms[i]
}

// assertGoldenMessages compares messages with testdata/prompts/name, captured
// from dev's prompt builders.
func assertGoldenMessages(t *testing.T, name string, got []ai.Message) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "prompts", name))
	if err != nil {
		t.Fatal(err)
	}
	var want []ai.Message
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s: %d messages, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: message %d\ngot  %+v\nwant %+v", name, i, got[i], want[i])
		}
	}
}

func TestPromptsEqualDevs(t *testing.T) {
	target, counters, synergies := tigreal(t)
	diggie, pharsa := partner(t, counters, "diggie"), partner(t, synergies, "pharsa")
	for _, lang := range []string{"en", "id"} {
		assertGoldenMessages(t, "counter-score."+lang+".json", counterScoringMessages(target, counters, lang))
		assertGoldenMessages(t, "counter-detail."+lang+".json", counterDetailMessages(target, diggie, lang))
		assertGoldenMessages(t, "synergy-score."+lang+".json", synergyScoringMessages(target, synergies, lang))
		assertGoldenMessages(t, "synergy-detail."+lang+".json", synergyDetailMessages(target, pharsa, lang))
	}
}
