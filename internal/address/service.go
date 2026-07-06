// internal/address/service.go
// Service coordinates LocalProvider and GoogleProvider.
//
// Cost protection rules (in order):
//   1. Local always runs first — zero cost
//   2. If local returns >= localThreshold results, return immediately
//   3. Google only called if query length >= minQueryLen
//   4. Per-user rate limit: max 10 Google calls per minute
//   5. Suggestion cache: same query hits Google once per 5 minutes
//   6. Details cache: same place_id hits Google once per 30 minutes
//   7. Google fails open — never blocks the user
//   8. Kill switch: if Google API key is empty, local only
package address

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
)

// Service is the single entry point for address autocomplete.
// Handlers use Service — never the providers directly.
type Service struct {
	local       *LocalProvider
	google      *GoogleProvider
	cache       *suggestionCache
	detailsCache *detailsCache
	rateLimiter *rateLimiter
}

// New creates an address Service.
// Reads GOOGLE_MAPS_API_KEY from environment — empty = local only.
func New(db *sql.DB) *Service {
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	return &Service{
		local:        NewLocalProvider(db),
		google:       NewGoogleProvider(apiKey),
		cache:        newCache(),
		detailsCache: newDetailsCache(),
		rateLimiter:  newRateLimiter(),
	}
}

// Suggest returns address suggestions for the given query.
// Combines local and Google results with cost protection.
func (s *Service) Suggest(
	ctx context.Context,
	userID int64,
	query string,
) ([]Suggestion, error) {
	if len(query) < 2 {
		return []Suggestion{}, nil
	}

	// Step 1: local search — always free
	local, err := s.local.Search(ctx, userID, query)
	if err != nil {
		local = nil // degrade gracefully
	}

	// Step 2: enough local results — skip Google entirely
	if len(local) >= localThreshold {
		return local, nil
	}

	// Step 3: Google eligibility check
	if !s.google.Enabled() || len(query) < minQueryLen {
		return local, nil
	}

	// Step 4: per-user rate limit
	if !s.rateLimiter.Allow(userID) {
		slog.Warn("address autocomplete rate limit reached", "user_id", userID)
		return local, nil
	}

	// Step 5: suggestion cache
	if cached, ok := s.cache.get(query); ok {
		return merge(local, cached), nil
	}

	// Step 6: call Google autocomplete
	googleResults, err := s.google.Autocomplete(ctx, query)
	if err != nil || googleResults == nil {
		return local, nil // fail open
	}

	// Step 7: cache Google results
	s.cache.set(query, googleResults)

	return merge(local, googleResults), nil
}

// PlaceDetails fetches full address details for a Google place_id.
// Checks cache first — only calls Google if not cached.
// Called once when the user selects a Google suggestion.
func (s *Service) PlaceDetails(
	ctx context.Context,
	placeID string,
) (*PlaceDetails, error) {
	// Check details cache first
	if cached, ok := s.detailsCache.get(placeID); ok {
		return cached, nil
	}

	details, err := s.google.Details(ctx, placeID)
	if err != nil || details == nil {
		return nil, err
	}

	// Cache for 30 minutes — place details don't change
	s.detailsCache.set(placeID, details)
	return details, nil
}

// merge combines local and Google suggestions, deduplicating by label.
// Local results always appear first. Total capped at 5.
func merge(local, google []Suggestion) []Suggestion {
	seen := make(map[string]bool)
	var out []Suggestion

	for _, s := range local {
		if !seen[s.Label] && len(out) < 5 {
			seen[s.Label] = true
			out = append(out, s)
		}
	}
	for _, s := range google {
		if !seen[s.Label] && len(out) < 5 {
			seen[s.Label] = true
			out = append(out, s)
		}
	}
	return out
}
