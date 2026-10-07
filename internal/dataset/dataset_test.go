package dataset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

const validHeroes = `[
  {"uid": "tigreal", "mlid": "6", "name": "Tigreal", "roles": ["tank"], "lanes": ["roam"], "images": {"head": "https://example.test/tigreal.png"}},
  {"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]},
  {"uid": "pharsa", "mlid": "70", "name": "Pharsa", "roles": ["mage"], "lanes": ["mid", "gold"]}
]`

// writeDataset lays files out under a fresh directory, keyed by their path
// relative to it, and returns the directory.
func writeDataset(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadHeroes(t *testing.T) {
	ds, err := Load(writeDataset(t, map[string]string{"heroes.json": validHeroes}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []hero.Hero{
		{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"},
			Images: json.RawMessage(`{"head": "https://example.test/tigreal.png"}`)},
		{UID: "diggie", MLID: 48, Name: "Diggie", Roles: []string{"support"}, Lanes: []string{"roam"},
			Images: json.RawMessage(`{}`)},
		{UID: "pharsa", MLID: 70, Name: "Pharsa", Roles: []string{"mage"}, Lanes: []string{"mid", "gold"},
			Images: json.RawMessage(`{}`)},
	}
	if !reflect.DeepEqual(ds.Heroes, want) {
		t.Errorf("heroes:\n got %+v\nwant %+v", ds.Heroes, want)
	}
}

func TestLoadRejectsInvalidHeroes(t *testing.T) {
	// replaceDiggie swaps the second hero's object for the given one.
	replaceDiggie := func(obj string) string {
		return strings.Replace(validHeroes,
			`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`, obj, 1)
	}
	cases := []struct {
		name   string
		heroes string
		want   string
	}{
		{"invalid JSON", `[{"uid": }]`, "heroes.json: invalid JSON at byte 10"},
		{"not an array", `{"uid": "tigreal"}`, "heroes.json: not a JSON array"},
		{"empty", `[]`, "heroes.json: no heroes"},
		{"row not an object", replaceDiggie(`"diggie"`), "heroes.json[1]: not a JSON object"},
		{"missing key", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"]}`),
			"heroes.json[1].lanes: missing"},
		{"unknown key", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"], "score": 90}`),
			"heroes.json[1].score: unknown key"},
		{"null value", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": null}`),
			"heroes.json[1].lanes: null"},
		{"wrong type", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": "support", "lanes": ["roam"]}`),
			"heroes.json[1].roles: want []string, got string"},
		{"mlid not digits", replaceDiggie(`{"uid": "diggie", "mlid": "4a", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			`heroes.json[1].mlid: want a positive integer as decimal digits, got "4a"`},
		{"mlid zero", replaceDiggie(`{"uid": "diggie", "mlid": "0", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			`heroes.json[1].mlid: want a positive integer as decimal digits, got "0"`},
		{"mlid a number", replaceDiggie(`{"uid": "diggie", "mlid": 48, "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			"heroes.json[1].mlid: want string, got number"},
		{"blank uid", replaceDiggie(`{"uid": " ", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			"heroes.json[1].uid: blank"},
		{"blank name", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "", "roles": ["support"], "lanes": ["roam"]}`),
			"heroes.json[1].name: blank"},
		{"no roles", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": [], "lanes": ["roam"]}`),
			"heroes.json[1].roles: empty"},
		{"unknown role", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["healer"], "lanes": ["roam"]}`),
			`heroes.json[1].roles[0]: "healer" is not a role`},
		{"repeated role", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support", "support"], "lanes": ["roam"]}`),
			`heroes.json[1].roles[1]: "support" repeats`},
		{"unknown lane", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["top"]}`),
			`heroes.json[1].lanes[0]: "top" is not a lane`},
		{"repeated lane", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam", "roam"]}`),
			`heroes.json[1].lanes[1]: "roam" repeats`},
		{"images not an object", replaceDiggie(`{"uid": "diggie", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"], "images": "head.png"}`),
			"heroes.json[1].images: not a JSON object"},
		{"duplicate hero id", replaceDiggie(`{"uid": "tigreal", "mlid": "48", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			`heroes.json[1].uid: "tigreal" repeats heroes.json[0]`},
		{"duplicate moonton id", replaceDiggie(`{"uid": "diggie", "mlid": "6", "name": "Diggie", "roles": ["support"], "lanes": ["roam"]}`),
			"heroes.json[1].mlid: 6 repeats heroes.json[0]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(writeDataset(t, map[string]string{"heroes.json": c.heroes}))
			if err == nil || err.Error() != c.want {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

const validCounters = `[
  {"targetHeroId": "tigreal", "counterHeroId": "diggie", "reasons": ["Diggie answers Tigreal's engage."], "counterTypes": ["anti-cc"],
   "proof": [{"id": "diggie-vs-tigreal", "category": "crowd-control-counter", "priority": "primary", "impact": "high",
              "summary": "Diggie's ultimate cleanses the engage.", "worksBestWhen": ["Diggie holds his ultimate."], "failureCases": ["Tigreal baits it."]}]},
  {"targetHeroId": "tigreal", "counterHeroId": "pharsa", "reasons": ["Pharsa outranges Tigreal."], "counterTypes": ["poke"],
   "proof": [{"id": "pharsa-vs-tigreal", "category": "range-advantage", "priority": "secondary", "impact": "medium",
              "summary": "Pharsa hits from beyond engage range.", "worksBestWhen": ["Pharsa keeps distance."], "failureCases": ["Tigreal flanks."]}]}
]`

const validSynergies = `[
  {"anchorHeroId": "tigreal", "synergyHeroId": "pharsa", "reasons": ["Pharsa follows Tigreal's engage."], "synergyTypes": ["cc-chain"],
   "proof": [{"id": "tigreal-with-pharsa", "category": "engage-follow-up", "priority": "primary", "impact": "high",
              "summary": "Pharsa's ultimate lands on grouped enemies.", "worksBestWhen": ["Pharsa is close."], "failureCases": ["Enemies spread."]}]}
]`

func validFiles() map[string]string {
	return map[string]string{
		"heroes.json":            validHeroes,
		"counters/tigreal.json":  validCounters,
		"synergies/tigreal.json": validSynergies,
	}
}

func TestLoadMatchups(t *testing.T) {
	ds, err := Load(writeDataset(t, validFiles()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantCounters := []counter.Counter{
		{TargetHeroID: "tigreal", CounterHeroID: "diggie", Reasons: []string{"Diggie answers Tigreal's engage."}, CounterTypes: []string{"anti-cc"},
			Proof: []counter.Proof{{ID: "diggie-vs-tigreal", Category: "crowd-control-counter", Priority: "primary", Impact: "high",
				Summary: "Diggie's ultimate cleanses the engage.", WorksBestWhen: []string{"Diggie holds his ultimate."}, FailureCases: []string{"Tigreal baits it."}}}},
		{TargetHeroID: "tigreal", CounterHeroID: "pharsa", Reasons: []string{"Pharsa outranges Tigreal."}, CounterTypes: []string{"poke"},
			Proof: []counter.Proof{{ID: "pharsa-vs-tigreal", Category: "range-advantage", Priority: "secondary", Impact: "medium",
				Summary: "Pharsa hits from beyond engage range.", WorksBestWhen: []string{"Pharsa keeps distance."}, FailureCases: []string{"Tigreal flanks."}}}},
	}
	if !reflect.DeepEqual(ds.Counters, wantCounters) {
		t.Errorf("counters:\n got %+v\nwant %+v", ds.Counters, wantCounters)
	}
	wantSynergies := []synergy.Synergy{
		{AnchorHeroID: "tigreal", SynergyHeroID: "pharsa", Reasons: []string{"Pharsa follows Tigreal's engage."}, SynergyTypes: []string{"cc-chain"},
			Proof: []synergy.Proof{{ID: "tigreal-with-pharsa", Category: "engage-follow-up", Priority: "primary", Impact: "high",
				Summary: "Pharsa's ultimate lands on grouped enemies.", WorksBestWhen: []string{"Pharsa is close."}, FailureCases: []string{"Enemies spread."}}}},
	}
	if !reflect.DeepEqual(ds.Synergies, wantSynergies) {
		t.Errorf("synergies:\n got %+v\nwant %+v", ds.Synergies, wantSynergies)
	}
}

func TestLoadRejectsInvalidMatchups(t *testing.T) {
	cases := []struct {
		name string
		edit func(files map[string]string)
		want string
	}{
		{"empty counter file", func(f map[string]string) { f["counters/tigreal.json"] = `[]` },
			"counters/tigreal.json: no counters"},
		{"counter file not an array", func(f map[string]string) { f["counters/tigreal.json"] = `{}` },
			"counters/tigreal.json: not a JSON array"},
		{"unknown counter key", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"counterTypes": ["poke"],`, `"counterTypes": ["poke"], "score": 90,`, 1)
		}, "counters/tigreal.json[1].score: unknown key"},
		{"missing counter key", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"counterTypes": ["poke"],`, ``, 1)
		}, "counters/tigreal.json[1].counterTypes: missing"},
		{"proof not an object", func(f map[string]string) {
			f["counters/tigreal.json"] = `[{"targetHeroId": "tigreal", "counterHeroId": "diggie", "reasons": ["r"], "counterTypes": ["t"], "proof": ["p"]}]`
		}, "counters/tigreal.json[0].proof[0]: not a JSON object"},
		{"unknown proof key", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"impact": "medium",`, `"impact": "medium", "scoreHint": 3,`, 1)
		}, "counters/tigreal.json[1].proof[0].scoreHint: unknown key"},
		{"missing proof key", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `, "failureCases": ["Tigreal flanks."]`, ``, 1)
		}, "counters/tigreal.json[1].proof[0].failureCases: missing"},
		{"no reasons", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["Pharsa outranges Tigreal."]`, `[]`, 1)
		}, "counters/tigreal.json[1].reasons: empty"},
		{"blank reason", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["Pharsa outranges Tigreal."]`, `["ok", " "]`, 1)
		}, "counters/tigreal.json[1].reasons[1]: blank"},
		{"no counter types", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["poke"]`, `[]`, 1)
		}, "counters/tigreal.json[1].counterTypes: empty"},
		{"blank counter type", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["poke"]`, `[""]`, 1)
		}, "counters/tigreal.json[1].counterTypes[0]: blank"},
		{"no proof", func(f map[string]string) {
			f["counters/tigreal.json"] = `[{"targetHeroId": "tigreal", "counterHeroId": "diggie", "reasons": ["r"], "counterTypes": ["t"], "proof": []}]`
		}, "counters/tigreal.json[0].proof: empty"},
		{"blank proof id", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"pharsa-vs-tigreal"`, `""`, 1)
		}, "counters/tigreal.json[1].proof[0].id: blank"},
		{"synergy category on a counter", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"range-advantage"`, `"engage-follow-up"`, 1)
		}, `counters/tigreal.json[1].proof[0].category: "engage-follow-up" is not a counter proof category`},
		{"bad priority", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"secondary"`, `"tertiary"`, 1)
		}, `counters/tigreal.json[1].proof[0].priority: "tertiary" is not a priority`},
		{"bad impact", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"medium"`, `"huge"`, 1)
		}, `counters/tigreal.json[1].proof[0].impact: "huge" is not an impact`},
		{"blank summary", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"Pharsa hits from beyond engage range."`, `"  "`, 1)
		}, "counters/tigreal.json[1].proof[0].summary: blank"},
		{"no works-best condition", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["Pharsa keeps distance."]`, `[]`, 1)
		}, "counters/tigreal.json[1].proof[0].worksBestWhen: empty"},
		{"blank failure case", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `["Tigreal flanks."]`, `[" "]`, 1)
		}, "counters/tigreal.json[1].proof[0].failureCases[0]: blank"},
		{"target differs from file name", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"targetHeroId": "tigreal", "counterHeroId": "pharsa"`, `"targetHeroId": "diggie", "counterHeroId": "pharsa"`, 1)
		}, `counters/tigreal.json[1].targetHeroId: "diggie" does not match the file name`},
		{"self pair", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"counterHeroId": "pharsa"`, `"counterHeroId": "tigreal"`, 1)
		}, `counters/tigreal.json[1].counterHeroId: "tigreal" is the target hero`},
		{"unknown counter hero", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(validCounters, `"counterHeroId": "pharsa"`, `"counterHeroId": "nana"`, 1)
		}, `counters/tigreal.json[1].counterHeroId: "nana" is not in heroes.json`},
		{"unknown target hero", func(f map[string]string) {
			f["counters/nana.json"] = strings.ReplaceAll(strings.ReplaceAll(validCounters, `"targetHeroId": "tigreal"`, `"targetHeroId": "nana"`), `-vs-tigreal`, `-vs-nana`)
		}, `counters/nana.json[0].targetHeroId: "nana" is not in heroes.json`},
		{"duplicate pair", func(f map[string]string) {
			f["counters/tigreal.json"] = strings.Replace(strings.Replace(validCounters, `"counterHeroId": "pharsa"`, `"counterHeroId": "diggie"`, 1), `"pharsa-vs-tigreal"`, `"diggie-vs-tigreal-2"`, 1)
		}, `counters/tigreal.json[1].counterHeroId: "diggie" repeats counters/tigreal.json[0]`},
		{"proof id reused across files", func(f map[string]string) {
			f["counters/diggie.json"] = `[{"targetHeroId": "diggie", "counterHeroId": "pharsa", "reasons": ["r"], "counterTypes": ["t"],
			  "proof": [{"id": "diggie-vs-tigreal", "category": "kiting", "priority": "primary", "impact": "low", "summary": "s", "worksBestWhen": ["w"], "failureCases": ["f"]}]}]`
		}, `counters/tigreal.json[0].proof[0].id: "diggie-vs-tigreal" repeats counters/diggie.json[0].proof[0]`},
		{"anchor differs from file name", func(f map[string]string) {
			f["synergies/tigreal.json"] = strings.Replace(validSynergies, `"anchorHeroId": "tigreal"`, `"anchorHeroId": "diggie"`, 1)
		}, `synergies/tigreal.json[0].anchorHeroId: "diggie" does not match the file name`},
		{"counter category on a synergy", func(f map[string]string) {
			f["synergies/tigreal.json"] = strings.Replace(validSynergies, `"engage-follow-up"`, `"kiting"`, 1)
		}, `synergies/tigreal.json[0].proof[0].category: "kiting" is not a synergy proof category`},
		{"unknown synergy hero", func(f map[string]string) {
			f["synergies/tigreal.json"] = strings.Replace(validSynergies, `"synergyHeroId": "pharsa"`, `"synergyHeroId": "nana"`, 1)
		}, `synergies/tigreal.json[0].synergyHeroId: "nana" is not in heroes.json`},
		{"synergy reuses a counter proof id", func(f map[string]string) {
			f["synergies/tigreal.json"] = strings.Replace(validSynergies, `"tigreal-with-pharsa"`, `"diggie-vs-tigreal"`, 1)
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := validFiles()
			c.edit(files)
			_, err := Load(writeDataset(t, files))
			if c.want == "" {
				if err != nil {
					t.Errorf("err = %v, want none: proof ids are unique per kind", err)
				}
				return
			}
			if err == nil || err.Error() != c.want {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

// TestLoadRepositoryData loads the authored data/ itself, so an authoring
// mistake fails here before it reaches a seed.
func TestLoadRepositoryData(t *testing.T) {
	const dir = "../../data"
	ds, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%s): %v", dir, err)
	}
	t.Logf("heroes %d, counters %d with %d proofs, synergies %d with %d proofs",
		len(ds.Heroes), len(ds.Counters), countProofs(ds.Counters, func(c counter.Counter) int { return len(c.Proof) }),
		len(ds.Synergies), countProofs(ds.Synergies, func(s synergy.Synergy) int { return len(s.Proof) }))

	targets := map[string]bool{}
	for _, c := range ds.Counters {
		targets[c.TargetHeroID] = true
	}
	anchors := map[string]bool{}
	for _, s := range ds.Synergies {
		anchors[s.AnchorHeroID] = true
	}
	for sub, firsts := range map[string]map[string]bool{"counters": targets, "synergies": anchors} {
		paths, err := filepath.Glob(filepath.Join(dir, sub, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 0 {
			t.Fatalf("no files in %s/%s", dir, sub)
		}
		for _, p := range paths {
			if stem := strings.TrimSuffix(filepath.Base(p), ".json"); !firsts[stem] {
				t.Errorf("%s/%s.json loaded no rows for %q", sub, stem, stem)
			}
		}
		if len(firsts) != len(paths) {
			t.Errorf("%s: %d heroes with rows, %d files", sub, len(firsts), len(paths))
		}
	}
}

func countProofs[T any](rows []T, n func(T) int) int {
	total := 0
	for _, r := range rows {
		total += n(r)
	}
	return total
}
