package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/yeremi777/mlbb-collector/internal/counter"
	"github.com/yeremi777/mlbb-collector/internal/hero"
	"github.com/yeremi777/mlbb-collector/internal/httpx"
	"github.com/yeremi777/mlbb-collector/internal/synergy"
)

type heroStore interface {
	Get(ctx context.Context, uid string) (hero.Hero, error)
}

type counterStore interface {
	ForTarget(ctx context.Context, target string) ([]counter.WithHero, error)
}

type synergyStore interface {
	ForAnchor(ctx context.Context, anchor string) ([]synergy.WithHero, error)
}

// Handler serves the four analyze routes.
type Handler struct {
	heroes    heroStore
	counters  counterStore
	synergies synergyStore
	analyzer  *Analyzer
}

// NewHandler analyzes with analyzer the Counters and Synergies read from the
// stores. A nil analyzer means no provider is configured.
func NewHandler(heroes heroStore, counters counterStore, synergies synergyStore, analyzer *Analyzer) Handler {
	return Handler{heroes: heroes, counters: counters, synergies: synergies, analyzer: analyzer}
}

// Register adds the analyze routes to mux.
func (h Handler) Register(mux httpx.Mux) {
	mux.HandleFunc("POST /api/counters/analyze-score", h.counterScore)
	mux.HandleFunc("POST /api/counters/analyze-detail", h.counterDetail)
	mux.HandleFunc("POST /api/synergies/analyze-score", h.synergyScore)
	mux.HandleFunc("POST /api/synergies/analyze-detail", h.synergyDetail)
}

// counterRequest is a Counter route's body; scoring leaves out CounterHeroID.
type counterRequest struct {
	TargetHeroID  string `json:"targetHeroId"`
	CounterHeroID string `json:"counterHeroId"`
	Language      string `json:"language"`
}

// synergyRequest is a Synergy route's body; scoring leaves out SynergyHeroID.
type synergyRequest struct {
	AnchorHeroID  string `json:"anchorHeroId"`
	SynergyHeroID string `json:"synergyHeroId"`
	Language      string `json:"language"`
}

type counterRecommendation struct {
	Rank          int    `json:"rank"`
	CounterHeroID string `json:"counterHeroId"`
	Score         int    `json:"score"`
	Confidence    int    `json:"confidence"`
}

type synergyRecommendation struct {
	Rank          int    `json:"rank"`
	SynergyHeroID string `json:"synergyHeroId"`
	Score         int    `json:"score"`
	Confidence    int    `json:"confidence"`
}

type counterScoreResponse struct {
	TargetHeroID    string                  `json:"targetHeroId"`
	Source          string                  `json:"source"`
	Recommendations []counterRecommendation `json:"recommendations"`
}

type synergyScoreResponse struct {
	AnchorHeroID    string                  `json:"anchorHeroId"`
	Source          string                  `json:"source"`
	Recommendations []synergyRecommendation `json:"recommendations"`
}

type detailBody struct {
	Score        int      `json:"score"`
	Confidence   int      `json:"confidence"`
	Summary      string   `json:"summary"`
	Strengths    []string `json:"strengths"`
	Conditions   []string `json:"conditions"`
	FailureCases []string `json:"failureCases"`
	EvidenceIDs  []string `json:"evidenceIds"`
}

func newDetailBody(d Detail) detailBody {
	return detailBody{Score: d.Score, Confidence: d.Confidence, Summary: d.Summary, Strengths: d.Strengths,
		Conditions: d.Conditions, FailureCases: d.FailureCases, EvidenceIDs: d.EvidenceIDs}
}

type counterDetailResponse struct {
	TargetHeroID  string `json:"targetHeroId"`
	CounterHeroID string `json:"counterHeroId"`
	Source        string `json:"source"`
	detailBody
}

type synergyDetailResponse struct {
	AnchorHeroID  string `json:"anchorHeroId"`
	SynergyHeroID string `json:"synergyHeroId"`
	Source        string `json:"source"`
	detailBody
}

func (h Handler) counterScore(w http.ResponseWriter, r *http.Request) {
	var req counterRequest
	if !h.accept(w, r, &req) {
		return
	}
	lang, ok := language(w, req.Language)
	if !ok {
		return
	}
	target, ms, ok := h.counterMatchups(w, r, req.TargetHeroID)
	if !ok {
		return
	}
	ranked, err := h.analyzer.ScoreCounters(r.Context(), target, ms, lang)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	recs := make([]counterRecommendation, len(ranked))
	for i, x := range ranked {
		recs[i] = counterRecommendation{Rank: x.Rank, CounterHeroID: x.HeroID, Score: x.Score, Confidence: x.Confidence}
	}
	httpx.WriteJSON(w, http.StatusOK, counterScoreResponse{TargetHeroID: req.TargetHeroID, Source: "ai", Recommendations: recs})
}

func (h Handler) synergyScore(w http.ResponseWriter, r *http.Request) {
	var req synergyRequest
	if !h.accept(w, r, &req) {
		return
	}
	lang, ok := language(w, req.Language)
	if !ok {
		return
	}
	anchor, ms, ok := h.synergyMatchups(w, r, req.AnchorHeroID)
	if !ok {
		return
	}
	ranked, err := h.analyzer.ScoreSynergies(r.Context(), anchor, ms, lang)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	recs := make([]synergyRecommendation, len(ranked))
	for i, x := range ranked {
		recs[i] = synergyRecommendation{Rank: x.Rank, SynergyHeroID: x.HeroID, Score: x.Score, Confidence: x.Confidence}
	}
	httpx.WriteJSON(w, http.StatusOK, synergyScoreResponse{AnchorHeroID: req.AnchorHeroID, Source: "ai", Recommendations: recs})
}

func (h Handler) counterDetail(w http.ResponseWriter, r *http.Request) {
	var req counterRequest
	if !h.accept(w, r, &req) {
		return
	}
	lang, ok := language(w, req.Language)
	if !ok {
		return
	}
	target, ms, ok := h.counterMatchups(w, r, req.TargetHeroID)
	if !ok {
		return
	}
	m, ok := h.partner(w, r, ms, req.CounterHeroID,
		"counter_hero_not_found", "Counter hero was not found in the dataset.",
		"counter_matchup_not_found", "Counter matchup was not found for the target hero.")
	if !ok {
		return
	}
	d, err := h.analyzer.CounterDetail(r.Context(), target, m, lang)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, counterDetailResponse{
		TargetHeroID: req.TargetHeroID, CounterHeroID: req.CounterHeroID, Source: "ai", detailBody: newDetailBody(d)})
}

func (h Handler) synergyDetail(w http.ResponseWriter, r *http.Request) {
	var req synergyRequest
	if !h.accept(w, r, &req) {
		return
	}
	lang, ok := language(w, req.Language)
	if !ok {
		return
	}
	anchor, ms, ok := h.synergyMatchups(w, r, req.AnchorHeroID)
	if !ok {
		return
	}
	m, ok := h.partner(w, r, ms, req.SynergyHeroID,
		"synergy_hero_not_found", "Synergy hero was not found in the dataset.",
		"synergy_matchup_not_found", "Synergy matchup was not found for the anchor hero.")
	if !ok {
		return
	}
	d, err := h.analyzer.SynergyDetail(r.Context(), anchor, m, lang)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, synergyDetailResponse{
		AnchorHeroID: req.AnchorHeroID, SynergyHeroID: req.SynergyHeroID, Source: "ai", detailBody: newDetailBody(d)})
}

// accept decodes the body into req and requires a provider, answering the
// first failure.
func (h Handler) accept(w http.ResponseWriter, r *http.Request, req any) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_request", "Request body is not valid JSON.")
		return false
	}
	if h.analyzer == nil {
		httpx.WriteError(w, http.StatusGatewayTimeout, "ai_provider_not_configured",
			"AI provider is not configured. Set the required API key in .env.")
		return false
	}
	return true
}

// language returns the requested language, empty meaning en, or answers 422.
func language(w http.ResponseWriter, requested string) (string, bool) {
	switch requested {
	case "":
		return "en", true
	case "en", "id":
		return requested, true
	}
	httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_request", "language must be 'en' or 'id'.")
	return "", false
}

// counterMatchups returns the target hero and its Counters, or answers 404
// when either is missing.
func (h Handler) counterMatchups(w http.ResponseWriter, r *http.Request, target string) (hero.Hero, []Matchup, bool) {
	t, ok := h.hero(w, r, target, "target_hero_not_found")
	if !ok {
		return hero.Hero{}, nil, false
	}
	counters, err := h.counters.ForTarget(r.Context(), target)
	if err != nil {
		httpx.InternalError(w, "list counters", err)
		return hero.Hero{}, nil, false
	}
	if len(counters) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "counter_data_not_found", "Counter data was not found for the target hero.")
		return hero.Hero{}, nil, false
	}
	ms := make([]Matchup, len(counters))
	for i, c := range counters {
		ms[i] = Matchup{Partner: c.CounterHero, Reasons: c.Reasons, Types: c.CounterTypes, Proof: make([]Proof, len(c.Proof))}
		for j, p := range c.Proof {
			ms[i].Proof[j] = Proof(p)
		}
	}
	return t, ms, true
}

// synergyMatchups returns the anchor hero and its Synergies, or answers 404
// when either is missing.
func (h Handler) synergyMatchups(w http.ResponseWriter, r *http.Request, anchor string) (hero.Hero, []Matchup, bool) {
	a, ok := h.hero(w, r, anchor, "anchor_hero_not_found")
	if !ok {
		return hero.Hero{}, nil, false
	}
	synergies, err := h.synergies.ForAnchor(r.Context(), anchor)
	if err != nil {
		httpx.InternalError(w, "list synergies", err)
		return hero.Hero{}, nil, false
	}
	if len(synergies) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "synergy_data_not_found", "Synergy data was not found for the anchor hero.")
		return hero.Hero{}, nil, false
	}
	ms := make([]Matchup, len(synergies))
	for i, s := range synergies {
		ms[i] = Matchup{Partner: s.SynergyHero, Reasons: s.Reasons, Types: s.SynergyTypes, Proof: make([]Proof, len(s.Proof))}
		for j, p := range s.Proof {
			ms[i].Proof[j] = Proof(p)
		}
	}
	return a, ms, true
}

// hero returns the hero with the given ID, or answers 404 with code.
func (h Handler) hero(w http.ResponseWriter, r *http.Request, uid, code string) (hero.Hero, bool) {
	found, err := h.heroes.Get(r.Context(), uid)
	if errors.Is(err, hero.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, code, "Hero was not found in the dataset.")
		return hero.Hero{}, false
	}
	if err != nil {
		httpx.InternalError(w, "get hero", err)
		return hero.Hero{}, false
	}
	return found, true
}

// partner returns the Matchup with the partner hero uid, answering 404 when
// the hero does not exist or is not a partner in ms.
func (h Handler) partner(w http.ResponseWriter, r *http.Request, ms []Matchup, uid, heroCode, heroMessage, matchupCode, matchupMessage string) (Matchup, bool) {
	if _, err := h.heroes.Get(r.Context(), uid); errors.Is(err, hero.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, heroCode, heroMessage)
		return Matchup{}, false
	} else if err != nil {
		httpx.InternalError(w, "get partner hero", err)
		return Matchup{}, false
	}
	for _, m := range ms {
		if m.Partner.UID == uid {
			return m, true
		}
	}
	httpx.WriteError(w, http.StatusNotFound, matchupCode, matchupMessage)
	return Matchup{}, false
}

// writeAnalysisError answers an analysis failure with its code.
func writeAnalysisError(w http.ResponseWriter, err error) {
	var ae *Error
	if !errors.As(err, &ae) {
		httpx.InternalError(w, "analyze", err)
		return
	}
	status := http.StatusBadGateway
	if ae.Code == "ai_provider_timeout" {
		status = http.StatusGatewayTimeout
	}
	httpx.WriteError(w, status, ae.Code, ae.Message)
}
