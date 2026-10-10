// Package cache implements a generic, concurrency-safe in-memory cache with
// TTL-based expiry and per-key request coalescing.
//
// The weather service uses it to avoid hammering the upstream weather API.
// Beyond a plain mutex-guarded map, the cache guarantees that for any given key
// at most one load runs at a time: concurrent callers that miss the cache share
// the result of a single execution rather than each issuing their own upstream
// request. Without this, a cold cache receiving N simultaneous requests for the
// same city would produce N identical calls ("thundering herd").
//
// Values are shared between every caller that receives them, so loaded values
// must be treated as immutable. Copying a struct value out of the cache is safe
// and is the intended usage; mutating memory that a value references (a slice
// element, a map entry, a pointee) is not.
package cache

import (
	"context"
	"sync"
	"time"
)

// Loader produces the value for a key. Get calls it on a cache miss.
//
// The context passed to Loader is the context of whichever caller won the race
// to perform the load. A cancellation there cancels the upstream work, and the
// error propagates to every caller waiting on that load.
type Loader[V any] func(ctx context.Context) (V, error)

// Option configures a Cache at construction time.
type Option[V any] func(*Cache[V])

// WithClock overrides the cache's clock so tests can simulate expiry without
// sleeping. It is exported for that purpose; production code should leave the
// default (time.Now).
func WithClock[V any](fn func() time.Time) Option[V] {
	return func(c *Cache[V]) {
		if fn != nil {
			c.now = fn
		}
	}
}

// entry is a cached value and the instant it stops being valid.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// flight is a single in-progress load for a key, shared by whoever is waiting
// on it. Once done is closed, value and err are final and safe to read.
type flight[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// Cache is a concurrency-safe map from string keys to values of type V, each
// valid for a fixed TTL.
//
// The zero value is not usable; construct one with New.
//
// Cache is safe for concurrent use by multiple goroutines.
type Cache[V any] struct {
	mu      sync.Mutex
	entries map[string]entry[V]
	flight  map[string]*flight[V]

	ttl time.Duration
	now func() time.Time
}

// New returns a Cache whose entries live for ttl. A non-positive ttl disables
// caching entirely: every Get calls the loader and returns its result.
func New[V any](ttl time.Duration, opts ...Option[V]) *Cache[V] {
	c := &Cache[V]{
		entries: make(map[string]entry[V]),
		flight:  make(map[string]*flight[V]),
		ttl:     ttl,
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// TTL reports the configured time-to-live.
func (c *Cache[V]) TTL() time.Duration { return c.ttl }

// Get returns the value for key, invoking load on a miss.
//
// The second return value reports whether the value came from the cache rather
// than from load; it is false for waiters that shared someone else's load.
//
// At most one execution of load runs per key at a time. Callers that arrive
// while a load is in progress wait for it and receive its result, so an
// upstream fetch happens once per key per TTL window.
//
// Failed loads are not cached, so a transient upstream error does not poison the
// key for the remainder of the TTL.
func (c *Cache[V]) Get(ctx context.Context, key string, load Loader[V]) (V, bool, error) {
	// Caching disabled: bypass entirely and never record a result.
	if c.ttl <= 0 {
		v, err := load(ctx)
		return v, false, err
	}

	if v, ok := c.lookup(key); ok {
		return v, true, nil
	}

	c.mu.Lock()

	// Re-check under the lock: a value may have landed between the lookup
	// above and acquiring the lock.
	if v, ok := c.lookupLocked(key); ok {
		c.mu.Unlock()
		return v, true, nil
	}

	if f, ok := c.flight[key]; ok {
		// A load for this key is already running. Join it instead of issuing a
		// duplicate upstream request.
		c.mu.Unlock()
		select {
		case <-f.done:
			// f.value and f.err were written before done was closed, so the
			// channel close provides the necessary happens-before edge.
			return f.value, false, f.err
		case <-ctx.Done():
			var zero V
			return zero, false, ctx.Err()
		}
	}

	f := &flight[V]{done: make(chan struct{})}
	c.flight[key] = f
	c.mu.Unlock()

	// This goroutine owns the load for this key.
	loaded, loadErr := load(ctx)

	c.mu.Lock()
	if loadErr == nil {
		c.entries[key] = entry[V]{value: loaded, expiresAt: c.now().Add(c.ttl)}
	}
	f.value, f.err = loaded, loadErr
	// Drop the flight before releasing the lock so that any caller arriving
	// afterwards either hits the cache or starts a new load, rather than
	// attaching to a finished one.
	delete(c.flight, key)
	close(f.done)
	c.mu.Unlock()

	return loaded, false, loadErr
}

// lookup reads a live entry, taking the lock.
func (c *Cache[V]) lookup(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lookupLocked(key)
}

// lookupLocked reads a live entry. The caller must hold c.mu.
func (c *Cache[V]) lookupLocked(key string) (V, bool) {
	e, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !c.now().Before(e.expiresAt) {
		// Expired: drop it so the map does not grow without bound.
		delete(c.entries, key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Delete removes a single key. It reports whether an entry was present,
// including one that had already expired but not yet been evicted.
func (c *Cache[V]) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entries[key]; !ok {
		return false
	}
	delete(c.entries, key)
	return true
}

// Purge empties the cache. In-flight loads are unaffected and will still
// populate the cache when they complete.
func (c *Cache[V]) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]entry[V])
}

// Len reports the number of entries currently held, including any that have
// expired but not yet been evicted by a lookup.
func (c *Cache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
