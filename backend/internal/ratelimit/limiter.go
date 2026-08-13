// Package ratelimit provides a small, in-memory, thread-safe rate limiter
// keyed by arbitrary string (e.g. client IP). It is designed for light
// anti-abuse protection of public endpoints (view counting, search, etc.),
// not as a global production-grade gateway.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows up to max events per window per key, using a sliding
// window stored in memory. Old entries are pruned lazily on each call and
// on a background ticker when Start is used.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
	now    func() time.Time
	stop   chan struct{}
}

// New creates a Limiter. max <= 0 disables limiting (Allow always true).
func New(max int, window time.Duration) *Limiter {
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{
		max:    max,
		window: window,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
		stop:   make(chan struct{}),
	}
}

// Allow reports whether key is within its allowance. If not, it returns the
// remaining cooldown before the key can make another request.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	if l == nil || l.max <= 0 {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	// Drop entries outside the window.
	entries := l.hits[key]
	kept := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.hits[key] = kept

	if len(kept) >= l.max {
		oldest := kept[0]
		retryAfter = l.window - now.Sub(oldest)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	l.hits[key] = append(kept, now)
	return true, 0
}

// Start launches a background janitor that clears stale keys so the map does
// not grow unboundedly from many distinct IPs.
func (l *Limiter) Start() {
	if l == nil || l.max <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(l.window)
		defer t.Stop()
		for {
			select {
			case <-l.stop:
				return
			case <-t.C:
				l.cleanup()
			}
		}
	}()
}

// Stop halts the background janitor.
func (l *Limiter) Stop() {
	if l != nil {
		close(l.stop)
	}
}

func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-l.window)
	for k, entries := range l.hits {
		kept := entries[:0]
		for _, t := range entries {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = kept
		}
	}
}

// SetNow overrides the clock for tests.
func (l *Limiter) SetNow(f func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = f
}
