package analysis

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/ai"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// Error is an analysis failure the routes answer with; Code is the API error
// code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func providerFailure(format string, args ...any) *Error {
	return &Error{Code: "ai_provider_error", Message: fmt.Sprintf(format, args...)}
}

// Config tunes an Analyzer. Timeout bounds all provider work of one request:
// every provider asked and the detail repair.
type Config struct {
	Timeout time.Duration
}

// Analyzer scores and explains Counters and Synergies by asking provider.
type Analyzer struct {
	provider ai.Provider
	timeout  time.Duration
}

// New asks provider.
func New(provider ai.Provider, cfg Config) *Analyzer {
	return &Analyzer{provider: provider, timeout: cfg.Timeout}
}

// Ranked is one Counter hero or Synergy hero with its Score and Confidence,
// numbered from 1 in rank order.
type Ranked struct {
	Rank       int
	HeroID     string
	Score      int
	Confidence int
}

// Detail is the written explanation of one Counter or Synergy.
type Detail struct {
	Score        int
	Confidence   int
	Summary      string
	Strengths    []string
	Conditions   []string
	FailureCases []string
	EvidenceIDs  []string
}

// ScoreCounters scores every Counter of the target hero in one request.
func (a *Analyzer) ScoreCounters(ctx context.Context, target hero.Hero, ms []Matchup, language string) ([]Ranked, error) {
	return a.score(ctx, counterScoringMessages(target, ms, language), "counterHeroId", ms)
}

// ScoreSynergies scores every Synergy of the anchor hero in one request.
func (a *Analyzer) ScoreSynergies(ctx context.Context, anchor hero.Hero, ms []Matchup, language string) ([]Ranked, error) {
	return a.score(ctx, synergyScoringMessages(anchor, ms, language), "synergyHeroId", ms)
}

// CounterDetail explains one Counter of the target hero.
func (a *Analyzer) CounterDetail(ctx context.Context, target hero.Hero, m Matchup, language string) (Detail, error) {
	return a.detail(ctx, counterDetailMessages(target, m, language), m, language)
}

// SynergyDetail explains one Synergy of the anchor hero.
func (a *Analyzer) SynergyDetail(ctx context.Context, anchor hero.Hero, m Matchup, language string) (Detail, error) {
	return a.detail(ctx, synergyDetailMessages(anchor, m, language), m, language)
}

// complete asks the provider, answering ai_provider_timeout once ctx's
// deadline has passed.
func (a *Analyzer) complete(ctx context.Context, messages []ai.Message) (map[string]any, error) {
	payload, err := a.provider.CompleteJSON(ctx, messages)
	if err == nil {
		return payload, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, &Error{Code: "ai_provider_timeout", Message: "AI provider did not answer in time."}
	}
	var pe *ai.Error
	if errors.As(err, &pe) {
		return nil, providerFailure("%s", pe.Message)
	}
	return nil, providerFailure("%v", err)
}

func (a *Analyzer) score(ctx context.Context, messages []ai.Message, idKey string, ms []Matchup) ([]Ranked, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	payload, err := a.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	expected := make([]string, len(ms))
	for i, m := range ms {
		expected[i] = m.Partner.UID
	}
	ranked, err := validateScoring(payload, idKey, expected)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(ranked, func(x, y Ranked) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(y.Confidence, x.Confidence), cmp.Compare(x.HeroID, y.HeroID))
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return ranked, nil
}

// validateScoring accepts recommendations holding each expected hero ID
// exactly once, each with an integer score and confidence from 0 to 100.
func validateScoring(payload map[string]any, idKey string, expected []string) ([]Ranked, error) {
	raw, ok := payload["recommendations"].([]any)
	if !ok {
		return nil, providerFailure("Invalid scoring response from model: recommendations must be an array")
	}
	ranked := make([]Ranked, 0, len(raw))
	seen := map[string]bool{}
	for _, entry := range raw {
		obj, ok := entry.(map[string]any)
		if !ok {
			return nil, providerFailure("Invalid scoring response from model: recommendation must be an object")
		}
		id, _ := obj[idKey].(string)
		score, scoreOK := obj["score"].(float64)
		confidence, confidenceOK := obj["confidence"].(float64)
		if id == "" || !scoreOK || !isRating(score) || !confidenceOK || !isRating(confidence) {
			return nil, providerFailure("Invalid scoring response from model: missing %s, score, or confidence", idKey)
		}
		seen[id] = true
		ranked = append(ranked, Ranked{HeroID: id, Score: int(score), Confidence: int(confidence)})
	}
	if len(ranked) != len(expected) || len(seen) != len(expected) {
		return nil, providerFailure("Scoring response must include every expected %s once.", idKey)
	}
	for _, id := range expected {
		if !seen[id] {
			return nil, providerFailure("Scoring response must include every expected %s once.", idKey)
		}
	}
	return ranked, nil
}

// isRating reports whether f is an integer from 0 to 100.
func isRating(f float64) bool { return f >= 0 && f <= 100 && f == math.Trunc(f) }

// detail asks for the explanation, and sends an invalid answer back once with
// a repair instruction unless it cites an unknown Proof ID.
func (a *Analyzer) detail(ctx context.Context, messages []ai.Message, m Matchup, language string) (Detail, error) {
	proofIDs := make([]string, len(m.Proof))
	for i, p := range m.Proof {
		proofIDs[i] = p.ID
	}
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	payload, err := a.complete(ctx, messages)
	if err != nil {
		return Detail{}, err
	}
	d, err := validateDetail(payload, proofIDs)
	if err == nil {
		return d, nil
	}
	if _, fatal := err.(*Error); fatal {
		return Detail{}, err
	}
	payload, err = a.complete(ctx, detailRepairMessages(messages, payload, err, language))
	if err != nil {
		return Detail{}, err
	}
	d, err = validateDetail(payload, proofIDs)
	if err == nil {
		return d, nil
	}
	if ae, ok := err.(*Error); ok {
		return Detail{}, ae
	}
	return Detail{}, providerFailure("Invalid detail response from model after retry: %v", err)
}

// validateDetail returns an *Error for an answer citing an unknown Proof ID,
// and a plain error, sent to the model in the repair, for any other invalid
// answer.
func validateDetail(payload map[string]any, proofIDs []string) (Detail, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Detail{}, providerFailure("%v", err)
	}
	// Anonymous, so a type error names the field as dev's repair message does.
	var answer struct {
		Score        *float64 `json:"score"`
		Confidence   *float64 `json:"confidence"`
		Summary      string   `json:"summary"`
		Strengths    []string `json:"strengths"`
		Conditions   []string `json:"conditions"`
		FailureCases []string `json:"failureCases"`
		EvidenceIDs  []string `json:"evidenceIds"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return Detail{}, fmt.Errorf("detail payload has wrong field types: %w", err)
	}
	if answer.Score == nil || !isRating(*answer.Score) || answer.Confidence == nil || !isRating(*answer.Confidence) ||
		strings.TrimSpace(answer.Summary) == "" || len(answer.Strengths) == 0 {
		return Detail{}, errors.New("detail payload missing required fields: score, confidence, non-empty summary, non-empty strengths")
	}
	for _, id := range answer.EvidenceIDs {
		if !slices.Contains(proofIDs, id) {
			return Detail{}, providerFailure("Detail response referenced unknown evidence ids.")
		}
	}
	return Detail{
		Score: int(*answer.Score), Confidence: int(*answer.Confidence), Summary: answer.Summary,
		Strengths: answer.Strengths, Conditions: orEmpty(answer.Conditions),
		FailureCases: orEmpty(answer.FailureCases), EvidenceIDs: orEmpty(answer.EvidenceIDs),
	}, nil
}
