package analysis

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/yeremi777/mlbb-collector/internal/hero"
)

const threeScores = `{"recommendations":[{"counterHeroId":"akai","score":80,"confidence":70},` +
	`{"counterHeroId":"diggie","score":90,"confidence":60},{"counterHeroId":"valir","score":70,"confidence":70}]}`

var cachingConfig = Config{Timeout: time.Minute, CacheTTL: time.Minute, CacheMaxEntries: 8}

func TestACachedResultAsksNoProvider(t *testing.T) {
	provider := &scriptedProvider{answers: []string{threeScores, validDiggieDetail, threeScores}}
	a := New(provider, cachingConfig)
	target, ms := hero.Hero{UID: "tigreal"}, threeCounters()

	first, err := a.ScoreCounters(context.Background(), target, ms, "en")
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.ScoreCounters(context.Background(), target, ms, "en")
	if err != nil || !reflect.DeepEqual(again, first) || len(provider.received) != 1 {
		t.Fatalf("again: %+v, %v after %d requests; want %+v after 1", again, err, len(provider.received), first)
	}

	detail, err := a.CounterDetail(context.Background(), target, ms[1], "en")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := a.CounterDetail(context.Background(), target, ms[1], "en"); err != nil || !reflect.DeepEqual(again, detail) || len(provider.received) != 2 {
		t.Fatalf("detail again: %+v, %v after %d requests; want %+v after 2", again, err, len(provider.received), detail)
	}

	if _, err := a.ScoreCounters(context.Background(), target, ms, "id"); err != nil || len(provider.received) != 3 {
		t.Errorf("another language: %v after %d requests; want a third request", err, len(provider.received))
	}
	if _, err := a.ScoreSynergies(context.Background(), target, ms, "en"); len(provider.received) != 4 {
		t.Errorf("synergies of the same hero: %v after %d requests; want a fourth request", err, len(provider.received))
	}
}

func TestACallerCannotChangeACachedResult(t *testing.T) {
	provider := &scriptedProvider{answers: []string{threeScores, validDiggieDetail}}
	a := New(provider, cachingConfig)
	target, ms := hero.Hero{UID: "tigreal"}, threeCounters()

	ranked, _ := a.ScoreCounters(context.Background(), target, ms, "en")
	ranked[0].Score = 0
	again, _ := a.ScoreCounters(context.Background(), target, ms, "en")
	if again[0].Score != 90 {
		t.Errorf("cached score changed to %d, want 90", again[0].Score)
	}
	again[0].Score = 0
	if third, _ := a.ScoreCounters(context.Background(), target, ms, "en"); third[0].Score != 90 {
		t.Errorf("a cache hit changed the cached score to %d, want 90", third[0].Score)
	}
	detail, _ := a.CounterDetail(context.Background(), target, ms[1], "en")
	detail.Strengths[0] = "changed"
	if again, _ := a.CounterDetail(context.Background(), target, ms[1], "en"); again.Strengths[0] != "Time Journey." {
		t.Errorf("cached strength changed to %q", again.Strengths[0])
	}
}

func TestAFailureIsNotCached(t *testing.T) {
	provider := &scriptedProvider{answers: []string{`{}`, threeScores}}
	a := New(provider, cachingConfig)
	target, ms := hero.Hero{UID: "tigreal"}, threeCounters()
	if _, err := a.ScoreCounters(context.Background(), target, ms, "en"); err == nil {
		t.Fatal("want the invalid answer to fail")
	}
	if got, err := a.ScoreCounters(context.Background(), target, ms, "en"); err != nil || got[0].HeroID != "diggie" || len(provider.received) != 2 {
		t.Errorf("got %+v, %v after %d requests; want the second answer after 2", got, err, len(provider.received))
	}
}

func TestAZeroSettingDisablesTheCache(t *testing.T) {
	for name, cfg := range map[string]Config{
		"zero lifetime": {Timeout: time.Minute, CacheMaxEntries: 8},
		"zero size":     {Timeout: time.Minute, CacheTTL: time.Minute},
	} {
		provider := &scriptedProvider{answers: []string{threeScores, threeScores}}
		a := New(provider, cfg)
		for range 2 {
			if _, err := a.ScoreCounters(context.Background(), hero.Hero{UID: "tigreal"}, threeCounters(), "en"); err != nil {
				t.Fatal(err)
			}
		}
		if len(provider.received) != 2 {
			t.Errorf("%s: %d requests, want 2", name, len(provider.received))
		}
	}
}

func TestTheCacheExpiresAndEvictsTheLeastRecentlyUsed(t *testing.T) {
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	c := newCache(time.Minute, 2, func() time.Time { return clock })
	a, b, d := cacheKey{hero: "a"}, cacheKey{hero: "b"}, cacheKey{hero: "d"}
	c.set(a, result{ranked: []Ranked{{HeroID: "a"}}})
	c.set(b, result{ranked: []Ranked{{HeroID: "b"}}})
	if _, ok := c.get(a); !ok {
		t.Fatal("a missing")
	}
	c.set(d, result{ranked: []Ranked{{HeroID: "d"}}})
	if _, ok := c.get(b); ok {
		t.Error("b, the least recently used, was kept")
	}
	if got, ok := c.get(a); !ok || got.ranked[0].HeroID != "a" {
		t.Errorf("a: got %+v, %v; want it kept", got, ok)
	}

	clock = clock.Add(time.Minute)
	if _, ok := c.get(d); !ok {
		t.Error("d expired at its lifetime, want it kept until after")
	}
	clock = clock.Add(time.Nanosecond)
	if _, ok := c.get(d); ok {
		t.Error("d kept after its lifetime")
	}
}

func TestAZeroLifetimeKeepsNothingEvenWhenTheClockStands(t *testing.T) {
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	c := newCache(0, 2, func() time.Time { return clock })
	c.set(cacheKey{hero: "a"}, result{ranked: []Ranked{{HeroID: "a"}}})
	if _, ok := c.get(cacheKey{hero: "a"}); ok {
		t.Error("kept a result with a zero lifetime")
	}
}
