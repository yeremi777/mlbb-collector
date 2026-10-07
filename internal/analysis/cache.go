package analysis

import (
	"container/list"
	"slices"
	"sync"
	"time"
)

// cacheKey names one result: kind is counter-score, synergy-score,
// counter-detail, or synergy-detail, and partner is empty for a score.
type cacheKey struct {
	kind, hero, partner, language string
}

// result is a cached scoring result or detail.
type result struct {
	ranked []Ranked
	detail Detail
}

// clone copies r so neither the cache nor a caller sees the other's changes.
func (r result) clone() result {
	d := r.detail
	d.Strengths, d.Conditions = slices.Clone(d.Strengths), slices.Clone(d.Conditions)
	d.FailureCases, d.EvidenceIDs = slices.Clone(d.FailureCases), slices.Clone(d.EvidenceIDs)
	return result{ranked: slices.Clone(r.ranked), detail: d}
}

// cache keeps results for ttl, holding at most max and evicting the least
// recently used. A zero ttl or max keeps nothing.
type cache struct {
	ttl time.Duration
	max int
	now func() time.Time

	mu      sync.Mutex
	entries map[cacheKey]*list.Element
	order   *list.List // front is the least recently used
}

type cacheEntry struct {
	key     cacheKey
	expires time.Time
	value   result
}

func newCache(ttl time.Duration, max int, now func() time.Time) *cache {
	return &cache{ttl: ttl, max: max, now: now, entries: map[cacheKey]*list.Element{}, order: list.New()}
}

func (c *cache) get(key cacheKey) (result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return result{}, false
	}
	entry := el.Value.(*cacheEntry)
	if c.now().After(entry.expires) {
		c.order.Remove(el)
		delete(c.entries, key)
		return result{}, false
	}
	c.order.MoveToBack(el)
	return entry.value.clone(), true
}

func (c *cache) set(key cacheKey, value result) {
	if c.ttl <= 0 || c.max <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.order.Remove(el)
	}
	c.entries[key] = c.order.PushBack(&cacheEntry{key: key, expires: c.now().Add(c.ttl), value: value.clone()})
	for c.order.Len() > c.max {
		oldest := c.order.Front()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheEntry).key)
	}
}
