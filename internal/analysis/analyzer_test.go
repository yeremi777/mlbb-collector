package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/ai"
	"github.com/yeremi777/mlbb-collector/internal/hero"
)

// scriptedProvider answers each request with the next of answers, an object
// written as JSON, and records the messages it received.
type scriptedProvider struct {
	answers  []string
	received [][]ai.Message
}

func (p *scriptedProvider) Name() string { return "Scripted" }

func (p *scriptedProvider) CompleteJSON(_ context.Context, messages []ai.Message) (map[string]any, error) {
	p.received = append(p.received, messages)
	if len(p.answers) == 0 {
		return nil, errors.New("scriptedProvider: no answer left")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(p.answers[0]), &payload); err != nil {
		return nil, err
	}
	p.answers = p.answers[1:]
	return payload, nil
}

func threeCounters() []Matchup {
	return []Matchup{
		{Partner: hero.Hero{UID: "akai"}},
		{Partner: hero.Hero{UID: "diggie"}, Proof: []Proof{{ID: "diggie-proof"}}},
		{Partner: hero.Hero{UID: "valir"}},
	}
}

func assertError(t *testing.T, err error, code, message string) {
	t.Helper()
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != code || ae.Message != message {
		t.Errorf("got error %v, want %s %q", err, code, message)
	}
}

func TestScoringRanksByScoreThenConfidenceThenHeroID(t *testing.T) {
	provider := &scriptedProvider{answers: []string{`{"recommendations":[
		{"counterHeroId":"valir","score":80,"confidence":70},
		{"counterHeroId":"diggie","score":90,"confidence":60},
		{"counterHeroId":"akai","score":80,"confidence":70}]}`}}
	got, err := New(provider, testConfig).ScoreCounters(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters(), "en")
	if err != nil {
		t.Fatal(err)
	}
	want := []Ranked{
		{Rank: 1, HeroID: "diggie", Score: 90, Confidence: 60},
		{Rank: 2, HeroID: "akai", Score: 80, Confidence: 70},
		{Rank: 3, HeroID: "valir", Score: 80, Confidence: 70},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}

	provider = &scriptedProvider{answers: []string{`{"recommendations":[
		{"synergyHeroId":"valir","score":80,"confidence":75},
		{"synergyHeroId":"diggie","score":80,"confidence":60},
		{"synergyHeroId":"akai","score":80,"confidence":70}]}`}}
	got, err = New(provider, testConfig).ScoreSynergies(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters(), "en")
	if err != nil {
		t.Fatal(err)
	}
	want = []Ranked{
		{Rank: 1, HeroID: "valir", Score: 80, Confidence: 75},
		{Rank: 2, HeroID: "akai", Score: 80, Confidence: 70},
		{Rank: 3, HeroID: "diggie", Score: 80, Confidence: 60},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("synergies: got %+v\nwant %+v", got, want)
	}
}

func TestScoringRejectsAnAnswerBreakingARule(t *testing.T) {
	const (
		every   = "Scoring response must include every expected counterHeroId once."
		missing = "Invalid scoring response from model: missing counterHeroId, score, or confidence"
	)
	akai, diggie := `{"counterHeroId":"akai","score":80,"confidence":70}`, `{"counterHeroId":"diggie","score":80,"confidence":70}`
	for _, tt := range []struct{ name, answer, message string }{
		{"no recommendations", `{}`, "Invalid scoring response from model: recommendations must be an array"},
		{"recommendations not an array", `{"recommendations":{}}`, "Invalid scoring response from model: recommendations must be an array"},
		{"recommendation not an object", `{"recommendations":[1]}`, "Invalid scoring response from model: recommendation must be an object"},
		{"hero ID missing", `{"recommendations":[{"score":80,"confidence":70}]}`, missing},
		{"score missing", `{"recommendations":[{"counterHeroId":"akai","confidence":70}]}`, missing},
		{"score not an integer", `{"recommendations":[{"counterHeroId":"akai","score":80.5,"confidence":70}]}`, missing},
		{"score above 100", `{"recommendations":[{"counterHeroId":"akai","score":101,"confidence":70}]}`, missing},
		{"confidence below 0", `{"recommendations":[{"counterHeroId":"akai","score":80,"confidence":-1}]}`, missing},
		{"a hero left out", `{"recommendations":[` + akai + `,` + diggie + `]}`, every},
		{"a hero twice", `{"recommendations":[` + akai + `,` + diggie + `,` + diggie + `]}`, every},
		{"every hero, one twice", `{"recommendations":[` + akai + `,` + diggie + `,` + diggie + `,{"counterHeroId":"valir","score":80,"confidence":70}]}`, every},
		{"a hero twice in place of another", `{"recommendations":[` + akai + `,` + akai + `,` + diggie + `]}`, every},
		{"an unexpected hero", `{"recommendations":[` + akai + `,` + diggie + `,{"counterHeroId":"lunox","score":80,"confidence":70}]}`, every},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{answers: []string{tt.answer}}
			_, err := New(provider, testConfig).ScoreCounters(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters(), "en")
			assertError(t, err, "ai_provider_error", tt.message)
		})
	}
}

const validDiggieDetail = `{"score":90,"confidence":80,"summary":"Cleanses the engage.","strengths":["Time Journey."],"evidenceIds":["diggie-proof"]}`

func TestDetailFillsMissingOptionalArrays(t *testing.T) {
	provider := &scriptedProvider{answers: []string{validDiggieDetail}}
	got, err := New(provider, testConfig).CounterDetail(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters()[1], "en")
	if err != nil {
		t.Fatal(err)
	}
	want := Detail{Score: 90, Confidence: 80, Summary: "Cleanses the engage.", Strengths: []string{"Time Journey."},
		Conditions: []string{}, FailureCases: []string{}, EvidenceIDs: []string{"diggie-proof"}}
	if !reflect.DeepEqual(got, want) || len(provider.received) != 1 {
		t.Errorf("got %+v after %d requests\nwant %+v after 1", got, len(provider.received), want)
	}
}

func TestDetailRepairsAnInvalidAnswerOnce(t *testing.T) {
	const missingFields = "detail payload missing required fields: score, confidence, non-empty summary, non-empty strengths"
	for _, tt := range []struct{ name, first, cause string }{
		{"score missing", `{"confidence":80,"summary":"s","strengths":["x"]}`, missingFields},
		{"score not an integer", `{"score":90.5,"confidence":80,"summary":"s","strengths":["x"]}`, missingFields},
		{"score above 100", `{"score":101,"confidence":80,"summary":"s","strengths":["x"]}`, missingFields},
		{"confidence below 0", `{"score":90,"confidence":-1,"summary":"s","strengths":["x"]}`, missingFields},
		{"confidence not an integer", `{"score":90,"confidence":79.9,"summary":"s","strengths":["x"]}`, missingFields},
		{"summary blank", `{"score":90,"confidence":80,"summary":"  ","strengths":["x"]}`, missingFields},
		{"strengths empty", `{"score":90,"confidence":80,"summary":"s","strengths":[]}`, missingFields},
		{"strengths not an array", `{"score":90,"confidence":80,"summary":"s","strengths":"x"}`,
			"detail payload has wrong field types: json: cannot unmarshal string into Go struct field .strengths of type []string"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{answers: []string{tt.first, validDiggieDetail}}
			got, err := New(provider, testConfig).CounterDetail(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters()[1], "en")
			if err != nil || got.Score != 90 || len(provider.received) != 2 {
				t.Fatalf("got %+v, %v after %d requests; want the repaired answer after 2", got, err, len(provider.received))
			}
			repair := provider.received[1]
			if len(repair) != 4 || repair[2].Role != "assistant" || repair[3].Role != "user" {
				t.Fatalf("repair messages: %+v", repair)
			}
			if want := "Validation error:\n" + tt.cause; repair[3].Content[len(repair[3].Content)-len(want):] != want {
				t.Errorf("repair ends with %q, want %q", repair[3].Content, want)
			}

			provider = &scriptedProvider{answers: []string{tt.first, tt.first}}
			_, err = New(provider, testConfig).CounterDetail(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters()[1], "en")
			assertError(t, err, "ai_provider_error", "Invalid detail response from model after retry: "+tt.cause)
			if len(provider.received) != 2 {
				t.Errorf("a second invalid answer: %d requests, want 2", len(provider.received))
			}
		})
	}
}

func TestDetailFailsAtOnceOnAnUnknownEvidenceID(t *testing.T) {
	provider := &scriptedProvider{answers: []string{
		`{"score":90,"confidence":80,"summary":"s","strengths":["x"],"evidenceIds":["made-up"]}`, validDiggieDetail}}
	_, err := New(provider, testConfig).CounterDetail(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters()[1], "en")
	assertError(t, err, "ai_provider_error", "Detail response referenced unknown evidence ids.")
	if len(provider.received) != 1 {
		t.Errorf("%d requests, want 1: no repair", len(provider.received))
	}

	provider = &scriptedProvider{answers: []string{`{"score":90}`,
		`{"score":90,"confidence":80,"summary":"s","strengths":["x"],"evidenceIds":["made-up"]}`}}
	_, err = New(provider, testConfig).CounterDetail(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters()[1], "en")
	assertError(t, err, "ai_provider_error", "Detail response referenced unknown evidence ids.")
}

func TestRepairMessagesEqualDevs(t *testing.T) {
	target, counters, _ := tigreal(t)
	diggie := partner(t, counters, "diggie")
	for name, first := range map[string]string{
		"missing-summary": `{"score":90,"confidence":80,"strengths":["Cleanses the engage."]}`,
		"wrong-types":     `{"score":90,"confidence":80,"summary":"s","strengths":"Cleanses the engage."}`,
	} {
		provider := &scriptedProvider{answers: []string{first, `{"score":90,"confidence":80,"summary":"s","strengths":["x"]}`}}
		if _, err := New(provider, testConfig).CounterDetail(context.Background(), target, diggie, "en"); err != nil {
			t.Fatal(err)
		}
		assertGoldenMessages(t, "counter-detail-repair-"+name+".en.json", provider.received[1])
	}
}

var testConfig = Config{Timeout: time.Minute}

// stalledProvider answers like a provider whose request outlasts ctx, after
// answering the first of answers at once.
type stalledProvider struct {
	answers  []string
	requests int
}

func (p *stalledProvider) Name() string { return "Stalled" }

func (p *stalledProvider) CompleteJSON(ctx context.Context, messages []ai.Message) (map[string]any, error) {
	p.requests++
	if len(p.answers) > 0 {
		answer := p.answers[0]
		p.answers = p.answers[1:]
		return (&scriptedProvider{answers: []string{answer}}).CompleteJSON(ctx, messages)
	}
	<-ctx.Done()
	return nil, &ai.Error{Message: "Stalled request timed out.", Retryable: true}
}

func TestTheDeadlineBoundsAllProviderWork(t *testing.T) {
	cfg := Config{Timeout: 50 * time.Millisecond}
	target, ms := hero.Hero{UID: "tigreal"}, threeCounters()
	for name, tt := range map[string]struct {
		run      func() (int, error)
		requests int
	}{
		"scoring": {func() (int, error) {
			p := &stalledProvider{}
			_, err := New(p, cfg).ScoreCounters(context.Background(), target, ms, "en")
			return p.requests, err
		}, 1},
		"a chain, which asks no further provider": {func() (int, error) {
			first, second := &stalledProvider{}, &stalledProvider{}
			_, err := New(ai.Chain(first, second), cfg).ScoreSynergies(context.Background(), target, ms, "en")
			return first.requests + second.requests, err
		}, 1},
		"the detail repair": {func() (int, error) {
			p := &stalledProvider{answers: []string{`{"score":90}`}}
			_, err := New(p, cfg).CounterDetail(context.Background(), target, ms[1], "en")
			return p.requests, err
		}, 2},
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			requests, err := tt.run()
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("answered after %v, want about 50ms", elapsed)
			}
			assertError(t, err, "ai_provider_timeout", "AI provider did not answer in time.")
			if requests != tt.requests {
				t.Errorf("%d requests, want %d", requests, tt.requests)
			}
		})
	}
}
