package auth

import (
	"strings"
	"sync"
	"time"
)

type loginEntry struct {
	fails int
	first time.Time
	last  time.Time
	until time.Time
}

// LoginLimiter counts failed authentication attempts per key (IP and email)
// inside a window and locks the key for a configured duration.
type LoginLimiter struct {
	mu        sync.Mutex
	entries   map[string]*loginEntry
	threshold int
	window    time.Duration
	lock      time.Duration
	lastSweep time.Time
}

func NewLoginLimiter(threshold int, window, lock time.Duration) *LoginLimiter {
	if threshold <= 0 {
		threshold = 3
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	if lock <= 0 {
		lock = 10 * time.Minute
	}
	return &LoginLimiter{
		entries:   map[string]*loginEntry{},
		threshold: threshold,
		window:    window,
		lock:      lock,
		lastSweep: time.Now(),
	}
}

func normalizeLoginKey(k string) string {
	return strings.ToLower(strings.TrimSpace(k))
}

// Allowed reports whether all keys are currently allowed. The duration is the
// longest remaining lockout.
func (l *LoginLimiter) Allowed(keys ...string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	allowed := true
	var remaining time.Duration
	for _, k := range keys {
		e := l.entries[normalizeLoginKey(k)]
		if e == nil || e.until.IsZero() {
			continue
		}
		if now.Before(e.until) {
			allowed = false
			if d := e.until.Sub(now); d > remaining {
				remaining = d
			}
		} else {
			delete(l.entries, normalizeLoginKey(k))
		}
	}
	l.sweepLocked(now)
	return allowed, remaining
}

func (l *LoginLimiter) Fail(keys ...string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		key := normalizeLoginKey(k)
		e := l.entries[key]
		if e == nil || (!e.first.IsZero() && now.Sub(e.first) > l.window) {
			e = &loginEntry{first: now}
			l.entries[key] = e
		}
		e.fails++
		e.last = now
		if e.fails >= l.threshold {
			e.until = now.Add(l.lock)
		}
	}
	l.sweepLocked(now)
}

func (l *LoginLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.entries, normalizeLoginKey(k))
	}
}

func (l *LoginLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < 5*time.Minute {
		return
	}
	for k, e := range l.entries {
		if now.Sub(e.last) > l.window+l.lock {
			delete(l.entries, k)
		}
	}
	l.lastSweep = now
}
