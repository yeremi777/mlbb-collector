//go:build integration

package counter_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/seed"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func proof(id, category string) counter.Proof {
	return counter.Proof{ID: id, Category: category, Priority: "primary", Impact: "high",
		Summary: "s " + id, WorksBestWhen: []string{"w " + id}, FailureCases: []string{"f " + id}}
}

func TestRepositoryForTarget(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	h := func(uid string, mlid int) hero.Hero {
		return hero.Hero{UID: uid, MLID: mlid, Name: uid, Roles: []string{"tank"}, Lanes: []string{}, Images: json.RawMessage(`{}`)}
	}
	tigreal, pharsa, diggie := h("tigreal", 6), h("pharsa", 70), h("diggie", 48)
	ds := dataset.Dataset{
		Heroes: []hero.Hero{tigreal, pharsa, diggie},
		Counters: []counter.Counter{
			{TargetHeroID: "tigreal", CounterHeroID: "pharsa", Reasons: []string{"r1"}, CounterTypes: []string{"poke"},
				Proof: []counter.Proof{proof("z-proof", "range-advantage"), proof("a-proof", "kiting")}},
			{TargetHeroID: "tigreal", CounterHeroID: "diggie", Reasons: []string{"r2"}, CounterTypes: []string{"anti-cc"},
				Proof: []counter.Proof{proof("m-proof", "crowd-control-counter")}},
			{TargetHeroID: "pharsa", CounterHeroID: "tigreal", Reasons: []string{"r3"}, CounterTypes: []string{"dive"},
				Proof: []counter.Proof{proof("other-proof", "game-phase")}},
		},
	}
	if err := seed.Sync(ctx, tx, ds); err != nil {
		t.Fatal(err)
	}
	repo := counter.NewRepository(tx)

	got, err := repo.ForTarget(ctx, "tigreal")
	if err != nil {
		t.Fatal(err)
	}
	want := []counter.WithHero{
		{TargetHeroID: "tigreal", CounterHero: diggie, Reasons: []string{"r2"}, CounterTypes: []string{"anti-cc"},
			Proof: []counter.Proof{proof("m-proof", "crowd-control-counter")}},
		{TargetHeroID: "tigreal", CounterHero: pharsa, Reasons: []string{"r1"}, CounterTypes: []string{"poke"},
			Proof: []counter.Proof{proof("a-proof", "kiting"), proof("z-proof", "range-advantage")}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForTarget(tigreal):\n got %+v\nwant %+v", got, want)
	}

	none, err := repo.ForTarget(ctx, "diggie")
	if err != nil || len(none) != 0 {
		t.Errorf("ForTarget(diggie) = %+v, %v; want no counters", none, err)
	}
}
