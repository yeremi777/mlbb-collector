// Package synergy is the authored claim that two heroes are stronger together,
// backed by Reasons and Proof.
package synergy

import (
	"fmt"
	"slices"
	"strings"
)

// Synergy is one authored Synergy: SynergyHeroID makes AnchorHeroID stronger.
type Synergy struct {
	AnchorHeroID  string   `json:"anchorHeroId"`
	SynergyHeroID string   `json:"synergyHeroId"`
	Reasons       []string `json:"reasons"`
	SynergyTypes  []string `json:"synergyTypes"`
	Proof         []Proof  `json:"proof"`
}

// Proof is one concrete interaction supporting a Synergy.
type Proof struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Priority      string   `json:"priority"`
	Impact        string   `json:"impact"`
	Summary       string   `json:"summary"`
	WorksBestWhen []string `json:"worksBestWhen"`
	FailureCases  []string `json:"failureCases"`
}

var (
	categories = []string{
		"skill-interaction", "crowd-control-chain", "engage-follow-up", "setup-combo",
		"damage-amplification", "protection", "peel", "frontline-enabler", "mobility-enabler",
		"vision-setup", "healing-sustain", "shielding", "poke-siege", "pickoff-combo",
		"teamfight-combo", "objective-control", "laning-synergy", "game-phase",
		"positioning-requirement", "cooldown-window", "execution-difficulty",
	}
	priorities = []string{"primary", "secondary", "condition"}
	impacts    = []string{"high", "medium", "low"}
)

// Validate reports the first rule the Synergy breaks, prefixed with the key
// that breaks it.
func (s Synergy) Validate() error {
	if s.SynergyHeroID == s.AnchorHeroID {
		return fmt.Errorf("synergyHeroId: %q is the anchor hero", s.SynergyHeroID)
	}
	if err := nonBlankList("reasons", s.Reasons); err != nil {
		return err
	}
	if err := nonBlankList("synergyTypes", s.SynergyTypes); err != nil {
		return err
	}
	if len(s.Proof) == 0 {
		return fmt.Errorf("proof: empty")
	}
	for i, p := range s.Proof {
		if err := p.validate(); err != nil {
			return fmt.Errorf("proof[%d].%w", i, err)
		}
	}
	return nil
}

func (p Proof) validate() error {
	switch {
	case blank(p.ID):
		return fmt.Errorf("id: blank")
	case !slices.Contains(categories, p.Category):
		return fmt.Errorf("category: %q is not a synergy proof category", p.Category)
	case !slices.Contains(priorities, p.Priority):
		return fmt.Errorf("priority: %q is not a priority", p.Priority)
	case !slices.Contains(impacts, p.Impact):
		return fmt.Errorf("impact: %q is not an impact", p.Impact)
	case blank(p.Summary):
		return fmt.Errorf("summary: blank")
	}
	if err := nonBlankList("worksBestWhen", p.WorksBestWhen); err != nil {
		return err
	}
	return nonBlankList("failureCases", p.FailureCases)
}

// nonBlankList requires at least one value and no blank value.
func nonBlankList(key string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s: empty", key)
	}
	for i, v := range values {
		if blank(v) {
			return fmt.Errorf("%s[%d]: blank", key, i)
		}
	}
	return nil
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }
