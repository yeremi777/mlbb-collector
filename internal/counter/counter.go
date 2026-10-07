// Package counter is the authored claim that one hero beats another, backed by
// Reasons and Proof.
package counter

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Counter is one authored Counter: CounterHeroID beats TargetHeroID.
type Counter struct {
	TargetHeroID  string   `json:"targetHeroId"`
	CounterHeroID string   `json:"counterHeroId"`
	Reasons       []string `json:"reasons"`
	CounterTypes  []string `json:"counterTypes"`
	Proof         []Proof  `json:"proof"`
}

// WithHero is a Counter as the API serves it, with the Counter hero in full.
type WithHero struct {
	TargetHeroID string    `json:"targetHeroId"`
	CounterHero  hero.Hero `json:"counterHero"`
	Reasons      []string  `json:"reasons"`
	CounterTypes []string  `json:"counterTypes"`
	Proof        []Proof   `json:"proof"`
}

// Proof is one concrete interaction supporting a Counter.
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
		"skill-interaction", "crowd-control-counter", "damage-type-advantage", "item-power-spike",
		"mobility-advantage", "range-advantage", "kiting", "sustain-anti-sustain",
		"positioning-requirement", "cooldown-window", "vision-awareness", "teamfight-role-counter",
		"game-phase", "execution-difficulty",
	}
	priorities = []string{"primary", "secondary", "condition"}
	impacts    = []string{"high", "medium", "low"}
)

// Validate reports the first rule the Counter breaks, prefixed with the key
// that breaks it.
func (c Counter) Validate() error {
	if c.CounterHeroID == c.TargetHeroID {
		return fmt.Errorf("counterHeroId: %q is the target hero", c.CounterHeroID)
	}
	if err := nonBlankList("reasons", c.Reasons); err != nil {
		return err
	}
	if err := nonBlankList("counterTypes", c.CounterTypes); err != nil {
		return err
	}
	if len(c.Proof) == 0 {
		return fmt.Errorf("proof: empty")
	}
	for i, p := range c.Proof {
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
		return fmt.Errorf("category: %q is not a counter proof category", p.Category)
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
