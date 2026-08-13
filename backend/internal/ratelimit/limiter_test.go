package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowsUpToMax(t *testing.T) {
	l := New(3, time.Minute)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("ip-1"); !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if ok, _ := l.Allow("ip-1"); ok {
		t.Error("4th request should be denied")
	}
	// Different key unaffected.
	if ok, _ := l.Allow("ip-2"); !ok {
		t.Error("different key should be allowed")
	}
}

func TestLimiterSlidingWindow(t *testing.T) {
	l := New(2, time.Minute)
	now := time.Now()
	l.SetNow(func() time.Time { return now })

	for i := 0; i < 2; i++ {
		l.Allow("ip")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("expected denied within window")
	}

	// Advance past window -> should be allowed again.
	now = now.Add(61 * time.Second)
	if ok, _ := l.Allow("ip"); !ok {
		t.Error("expected allowed after window elapsed")
	}
}

func TestLimiterRetryAfter(t *testing.T) {
	l := New(1, 10*time.Second)
	now := time.Now()
	l.SetNow(func() time.Time { return now })
	l.Allow("ip")

	ok, retry := l.Allow("ip")
	if ok {
		t.Fatal("expected denied")
	}
	if retry <= 0 || retry > 10*time.Second {
		t.Errorf("retryAfter = %v, want (0, 10s]", retry)
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := New(0, time.Minute)
	for i := 0; i < 100; i++ {
		if ok, _ := l.Allow("any"); !ok {
			t.Fatal("disabled limiter must always allow")
		}
	}
}

func TestLimiterCleanup(t *testing.T) {
	l := New(1, time.Minute)
	now := time.Now()
	l.SetNow(func() time.Time { return now })
	l.Allow("old")
	l.Allow("stale")

	now = now.Add(2 * time.Minute)
	l.cleanup()
	if ok, _ := l.Allow("old"); !ok {
		t.Error("expected allowed after cleanup")
	}
}
