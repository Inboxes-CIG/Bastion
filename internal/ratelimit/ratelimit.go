package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a simple in-memory token-bucket rate limiter keyed by an arbitrary
// string (e.g. site + client IP).
type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	lastCleanup time.Time
}

func New() *Limiter {
	return &Limiter{
		buckets:     map[string]*bucket{},
		lastCleanup: time.Now(),
	}
}

// Allow consumes one token for key. rps is the refill rate (requests/second),
// burst is the bucket capacity. A non-positive rps disables limiting.
func (l *Limiter) Allow(key string, rps, burst int) bool {
	if rps <= 0 {
		return true
	}
	if burst <= 0 {
		burst = rps
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: float64(burst), last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * float64(rps)
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
	}
	b.last = now

	allowed := b.tokens >= 1
	if allowed {
		b.tokens--
	}

	if now.Sub(l.lastCleanup) > 5*time.Minute {
		l.cleanupLocked(now)
	}

	return allowed
}

// cleanupLocked removes buckets that have been idle for a while.
func (l *Limiter) cleanupLocked(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, k)
		}
	}
	l.lastCleanup = now
}

// Size returns the number of tracked buckets (for diagnostics).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
