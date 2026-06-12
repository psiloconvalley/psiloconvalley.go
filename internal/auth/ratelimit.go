package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// =====================================================================
// IP-based sliding window rate limiter
// No external dependencies — pure stdlib
// =====================================================================

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	// Background cleanup — prevent unbounded memory growth
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()
	return rl
}

// allow returns true if the IP is within the rate limit.
func (rl *rateLimiter) allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Slide the window — drop requests older than cutoff
	times := rl.requests[ip]
	valid := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

// cleanup removes IPs with no recent requests.
func (rl *rateLimiter) cleanup() {
	cutoff := time.Now().Add(-rl.window)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, times := range rl.requests {
		allOld := true
		for _, t := range times {
			if t.After(cutoff) {
				allOld = false
				break
			}
		}
		if allOld {
			delete(rl.requests, ip)
		}
	}
}

// realIP extracts the real client IP, respecting Railway's proxy headers.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For can be comma-separated; take the first
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// =====================================================================
// Pre-built limiters for auth routes
// =====================================================================

var (
	loginLimiter        = newRateLimiter(10, time.Minute)
	registerLimiter     = newRateLimiter(5, time.Minute)
	forgotPasswordLimiter = newRateLimiter(5, time.Minute)
	magicLimiter        = newRateLimiter(10, time.Minute)
)

// RateLimit returns a middleware that applies the given limiter.
// Returns 429 Too Many Requests when the limit is exceeded.
func RateLimit(limiter *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !limiter.allow(ip) {
				http.Error(w, "Too many requests. Please wait a moment and try again.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Named middleware functions for each route group.
// Used in router.go to keep the routing table readable.

func RateLimitLogin(next http.Handler) http.Handler {
	return RateLimit(loginLimiter)(next)
}

func RateLimitRegister(next http.Handler) http.Handler {
	return RateLimit(registerLimiter)(next)
}

func RateLimitForgotPassword(next http.Handler) http.Handler {
	return RateLimit(forgotPasswordLimiter)(next)
}

func RateLimitMagicLink(next http.Handler) http.Handler {
	return RateLimit(magicLimiter)(next)
}
