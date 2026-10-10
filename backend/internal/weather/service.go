package weather

import (
	"context"
	"strings"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
)

// DefaultCacheTTL is how long a report stays valid before the upstream API is
// consulted again.
const DefaultCacheTTL = 1 * 60 * 60 // 1 hour

// Service resolves city names to reports, consulting the cache first.
type Service struct {
	client *Client
	cache  *cache.Cache[Report]
}

// NewService wires a client and a cache into a Service. A nil cache or client
// is replaced with a working default so the service is always usable.
func NewService(client *Client, c *cache.Cache[Report]) *Service {
	if client == nil {
		client = NewClient()
	}
	if c == nil {
		c = cache.New[Report](DefaultCacheTTL)
	}
	return &Service{client: client, cache: c}
}

// Get returns the report for city, loading it from the upstream API on a cache
// miss.
//
// Reports are stored in the cache by value. Returning the value (rather than a
// pointer) means the caller receives a copy, so stamping the FromCache field
// does not mutate the shared cached entry. The Hourly slice is shared with the
// cached entry and must only be read.
func (s *Service) Get(ctx context.Context, city string) (Report, error) {
	report, fromCache, err := s.cache.Get(ctx, CacheKey(city), func(ctx context.Context) (Report, error) {
		return s.client.Fetch(ctx, city)
	})
	if err != nil {
		return Report{}, err
	}

	report.FromCache = fromCache
	return report, nil
}

// Purge empties the cache, forcing the next request for every city to reload.
func (s *Service) Purge() { s.cache.Purge() }

// CachedCities reports how many reports are currently held. Exposed for the
// health endpoint.
func (s *Service) CachedCities() int { return s.cache.Len() }

// CacheKey normalises a city name so that lookups differing only by case or
// surrounding whitespace share a single cache entry.
func CacheKey(city string) string {
	return strings.ToLower(strings.TrimSpace(city))
}
