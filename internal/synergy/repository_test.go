//go:build integration

package synergy_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/seed"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

func proof(id, category string) synergy.Proof {
	return synergy.Proof{ID: id, Category: category, Priority: "primary", Impact: "high",
		Summary: "s " + id, WorksBestWhen: []string{"w " + id}, FailureCases: []string{"f " + id}}
}

func TestRepositoryForAnchor(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	h := func(uid string, mlid int) hero.Hero {
		return hero.Hero{UID: uid, MLID: mlid, Name: uid, Roles: []string{"tank"}, Lanes: []string{}, Images: json.RawMessage(`{}`)}
	}
	tigreal, pharsa, diggie := h("tigreal", 6), h("pharsa", 70), h("diggie", 48)
	ds := dataset.Dataset{
		Heroes: []hero.Hero{tigreal, pharsa, diggie},
		Synergies: []synergy.Synergy{
			{AnchorHeroID: "tigreal", SynergyHeroID: "pharsa", Reasons: []string{"r1"}, SynergyTypes: []string{"poke"},
				Proof: []synergy.Proof{proof("z-proof", "engage-follow-up"), proof("a-proof", "protection")}},
			{AnchorHeroID: "tigreal", SynergyHeroID: "diggie", Reasons: []string{"r2"}, SynergyTypes: []string{"anti-cc"},
				Proof: []synergy.Proof{proof("m-proof", "crowd-control-chain")}},
			{AnchorHeroID: "pharsa", SynergyHeroID: "tigreal", Reasons: []string{"r3"}, SynergyTypes: []string{"dive"},
				Proof: []synergy.Proof{proof("other-proof", "game-phase")}},
		},
	}
	if err := seed.Sync(ctx, tx, ds); err != nil {
		t.Fatal(err)
	}
	repo := synergy.NewRepository(tx)

	got, err := repo.ForAnchor(ctx, "tigreal")
	if err != nil {
		t.Fatal(err)
	}
	want := []synergy.WithHero{
		{AnchorHeroID: "tigreal", SynergyHero: diggie, Reasons: []string{"r2"}, SynergyTypes: []string{"anti-cc"},
			Proof: []synergy.Proof{proof("m-proof", "crowd-control-chain")}},
		{AnchorHeroID: "tigreal", SynergyHero: pharsa, Reasons: []string{"r1"}, SynergyTypes: []string{"poke"},
			Proof: []synergy.Proof{proof("a-proof", "protection"), proof("z-proof", "engage-follow-up")}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ForAnchor(tigreal):\n got %+v\nwant %+v", got, want)
	}

	none, err := repo.ForAnchor(ctx, "diggie")
	if err != nil || len(none) != 0 {
		t.Errorf("ForAnchor(diggie) = %+v, %v; want no synergies", none, err)
	}
}
