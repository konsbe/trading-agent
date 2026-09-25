package momentumapi

import (
	"sync"
	"time"
)

// responseCache holds encoded 200 responses for a short TTL. The data changes
// once a day; the TTL is short so a corrected re-run shows up within minutes
// rather than being hidden behind the cache.
type responseCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]cacheEntry
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

func newResponseCache(ttl time.Duration, now func() time.Time) *responseCache {
	return &responseCache{ttl: ttl, now: now, entries: map[string]cacheEntry{}}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	if c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.body, true
}

func (c *responseCache) set(key string, body []byte) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = cacheEntry{body: body, expires: now.Add(c.ttl)}
}

// setFor stores body with its own TTL, for responses whose freshness cadence
// differs from the default (the once-per-6h market report). ttl <= 0 disables.
func (c *responseCache) setFor(key string, body []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{body: body, expires: c.now().Add(ttl)}
}

// drop removes one entry, e.g. the market report when the watchlist changes.
func (c *responseCache) drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}
