// Package cache provides a small, thread-safe, TTL-based in-memory cache.
// It is designed for lightweight memoization of expensive queries (e.g.
// trending articles) where staleness of a few minutes is acceptable.
package cache

import (
	"sync"
	"time"
)

// Item is a cached value with its expiry.
type Item struct {
	Value   any
	Expires time.Time
}

// Cache is a TTL cache keyed by string.
type Cache struct {
	mu    sync.RWMutex
	items map[string]Item
	ttl   time.Duration
	now   func() time.Time
	stop  chan struct{}
}

// New creates a Cache. ttl <= 0 disables expiry (values live forever until
// evicted by Set/Delete or the janitor).
func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Cache{
		items: make(map[string]Item),
		ttl:   ttl,
		now:   time.Now,
		stop:  make(chan struct{}),
	}
}

// Get returns the cached value for key, if present and not expired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !item.Expires.IsZero() && c.now().After(item.Expires) {
		c.Delete(key)
		return nil, false
	}
	return item.Value, true
}

// Set stores value under key with the cache TTL.
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	c.items[key] = Item{Value: value, Expires: c.now().Add(c.ttl)}
	c.mu.Unlock()
}

// Delete removes key from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Start launches a background janitor that removes expired keys periodically.
func (c *Cache) Start() {
	go func() {
		t := time.NewTicker(c.ttl / 2)
		defer t.Stop()
		for {
			select {
			case <-c.stop:
				return
			case <-t.C:
				c.cleanup()
			}
		}
	}()
}

// Stop halts the background janitor.
func (c *Cache) Stop() {
	close(c.stop)
}

func (c *Cache) cleanup() {
	now := c.now()
	c.mu.Lock()
	for k, item := range c.items {
		if !item.Expires.IsZero() && now.After(item.Expires) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

// SetNow overrides the clock for tests.
func (c *Cache) SetNow(f func() time.Time) {
	c.mu.Lock()
	c.now = f
	c.mu.Unlock()
}

// Len returns the number of cached keys (for tests/inspection).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
