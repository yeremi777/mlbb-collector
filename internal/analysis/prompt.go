package analysis

import (
	"bytes"
	"encoding/json"

	"github.com/yeremi777/mlbb-collector/internal/ai"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

const counterScoringInstruction = `You are scoring Mobile Legends hero counter recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each counter in the batch, return:
- counterHeroId: must match an id from the input
- score: 0-100 matchup strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: hard counter or very direct mechanic counter
- 85-94: strong and reliable counter
- 75-84: good counter with meaningful conditions
- 65-74: situational counter
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill interactions and direct crowd-control counters should score higher than generic damage or item-dependent counters.

Respond with JSON only in this shape:
{"recommendations":[{"counterHeroId":"...","score":0,"confidence":0}]}
Include every counterHeroId from the input exactly once.`

const counterDetailInstruction = `You are explaining one Mobile Legends hero counter matchup.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or matchup facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input.`

const synergyScoringInstruction = `You are scoring Mobile Legends hero synergy recommendations.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

For each synergy in the batch, return:
- synergyHeroId: must match an id from the input
- score: 0-100 synergy strength
- confidence: 0-100 evidence confidence

Score guidance:
- 95-100: defining, near-mandatory pairing
- 85-94: strong and reliable synergy
- 75-84: good synergy with meaningful conditions
- 65-74: situational synergy
- below 65: weak, incomplete, or too conditional

High context increases confidence, not necessarily score.
Direct skill-combo and setup-into-payoff synergies should score higher than generic or purely situational pairings.

Respond with JSON only in this shape:
{"recommendations":[{"synergyHeroId":"...","score":0,"confidence":0}]}
Include every synergyHeroId from the input exactly once.`

const synergyDetailInstruction = `You are explaining one Mobile Legends hero synergy pairing.

Use only the provided dataset context.
Do not invent hero skills, item requirements, patch facts, or synergy facts.
Do not use outside knowledge unless the caller explicitly includes it.

Return JSON only in this shape:
{
  "score": 0,
  "confidence": 0,
  "summary": "concise explanation",
  "strengths": ["concrete strengths from provided evidence"],
  "conditions": ["works-best conditions from provided evidence"],
  "failureCases": ["failure cases from provided evidence"],
  "evidenceIds": ["proof ids used"]
}

All keys are required. Use an empty array for optional arrays only when the dataset has no matching evidence.
strengths must be a non-empty array and must come from provided reasons and proof.
conditions must come from proof.worksBestWhen when available.
failureCases must come from proof.failureCases when available.
evidenceIds must only list proof ids present in the input.`

// languageInstructions name the language of the explanatory prose. Both name
// counterHeroId for Synergies too, as dev's do.
var languageInstructions = map[string]string{
	"en": "Write all user-visible explanation text (summary, strengths, conditions, failureCases) " +
		"in English. Keep every identifier exactly as given: counterHeroId, evidenceIds, and proof ids.",
	"id": "Write all user-visible explanation text (summary, strengths, conditions, failureCases) " +
		"in natural Indonesian (Bahasa Indonesia). " +
		"Do not translate or alter any identifier: keep counterHeroId, evidenceIds, and proof ids " +
		"exactly as given. Keep hero names as written. Only the explanatory prose should be Indonesian.",
}

type heroContext struct {
	UID   string   `json:"uid"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	Lanes []string `json:"lanes"`
}

func heroCtx(h hero.Hero) heroContext {
	return heroContext{UID: h.UID, Name: h.Name, Roles: h.Roles, Lanes: h.Lanes}
}

// proofContext is a Proof in a prompt. Scoring leaves out WorksBestWhen and
// FailureCases; detail includes them, empty or not.
type proofContext struct {
	ID            string    `json:"id"`
	Category      string    `json:"category"`
	Priority      string    `json:"priority"`
	Impact        string    `json:"impact"`
	Summary       string    `json:"summary"`
	WorksBestWhen *[]string `json:"worksBestWhen,omitempty"`
	FailureCases  *[]string `json:"failureCases,omitempty"`
}

func proofCtxs(proofs []Proof, withDetail bool) []proofContext {
	out := make([]proofContext, len(proofs))
	for i, p := range proofs {
		out[i] = proofContext{ID: p.ID, Category: p.Category, Priority: p.Priority, Impact: p.Impact, Summary: p.Summary}
		if withDetail {
			worksBestWhen, failureCases := orEmpty(p.WorksBestWhen), orEmpty(p.FailureCases)
			out[i].WorksBestWhen, out[i].FailureCases = &worksBestWhen, &failureCases
		}
	}
	return out
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// compactJSON encodes v with compact separators, no HTML escaping, and unicode
// kept as is.
func compactJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

func userMessage(language string, payload any) ai.Message {
	return ai.Message{Role: "user", Content: languageInstructions[language] + "\n\nDataset context:\n" + compactJSON(payload)}
}

type counterContext struct {
	CounterHeroID string         `json:"counterHeroId"`
	Reasons       []string       `json:"reasons"`
	CounterTypes  []string       `json:"counterTypes"`
	Proof         []proofContext `json:"proof"`
	CounterHero   heroContext    `json:"counterHero"`
}

func counterCtx(m Matchup, withDetail bool) counterContext {
	return counterContext{
		CounterHeroID: m.Partner.UID, Reasons: m.Reasons, CounterTypes: m.Types,
		Proof: proofCtxs(m.Proof, withDetail), CounterHero: heroCtx(m.Partner),
	}
}

func counterScoringMessages(target hero.Hero, ms []Matchup, language string) []ai.Message {
	ctxs := make([]counterContext, len(ms))
	for i, m := range ms {
		ctxs[i] = counterCtx(m, false)
	}
	payload := map[string]any{"targetHero": heroCtx(target), "matchups": ctxs, "outputLanguage": language}
	return []ai.Message{{Role: "system", Content: counterScoringInstruction}, userMessage(language, payload)}
}

func counterDetailMessages(target hero.Hero, m Matchup, language string) []ai.Message {
	payload := map[string]any{"targetHero": heroCtx(target), "matchup": counterCtx(m, true), "outputLanguage": language}
	return []ai.Message{{Role: "system", Content: counterDetailInstruction}, userMessage(language, payload)}
}

type synergyContext struct {
	SynergyHeroID string         `json:"synergyHeroId"`
	Reasons       []string       `json:"reasons"`
	SynergyTypes  []string       `json:"synergyTypes"`
	Proof         []proofContext `json:"proof"`
	SynergyHero   heroContext    `json:"synergyHero"`
}

func synergyCtx(m Matchup, withDetail bool) synergyContext {
	return synergyContext{
		SynergyHeroID: m.Partner.UID, Reasons: m.Reasons, SynergyTypes: m.Types,
		Proof: proofCtxs(m.Proof, withDetail), SynergyHero: heroCtx(m.Partner),
	}
}

func synergyScoringMessages(anchor hero.Hero, ms []Matchup, language string) []ai.Message {
	ctxs := make([]synergyContext, len(ms))
	for i, m := range ms {
		ctxs[i] = synergyCtx(m, false)
	}
	payload := map[string]any{"anchorHero": heroCtx(anchor), "synergies": ctxs, "outputLanguage": language}
	return []ai.Message{{Role: "system", Content: synergyScoringInstruction}, userMessage(language, payload)}
}

func synergyDetailMessages(anchor hero.Hero, m Matchup, language string) []ai.Message {
	payload := map[string]any{"anchorHero": heroCtx(anchor), "synergy": synergyCtx(m, true), "outputLanguage": language}
	return []ai.Message{{Role: "system", Content: synergyDetailInstruction}, userMessage(language, payload)}
}
