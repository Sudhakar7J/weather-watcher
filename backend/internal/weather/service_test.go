package weather_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

// newCountingService returns a Service backed by a stub upstream plus a
// counter of how many forecast requests actually reached it. A second counter
// tracks geocode calls.
func newCountingService(t *testing.T, forecastCalls, geoCalls *atomic.Int64) *weather.Service {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			if geoCalls != nil {
				geoCalls.Add(1)
			}
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		if forecastCalls != nil {
			forecastCalls.Add(1)
		}
		_, _ = w.Write([]byte(forecastBody))
	}))
	t.Cleanup(srv.Close)

	client := weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithEndpoints(srv.URL+"/geocode", srv.URL+"/forecast"),
	)
	return weather.NewService(client, cache.New[weather.Report](time.Hour))
}

func TestServiceFirstCallFetchesThenSubsequentCallsHitCache(t *testing.T) {
	t.Parallel()

	var forecastCalls atomic.Int64
	svc := newCountingService(t, &forecastCalls, nil)
	ctx := context.Background()

	first, err := svc.Get(ctx, "Singapore")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if first.FromCache {
		t.Error("first Get reported FromCache = true, want false")
	}

	second, err := svc.Get(ctx, "Singapore")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if !second.FromCache {
		t.Error("second Get reported FromCache = false, want true")
	}

	if n := forecastCalls.Load(); n != 1 {
		t.Errorf("upstream forecast called %d times, want 1", n)
	}
	if got := svc.CachedCities(); got != 1 {
		t.Errorf("CachedCities() = %d, want 1", got)
	}
}

// TestServiceDoesNotMutateCachedEntry guards the subtlety that makes the cache
// safe: Get stamps FromCache on a copy, never on the shared cached value. If
// it mutated the stored entry, a later reader would observe a corrupted field
// and concurrent readers would race.
func TestServiceDoesNotMutateCachedEntry(t *testing.T) {
	t.Parallel()

	var forecastCalls atomic.Int64
	svc := newCountingService(t, &forecastCalls, nil)
	ctx := context.Background()

	// Populate the cache.
	if _, err := svc.Get(ctx, "Singapore"); err != nil {
		t.Fatalf("priming Get: %v", err)
	}

	// Repeated hits must keep reporting FromCache = true, and the payload
	// must stay intact.
	for i := range 5 {
		got, err := svc.Get(ctx, "Singapore")
		if err != nil {
			t.Fatalf("hit %d: %v", i, err)
		}
		if !got.FromCache {
			t.Errorf("hit %d: FromCache = false, want true", i)
		}
		if got.Current.Description != "Overcast" {
			t.Errorf("hit %d: Description = %q, want %q (cached entry was corrupted)", i, got.Current.Description, "Overcast")
		}
		if got.FetchedAt.IsZero() {
			t.Errorf("hit %d: FetchedAt was lost", i)
		}
	}

	if n := forecastCalls.Load(); n != 1 {
		t.Errorf("upstream called %d times, want 1", n)
	}
}

func TestServiceCacheKeyIsNormalised(t *testing.T) {
	t.Parallel()

	var forecastCalls, geoCalls atomic.Int64
	svc := newCountingService(t, &forecastCalls, &geoCalls)
	ctx := context.Background()

	if _, err := svc.Get(ctx, "Singapore"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Different spelling, same cache entry.
	if _, err := svc.Get(ctx, "  singapore  "); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if n := forecastCalls.Load(); n != 1 {
		t.Errorf("upstream forecast called %d times, want 1 (variants should share a cache entry)", n)
	}
	if got := svc.CachedCities(); got != 1 {
		t.Errorf("CachedCities() = %d, want 1", got)
	}
}

// TestServiceCoalescesConcurrentGets exercises the cache and service together:
// many simultaneous requests for one cold city must produce one upstream call.
func TestServiceCoalescesConcurrentGets(t *testing.T) {
	t.Parallel()

	var forecastCalls atomic.Int64
	svc := newCountingService(t, &forecastCalls, nil)

	const goroutines = 32
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	start := make(chan struct{})
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = svc.Get(context.Background(), "Singapore")
		}()
	}
	close(start)
	wg.Wait()

	if n := forecastCalls.Load(); n != 1 {
		t.Errorf("upstream forecast called %d times, want 1", n)
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

func TestServiceDoesNotCacheFailures(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"generationtime_ms":0.9}`)) // unknown city
	}))
	t.Cleanup(srv.Close)

	svc := weather.NewService(
		weather.NewClient(
			weather.WithHTTPClient(srv.Client()),
			weather.WithEndpoints(srv.URL+"/geocode", srv.URL+"/forecast"),
		),
		cache.New[weather.Report](time.Hour),
	)

	for i := range 2 {
		if _, err := svc.Get(context.Background(), "Notarealcityxyz"); err == nil {
			t.Fatalf("attempt %d: Get succeeded, want ErrCityNotFound", i)
		}
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("upstream called %d times, want 2 (failures must not be cached)", n)
	}
	if got := svc.CachedCities(); got != 0 {
		t.Errorf("CachedCities() = %d, want 0", got)
	}
}

func TestServicePurgeForcesRefetch(t *testing.T) {
	t.Parallel()

	var forecastCalls atomic.Int64
	svc := newCountingService(t, &forecastCalls, nil)
	ctx := context.Background()

	if _, err := svc.Get(ctx, "Singapore"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	svc.Purge()
	if got := svc.CachedCities(); got != 0 {
		t.Fatalf("CachedCities() = %d after Purge, want 0", got)
	}

	report, err := svc.Get(ctx, "Singapore")
	if err != nil {
		t.Fatalf("Get after Purge: %v", err)
	}
	if report.FromCache {
		t.Error("FromCache = true after Purge, want false")
	}
	if n := forecastCalls.Load(); n != 2 {
		t.Errorf("upstream called %d times, want 2", n)
	}
}

func TestNewServiceToleratesNilDependencies(t *testing.T) {
	t.Parallel()

	// Defaults must be applied so a partially wired service still works.
	svc := weather.NewService(nil, nil)
	if svc == nil {
		t.Fatal("NewService(nil, nil) returned nil")
	}
	if got := svc.CachedCities(); got != 0 {
		t.Errorf("CachedCities() = %d, want 0", got)
	}
}
