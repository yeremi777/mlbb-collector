//go:build integration

package seed

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/database/dbtest"
	"github.com/yeremi777/mlbb-collector/internal/dataset"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

func fixture() dataset.Dataset {
	return dataset.Dataset{
		Heroes: []hero.Hero{
			{UID: "tigreal", MLID: 6, Name: "Tigreal", Roles: []string{"tank"}, Lanes: []string{"roam"},
				Images: json.RawMessage(`{"head": "https://example.test/tigreal.png"}`)},
			{UID: "diggie", MLID: 48, Name: "Diggie", Roles: []string{"support"}, Lanes: []string{"roam"}, Images: json.RawMessage(`{}`)},
			{UID: "pharsa", MLID: 70, Name: "Pharsa", Roles: []string{"mage"}, Lanes: []string{"mid", "gold"}, Images: json.RawMessage(`{}`)},
		},
		Counters: []counter.Counter{
			{TargetHeroID: "tigreal", CounterHeroID: "diggie", Reasons: []string{"Diggie answers the engage."}, CounterTypes: []string{"anti-cc"},
				Proof: []counter.Proof{{ID: "diggie-vs-tigreal", Category: "crowd-control-counter", Priority: "primary", Impact: "high",
					Summary: "Cleanses the engage.", WorksBestWhen: []string{"Ultimate held."}, FailureCases: []string{"Ultimate baited."}}}},
			{TargetHeroID: "tigreal", CounterHeroID: "pharsa", Reasons: []string{"Pharsa outranges Tigreal."}, CounterTypes: []string{"poke"},
				Proof: []counter.Proof{{ID: "pharsa-vs-tigreal", Category: "range-advantage", Priority: "secondary", Impact: "medium",
					Summary: "Hits from beyond engage range.", WorksBestWhen: []string{"Distance kept."}, FailureCases: []string{"Flanked."}}}},
		},
		Synergies: []synergy.Synergy{
			{AnchorHeroID: "tigreal", SynergyHeroID: "pharsa", Reasons: []string{"Pharsa follows the engage."}, SynergyTypes: []string{"cc-chain"},
				Proof: []synergy.Proof{{ID: "tigreal-with-pharsa", Category: "engage-follow-up", Priority: "primary", Impact: "high",
					Summary: "Ultimate lands on grouped enemies.", WorksBestWhen: []string{"Pharsa is close."}, FailureCases: []string{"Enemies spread."}}}},
		},
	}
}

// rows returns every row of query as one string per row, so a table's state
// compares as a plain slice.
func rows(t *testing.T, ctx context.Context, tx pgx.Tx, query string) []string {
	t.Helper()
	r, err := tx.Query(ctx, query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	got, err := pgx.CollectRows(r, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return got
}

const (
	heroRows        = `SELECT concat_ws('|', uid, mlid, name, roles, lanes, images) FROM heroes ORDER BY mlid`
	counterRows     = `SELECT concat_ws('|', target_hero_id, counter_hero_id, reasons, counter_types) FROM counters ORDER BY 1`
	counterProofRow = `SELECT concat_ws('|', id, target_hero_id, counter_hero_id, category, priority, impact, summary, works_best_when, failure_cases) FROM counter_proofs ORDER BY id`
	synergyRows     = `SELECT concat_ws('|', anchor_hero_id, synergy_hero_id, reasons, synergy_types) FROM synergies ORDER BY 1`
	synergyProofRow = `SELECT concat_ws('|', id, anchor_hero_id, synergy_hero_id, category, priority, impact, summary, works_best_when, failure_cases) FROM synergy_proofs ORDER BY id`
	// ctids changes for a row whenever the row is rewritten, even inside the
	// transaction that rewrote it.
	ctids = `SELECT t || ':' || ctid FROM (
	           SELECT 'heroes' t, ctid FROM heroes UNION ALL
	           SELECT 'counters', ctid FROM counters UNION ALL
	           SELECT 'counter_proofs', ctid FROM counter_proofs UNION ALL
	           SELECT 'synergies', ctid FROM synergies UNION ALL
	           SELECT 'synergy_proofs', ctid FROM synergy_proofs) s ORDER BY 1`
)

func TestSyncWritesTheDataset(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	want := map[string][]string{
		heroRows: {
			`tigreal|6|Tigreal|{tank}|{roam}|{"head": "https://example.test/tigreal.png"}`,
			`diggie|48|Diggie|{support}|{roam}|{}`,
			`pharsa|70|Pharsa|{mage}|{mid,gold}|{}`,
		},
		counterRows: {
			`tigreal|diggie|{"Diggie answers the engage."}|{anti-cc}`,
			`tigreal|pharsa|{"Pharsa outranges Tigreal."}|{poke}`,
		},
		counterProofRow: {
			`diggie-vs-tigreal|tigreal|diggie|crowd-control-counter|primary|high|Cleanses the engage.|{"Ultimate held."}|{"Ultimate baited."}`,
			`pharsa-vs-tigreal|tigreal|pharsa|range-advantage|secondary|medium|Hits from beyond engage range.|{"Distance kept."}|{Flanked.}`,
		},
		synergyRows: {
			`tigreal|pharsa|{"Pharsa follows the engage."}|{cc-chain}`,
		},
		synergyProofRow: {
			`tigreal-with-pharsa|tigreal|pharsa|engage-follow-up|primary|high|Ultimate lands on grouped enemies.|{"Pharsa is close."}|{"Enemies spread."}`,
		},
	}
	for query, w := range want {
		if got := rows(t, ctx, tx, query); !reflect.DeepEqual(got, w) {
			t.Errorf("%s\n got %q\nwant %q", query, got, w)
		}
	}
}

func TestSyncRewritesOnlyChangedRows(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	before := rows(t, ctx, tx, ctids)

	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if after := rows(t, ctx, tx, ctids); !reflect.DeepEqual(after, before) {
		t.Errorf("an unchanged seed rewrote rows:\nbefore %q\n after %q", before, after)
	}

	tigrealBefore := rows(t, ctx, tx, `SELECT ctid::text FROM heroes WHERE uid = 'tigreal'`)
	diggieBefore := rows(t, ctx, tx, `SELECT ctid::text FROM heroes WHERE uid = 'diggie'`)
	edited := fixture()
	edited.Heroes[1].Name = "Diggie the Owl"
	if err := Sync(ctx, tx, edited); err != nil {
		t.Fatalf("edited Sync: %v", err)
	}
	if got := rows(t, ctx, tx, `SELECT name FROM heroes WHERE uid = 'diggie'`); !reflect.DeepEqual(got, []string{"Diggie the Owl"}) {
		t.Errorf("diggie name = %q, want the edited name", got)
	}
	if got := rows(t, ctx, tx, `SELECT ctid::text FROM heroes WHERE uid = 'diggie'`); reflect.DeepEqual(got, diggieBefore) {
		t.Error("edited diggie was not rewritten")
	}
	if got := rows(t, ctx, tx, `SELECT ctid::text FROM heroes WHERE uid = 'tigreal'`); !reflect.DeepEqual(got, tigrealBefore) {
		t.Error("unedited tigreal was rewritten")
	}
}

func TestSyncDeletesWhatTheFilesNoLongerHold(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	withoutPharsaCounter := fixture()
	withoutPharsaCounter.Counters = withoutPharsaCounter.Counters[:1]
	if err := Sync(ctx, tx, withoutPharsaCounter); err != nil {
		t.Fatalf("Sync without a counter: %v", err)
	}
	if got := rows(t, ctx, tx, `SELECT counter_hero_id FROM counters`); !reflect.DeepEqual(got, []string{"diggie"}) {
		t.Errorf("counters = %q, want only diggie", got)
	}
	if got := rows(t, ctx, tx, `SELECT id FROM counter_proofs`); !reflect.DeepEqual(got, []string{"diggie-vs-tigreal"}) {
		t.Errorf("counter proofs = %q, want only diggie's", got)
	}

	withoutDiggie := fixture()
	withoutDiggie.Heroes = []hero.Hero{withoutDiggie.Heroes[0], withoutDiggie.Heroes[2]}
	withoutDiggie.Counters = withoutDiggie.Counters[1:]
	if err := Sync(ctx, tx, withoutDiggie); err != nil {
		t.Fatalf("Sync without a hero: %v", err)
	}
	if got := rows(t, ctx, tx, `SELECT uid FROM heroes ORDER BY mlid`); !reflect.DeepEqual(got, []string{"tigreal", "pharsa"}) {
		t.Errorf("heroes = %q, want tigreal and pharsa", got)
	}
	if got := rows(t, ctx, tx, `SELECT counter_hero_id FROM counters`); !reflect.DeepEqual(got, []string{"pharsa"}) {
		t.Errorf("counters = %q, want only pharsa", got)
	}
}

func TestSyncEmptiesAKindWithNoRows(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	noSynergies := fixture()
	noSynergies.Synergies = nil
	if err := Sync(ctx, tx, noSynergies); err != nil {
		t.Fatalf("Sync without synergies: %v", err)
	}
	got := rows(t, ctx, tx, `SELECT 'synergies ' || count(*) FROM synergies UNION ALL SELECT 'synergy_proofs ' || count(*) FROM synergy_proofs`)
	if want := []string{"synergies 0", "synergy_proofs 0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := rows(t, ctx, tx, `SELECT 'counters ' || count(*) FROM counters`); !reflect.DeepEqual(got, []string{"counters 2"}) {
		t.Errorf("got %q, want counters kept", got)
	}
}

func TestSyncReportsADatabaseFailure(t *testing.T) {
	ctx, tx := dbtest.BeginTx(t)
	if err := Sync(ctx, tx, fixture()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	before := rows(t, ctx, tx, ctids)

	savepoint, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	broken := fixture()
	broken.Counters[1].CounterHeroID = "nana"
	if err := Sync(ctx, savepoint, broken); err == nil {
		t.Fatal("Sync with a counter naming an unknown hero succeeded")
	}
	if err := savepoint.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if after := rows(t, ctx, tx, ctids); !reflect.DeepEqual(after, before) {
		t.Errorf("a failed seed changed the tables:\nbefore %q\n after %q", before, after)
	}
}
