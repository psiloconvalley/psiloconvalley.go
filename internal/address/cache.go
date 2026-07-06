// internal/address/cache.go
// In-memory caches for Google Places results.
// Two separate caches:
//   suggestionCache — autocomplete query results (5 min TTL)
//   detailsCache    — place details by place_id (30 min TTL)
// Both prevent duplicate Google API calls and directly reduce cost.
package address

import (
	"sync"
	"time"
)

const (
	cacheTTL        = 5 * time.Minute
	detailsCacheTTL = 30 * time.Minute
	cleanupInterval = 10 * time.Minute
)

// ── Suggestion cache ──────────────────────────────────────────────────────

type cacheEntry struct {
	results   []Suggestion
	expiresAt time.Time
}

type suggestionCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newCache() *suggestionCache {
	c := &suggestionCache{entries: make(map[string]cacheEntry)}
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
	c.entries[key] = cacheEntry{results: results, expiresAt: time.Now().Add(cacheTTL)}
}

func (c *suggestionCache) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
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

// ── Details cache ─────────────────────────────────────────────────────────

type detailsEntry struct {
	details   *PlaceDetails
	expiresAt time.Time
}

type detailsCache struct {
	mu      sync.RWMutex
	entries map[string]detailsEntry
}

func newDetailsCache() *detailsCache {
	c := &detailsCache{entries: make(map[string]detailsEntry)}
	go c.cleanup()
	return c
}

func (c *detailsCache) get(placeID string) (*PlaceDetails, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[placeID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.details, true
}

func (c *detailsCache) set(placeID string, details *PlaceDetails) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[placeID] = detailsEntry{
		details:   details,
		expiresAt: time.Now().Add(detailsCacheTTL),
	}
}

func (c *detailsCache) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
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

// ── Per-user rate limiter ─────────────────────────────────────────────────
// Tracks Google API calls per user per minute.
// Prevents a single user from burning through quota.

const (
	rateLimitPerMinute = 10 // max Google calls per user per minute
	rateLimitWindow    = time.Minute
)

type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[int64]rateLimitEntry
}

func newRateLimiter() *rateLimiter {
	r := &rateLimiter{entries: make(map[int64]rateLimitEntry)}
	go r.cleanup()
	return r
}

// Allow returns true if the user is within the rate limit.
func (r *rateLimiter) Allow(userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	e, ok := r.entries[userID]

	if !ok || now.After(e.windowEnd) {
		// New window
		r.entries[userID] = rateLimitEntry{count: 1, windowEnd: now.Add(rateLimitWindow)}
		return true
	}

	if e.count >= rateLimitPerMinute {
		return false
	}

	e.count++
	r.entries[userID] = e
	return true
}

func (r *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for k, e := range r.entries {
			if now.After(e.windowEnd) {
				delete(r.entries, k)
			}
		}
		r.mu.Unlock()
	}
}
