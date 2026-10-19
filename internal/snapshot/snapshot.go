// Package snapshot fetches Moonton's daily hero statistics and appends them to
// raw.hero_rank_snapshots (ADR-0004).
package snapshot

import "encoding/json"

// Combination is one Window and Rank tier, fetched as one Snapshot.
type Combination struct {
	Window   int
	RankTier string
}

// Each Window has its own Moonton endpoint, and each Rank tier its bigrank filter.
var (
	windows = []struct{ days, endpoint int }{
		{1, 2756567}, {3, 2756568}, {7, 2756569}, {15, 2756565}, {30, 2756570},
	}
	rankTiers = []struct{ name, bigrank string }{
		{"all", "101"}, {"epic", "5"}, {"legend", "6"}, {"mythic", "7"}, {"honor", "8"}, {"glory", "9"},
	}
)

// Combinations returns the 30 combinations a run covers: Windows ascending,
// then Rank tiers from all to glory.
func Combinations() []Combination {
	out := make([]Combination, 0, len(windows)*len(rankTiers))
	for _, w := range windows {
		for _, tier := range rankTiers {
			out = append(out, Combination{Window: w.days, RankTier: tier.name})
		}
	}
	return out
}

// upstreamKeys returns combo's endpoint and bigrank; ok is false for a
// Window or Rank tier Moonton does not serve.
func upstreamKeys(combo Combination) (endpoint int, bigrank string, ok bool) {
	for _, w := range windows {
		if w.days == combo.Window {
			endpoint = w.endpoint
		}
	}
	for _, tier := range rankTiers {
		if tier.name == combo.RankTier {
			bigrank = tier.bigrank
		}
	}
	return endpoint, bigrank, endpoint != 0 && bigrank != ""
}

// Record is one hero's statistics in a Snapshot.
type Record struct {
	MoontonID       int
	WinRate         float64
	AppearanceShare float64
	BanRate         float64
	// Payload is the record exactly as received.
	Payload json.RawMessage
}
