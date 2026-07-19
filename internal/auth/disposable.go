package auth

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed disposable_domains.txt
var rawDisposableDomains string

var (
	disposableDomainSet  map[string]bool
	disposableDomainOnce sync.Once
)

// loadDisposableDomains parses the embedded domain list once.
// Thread-safe via sync.Once — zero cost after initialization.
func loadDisposableDomains() {
	lines := strings.Split(rawDisposableDomains, "\n")
	disposableDomainSet = make(map[string]bool, len(lines))
	for _, line := range lines {
		d := strings.TrimSpace(strings.ToLower(line))
		if d == "" || strings.HasPrefix(d, "#") {
			continue
		}
		disposableDomainSet[d] = true
	}
}

// IsDisposableEmail reports whether the email address uses a known
// disposable or temporary email domain.
// Thread-safe. The domain list is loaded once on first call.
func IsDisposableEmail(email string) bool {
	disposableDomainOnce.Do(loadDisposableDomains)
	parts := strings.SplitN(strings.ToLower(email), "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false
	}
	return disposableDomainSet[strings.TrimSpace(parts[1])]
}
