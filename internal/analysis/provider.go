package analysis

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/yeremi777/mlbb-collector/internal/ai"
	"github.com/yeremi777/mlbb-collector/internal/config"
)

// NewFromConfig builds an Analyzer on the providers cfg names, in order,
// skipping with a warning a chat provider without its API key or model. It
// returns nil when no provider is usable.
func NewFromConfig(cfg config.AI) *Analyzer {
	var providers []ai.Provider
	for _, name := range cfg.Providers {
		if name == "mock" {
			providers = append(providers, Mock{})
			continue
		}
		chat := cfg.OpenRouter
		if name == "opencode_zen" {
			chat = cfg.OpenCodeZen
		}
		p, err := ai.NewChat(chat)
		if err != nil {
			slog.Warn("skipping AI provider", "provider", name, "reason", err)
			continue
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		return nil
	}
	return New(ai.Chain(providers...), Config{Timeout: cfg.Timeout, CacheTTL: cfg.CacheTTL, CacheMaxEntries: cfg.CacheMaxEntries})
}

// Mock answers without a network call, deriving a valid answer from the
// Counters or Synergies in the request: a Score of 60 plus 10 per Proof and a
// Confidence of 50 plus 10 per Reason, each at most 95, and a detail built
// from the Reasons and Proof.
type Mock struct{}

func (Mock) Name() string { return "Mock" }

type mockMatchup struct {
	CounterHeroID string   `json:"counterHeroId"`
	SynergyHeroID string   `json:"synergyHeroId"`
	Reasons       []string `json:"reasons"`
	Proof         []struct {
		ID            string   `json:"id"`
		WorksBestWhen []string `json:"worksBestWhen"`
		FailureCases  []string `json:"failureCases"`
	} `json:"proof"`
}

func (m mockMatchup) score() int      { return min(95, 60+10*len(m.Proof)) }
func (m mockMatchup) confidence() int { return min(95, 50+10*len(m.Reasons)) }

type mockScore struct {
	CounterHeroID string `json:"counterHeroId,omitempty"`
	SynergyHeroID string `json:"synergyHeroId,omitempty"`
	Score         int    `json:"score"`
	Confidence    int    `json:"confidence"`
}

func (Mock) CompleteJSON(_ context.Context, messages []ai.Message) (map[string]any, error) {
	// messages[1] is the user message of every prompt, the repair included.
	const marker = "Dataset context:\n"
	if len(messages) < 2 || !strings.Contains(messages[1].Content, marker) {
		return nil, &ai.Error{Message: "Mock found no dataset context."}
	}
	var payload struct {
		Matchups  []mockMatchup `json:"matchups"`
		Synergies []mockMatchup `json:"synergies"`
		Matchup   *mockMatchup  `json:"matchup"`
		Synergy   *mockMatchup  `json:"synergy"`
	}
	_, dataset, _ := strings.Cut(messages[1].Content, marker)
	if err := json.Unmarshal([]byte(dataset), &payload); err != nil {
		return nil, &ai.Error{Message: "Mock could not read the dataset context: " + err.Error()}
	}

	var answer any
	switch {
	case payload.Matchups != nil || payload.Synergies != nil:
		recs := make([]mockScore, 0, len(payload.Matchups)+len(payload.Synergies))
		for _, m := range payload.Matchups {
			recs = append(recs, mockScore{CounterHeroID: m.CounterHeroID, Score: m.score(), Confidence: m.confidence()})
		}
		for _, m := range payload.Synergies {
			recs = append(recs, mockScore{SynergyHeroID: m.SynergyHeroID, Score: m.score(), Confidence: m.confidence()})
		}
		answer = map[string]any{"recommendations": recs}
	case payload.Matchup != nil || payload.Synergy != nil:
		m := payload.Matchup
		if m == nil {
			m = payload.Synergy
		}
		d := detailBody{Score: m.score(), Confidence: m.confidence(), Strengths: m.Reasons,
			Conditions: []string{}, FailureCases: []string{}, EvidenceIDs: []string{}}
		if len(m.Reasons) > 0 {
			d.Summary = m.Reasons[0]
		}
		for _, p := range m.Proof {
			d.Conditions = append(d.Conditions, p.WorksBestWhen...)
			d.FailureCases = append(d.FailureCases, p.FailureCases...)
			d.EvidenceIDs = append(d.EvidenceIDs, p.ID)
		}
		answer = d
	default:
		return nil, &ai.Error{Message: "Mock found no Counter or Synergy in the dataset context."}
	}

	// Round-trip through JSON so the object has the types a provider's has.
	raw, err := json.Marshal(answer)
	if err != nil {
		return nil, &ai.Error{Message: "Mock could not encode its answer: " + err.Error()}
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, &ai.Error{Message: "Mock could not decode its answer: " + err.Error()}
	}
	return object, nil
}
