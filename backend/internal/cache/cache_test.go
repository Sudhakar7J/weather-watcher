package cache_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
)

// fakeClock is a manually advanced clock so TTL expiry can be tested without
// sleeping and without introducing flakiness.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestGetMissesThenHitsCache(t *testing.T) {
	t.Parallel()

	c := cache.New[string](time.Hour)
	var calls atomic.Int64

	load := func(context.Context) (string, error) {
		calls.Add(1)
		return "sunny", nil
	}

	got, fromCache, err := c.Get(context.Background(), "singapore", load)
	if err != nil {
		t.Fatalf("first Get: unexpected error: %v", err)
	}
	if fromCache {
		t.Error("first Get reported fromCache = true, want false")
	}
	if got != "sunny" {
		t.Errorf("first Get = %q, want %q", got, "sunny")
	}

	got, fromCache, err = c.Get(context.Background(), "singapore", load)
	if err != nil {
		t.Fatalf("second Get: unexpected error: %v", err)
	}
	if !fromCache {
		t.Error("second Get reported fromCache = false, want true")
	}
	if got != "sunny" {
		t.Errorf("second Get = %q, want %q", got, "sunny")
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("loader called %d times, want 1", n)
	}
}

func TestDistinctKeysAreCachedIndependently(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	ctx := context.Background()

	for _, key := range []string{"singapore", "tokyo", "london"} {
		_, fromCache, err := c.Get(ctx, key, func(context.Context) (int, error) { return 20, nil })
		if err != nil {
			t.Fatalf("Get(%q): unexpected error: %v", key, err)
		}
		if fromCache {
			t.Errorf("Get(%q): fromCache = true on first access", key)
		}
	}

	if got := c.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
}

// TestCoalescesConcurrentMisses is the central concurrency guarantee: when many
// goroutines request the same uncached key at once, the loader runs exactly
// once and they all share the result.
//
// The assertion is timing-independent. A goroutine either arrives while the
// load is in flight (it joins the flight) or after it finishes (it reads the
// cache). Either way the loader has already run once, so the count is 1. Only a
// genuine overlap between two loads would push it higher.
func TestCoalescesConcurrentMisses(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	var calls atomic.Int64

	load := func(context.Context) (int, error) {
		calls.Add(1)
		// Hold the load open so late arrivals queue behind it.
		time.Sleep(100 * time.Millisecond)
		return 42, nil
	}

	const goroutines = 64
	results := make([]int, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all goroutines together
			v, _, err := c.Get(context.Background(), "tokyo", load)
			results[i], errs[i] = v, err
		}()
	}
	close(start)
	wg.Wait()

	if n := calls.Load(); n != 1 {
		t.Errorf("loader called %d times, want 1: concurrent misses were not coalesced", n)
	}
	for i := range goroutines {
		if errs[i] != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, errs[i])
		}
		if results[i] != 42 {
			t.Errorf("goroutine %d: got %d, want 42: waiters did not share the result", i, results[i])
		}
	}
}

// TestDistinctKeysLoadConcurrently guards against the cache becoming a global
// serialisation point: requests for different keys must still run in parallel.
func TestDistinctKeysLoadConcurrently(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	const keys = 8

	var wg sync.WaitGroup
	var active, peak atomic.Int64

	for i := range keys {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("city-%d", i)
			_, _, _ = c.Get(context.Background(), key, func(context.Context) (int, error) {
				cur := active.Add(1)
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				active.Add(-1)
				return i, nil
			})
		}(i)
	}
	wg.Wait()

	if got := peak.Load(); got < 2 {
		t.Errorf("peak concurrent loads for distinct keys = %d, want > 1: cache is serialising unrelated keys", got)
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	var calls atomic.Int64

	wantErr := errors.New("upstream unavailable")
	failing := func(context.Context) (int, error) {
		calls.Add(1)
		return 0, wantErr
	}

	for i := range 2 {
		if _, _, err := c.Get(context.Background(), "london", failing); !errors.Is(err, wantErr) {
			t.Fatalf("attempt %d: got err = %v, want %v", i, err, wantErr)
		}
	}

	// A failure must not poison the key for the whole TTL.
	if n := calls.Load(); n != 2 {
		t.Errorf("loader called %d times, want 2: the error was cached", n)
	}

	// A subsequent success should be cached normally.
	if _, fromCache, err := c.Get(context.Background(), "london", func(context.Context) (int, error) {
		return 7, nil
	}); err != nil || fromCache {
		t.Errorf("recovery Get: fromCache = %v, err = %v; want false, nil", fromCache, err)
	}
	if _, fromCache, _ := c.Get(context.Background(), "london", func(context.Context) (int, error) {
		t.Error("value recovered after failure must be cached")
		return 0, nil
	}); !fromCache {
		t.Error("expected cached value after successful load")
	}
}

func TestEntryExpiresAfterTTL(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.New[string](time.Hour, cache.WithClock[string](clock.Now))

	load := func(context.Context) (string, error) { return "value", nil }

	if _, _, err := c.Get(context.Background(), "k", load); err != nil {
		t.Fatalf("initial Get: %v", err)
	}

	// Still fresh just before the TTL elapses.
	clock.Advance(59 * time.Minute)
	if _, fromCache, _ := c.Get(context.Background(), "k", load); !fromCache {
		t.Error("entry expired early: fromCache = false before the TTL elapsed")
	}

	// Expired once the TTL has passed.
	clock.Advance(2 * time.Minute)
	if _, fromCache, _ := c.Get(context.Background(), "k", load); fromCache {
		t.Error("entry served after expiry: fromCache = true past the TTL")
	}
}

func TestNonPositiveTTLDisablesCaching(t *testing.T) {
	t.Parallel()

	for _, ttl := range []time.Duration{0, -time.Second} {
		c := cache.New[int](ttl)
		var calls atomic.Int64
		load := func(context.Context) (int, error) {
			calls.Add(1)
			return 1, nil
		}

		for range 3 {
			if _, fromCache, err := c.Get(context.Background(), "k", load); err != nil || fromCache {
				t.Fatalf("ttl = %v: fromCache = %v, err = %v; want false, nil", ttl, fromCache, err)
			}
		}
		if n := calls.Load(); n != 3 {
			t.Errorf("ttl = %v: loader called %d times, want 3 (caching should be off)", ttl, n)
		}
		if got := c.Len(); got != 0 {
			t.Errorf("ttl = %v: Len() = %d, want 0 (nothing should be stored)", ttl, got)
		}
	}
}

func TestWaiterHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)

	release := make(chan struct{})
	started := make(chan struct{})

	ownerDone := make(chan struct{})
	go func() {
		defer close(ownerDone)
		_, _, _ = c.Get(context.Background(), "k", func(context.Context) (int, error) {
			close(started)
			<-release
			return 1, nil
		})
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, _, err := c.Get(ctx, "k", func(context.Context) (int, error) {
		t.Error("a waiting caller must not start its own load")
		return 0, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got err = %v, want context.DeadlineExceeded", err)
	}

	close(release)
	<-ownerDone
}

func TestDeletePurgeAndLen(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	ctx := context.Background()
	load := func(context.Context) (int, error) { return 1, nil }

	for _, k := range []string{"a", "b", "c"} {
		if _, _, err := c.Get(ctx, k, load); err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
	}
	if got := c.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}

	if !c.Delete("b") {
		t.Error("Delete(b) = false, want true")
	}
	if c.Delete("b") {
		t.Error("Delete(b) = true on second call, want false")
	}
	if got := c.Len(); got != 2 {
		t.Errorf("Len() = %d after Delete, want 2", got)
	}

	c.Purge()
	if got := c.Len(); got != 0 {
		t.Errorf("Len() = %d after Purge, want 0", got)
	}
	if _, fromCache, _ := c.Get(ctx, "a", load); fromCache {
		t.Error("value survived Purge")
	}
}

// TestDeletedKeyIsReloaded guards the invariant that deleting a key with a load
// still in flight does not leave the key permanently absent or stale.
func TestDeleteWhileLoadInFlightDoesNotCorruptState(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Hour)
	release := make(chan struct{})
	started := make(chan struct{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = c.Get(context.Background(), "k", func(context.Context) (int, error) {
			close(started)
			<-release
			return 99, nil
		})
	}()
	<-started

	// Delete finds nothing yet, because the value only lands on completion.
	if c.Delete("k") {
		t.Error("Delete returned true for a key whose load has not completed")
	}

	close(release)
	<-done

	// The completed load must have populated the cache.
	if _, fromCache, _ := c.Get(context.Background(), "k", func(context.Context) (int, error) {
		t.Error("expected the completed load to have populated the cache")
		return 0, nil
	}); !fromCache {
		t.Error("completed load did not populate the cache")
	}
}

func TestTTLAccessor(t *testing.T) {
	t.Parallel()

	if got := cache.New[int](90 * time.Minute).TTL(); got != 90*time.Minute {
		t.Errorf("TTL() = %v, want %v", got, 90*time.Minute)
	}
}
