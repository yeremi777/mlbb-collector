//go:build integration

package hero_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/seed"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestRepository(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	heroes := []hero.Hero{
		{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"},
			Images: json.RawMessage(`{"head": "https://example.test/t.png"}`)},
		{UID: "miya", MLID: 1, Name: "Miya", Roles: []string{"marksman"}, Lanes: []string{"gold"}, Images: json.RawMessage(`{}`)},
	}
	if err := seed.Sync(ctx, tx, dataset.Dataset{Heroes: heroes}); err != nil {
		t.Fatal(err)
	}
	repo := hero.NewRepository(tx)

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []hero.Hero{heroes[1], heroes[0]}; !reflect.DeepEqual(list, want) {
		t.Errorf("List:\n got %+v\nwant %+v (ordered by Moonton ID)", list, want)
	}

	got, err := repo.Get(ctx, "tigreal")
	if err != nil || !reflect.DeepEqual(got, heroes[0]) {
		t.Errorf("Get(tigreal) = %+v, %v", got, err)
	}
	if _, err := repo.Get(ctx, "nope"); !errors.Is(err, hero.ErrNotFound) {
		t.Errorf("Get(nope) err = %v, want ErrNotFound", err)
	}
}
