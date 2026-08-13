package cache

import (
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	c := New(time.Minute)
	c.Set("k", 42)
	if v, ok := c.Get("k"); !ok || v != 42 {
		t.Errorf("Get = %v/%v, want 42/true", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Error("missing key should not be found")
	}
}

func TestCacheExpiry(t *testing.T) {
	c := New(time.Minute)
	now := time.Now()
	c.SetNow(func() time.Time { return now })
	c.Set("k", "v")
	if _, ok := c.Get("k"); !ok {
		t.Fatal("key should be present before expiry")
	}
	now = now.Add(61 * time.Second)
	if _, ok := c.Get("k"); ok {
		t.Error("key should be expired")
	}
}

func TestCacheOverwrite(t *testing.T) {
	c := New(time.Minute)
	c.Set("k", 1)
	c.Set("k", 2)
	if v, _ := c.Get("k"); v != 2 {
		t.Errorf("Get = %v, want 2 (overwritten)", v)
	}
}

func TestCacheCleanup(t *testing.T) {
	c := New(time.Minute)
	now := time.Now()
	c.SetNow(func() time.Time { return now })
	c.Set("a", 1)
	c.Set("b", 2)
	now = now.Add(2 * time.Minute)
	c.cleanup()
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0 after cleanup", c.Len())
	}
}
