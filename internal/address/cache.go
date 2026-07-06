// internal/address/cache.go
// In-memory cache for Google Places autocomplete results.
// Prevents duplicate API calls for identical queries within cacheTTL.
// This is the second most important cost-protection mechanism
// after the local-first strategy.
package address

import (
	"sync"
	"time"
)

const cacheTTL = 5 * time.Minute

type cacheEntry struct {
	results   []Suggestion
	expiresAt time.Time
}

type suggestionCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newCache() *suggestionCache {
	c := &suggestionCache{
		entries: make(map[string]cacheEntry),
	}
	go c.cleanup()
	return c
}

func (c *suggestionCache) get(key string) ([]Suggestion, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.results, true
}

func (c *suggestionCache) set(key string, results []Suggestion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		results:   results,
		expiresAt: time.Now().Add(cacheTTL),
	}
}

func (c *suggestionCache) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}
