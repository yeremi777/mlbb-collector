// Package analysis asks an AI provider for the Score and Confidence of every
// Counter or Synergy of a hero, or a written explanation of one, from the
// authored Reasons and Proof only. Results live in an in-memory cache and are
// never stored.
package analysis

import "github.com/yeremi777/mlbb-collector/internal/hero"

// Matchup is one Counter or Synergy as analysis reads it: Partner is the
// Counter hero or the Synergy hero, and Types its Counter or Synergy types.
type Matchup struct {
	Partner hero.Hero
	Reasons []string
	Types   []string
	Proof   []Proof
}

// Proof is one Proof of a Counter or Synergy.
type Proof struct {
	ID            string
	Category      string
	Priority      string
	Impact        string
	Summary       string
	WorksBestWhen []string
	FailureCases  []string
}
