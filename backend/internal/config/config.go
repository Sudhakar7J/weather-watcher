// Package config loads runtime settings from the environment.
//
// Every value has a working default so the service starts with no configuration
// at all during local development, while remaining fully overridable in a
// deployed environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the resolved runtime settings.
type Config struct {
	// Addr is the TCP address the HTTP server listens on.
	Addr string

	// AllowedOrigins is the set of browser origins permitted to call the API.
	// The single entry "*" allows any origin.
	AllowedOrigins []string

	// CacheTTL is how long a weather report is served before the upstream API
	// is consulted again.
	CacheTTL time.Duration

	// RequestTimeout bounds the handling of a single inbound request.
	RequestTimeout time.Duration

	// ShutdownTimeout bounds how long graceful shutdown waits for in-flight
	// requests to drain.
	ShutdownTimeout time.Duration

	// MaxCities caps how many cities one request may ask for.
	MaxCities int
}

// Defaults applied when the corresponding environment variable is unset.
const (
	DefaultAddr            = ":8080"
	DefaultCacheTTL        = time.Hour
	DefaultRequestTimeout  = 15 * time.Second
	DefaultShutdownTimeout = 15 * time.Second
	DefaultMaxCities       = 20
)

// Load reads the configuration from the environment, applying defaults for
// anything unset. It returns an error if a value is present but unparseable,
// rather than silently falling back, so a typo in a deployment variable is
// caught at startup instead of at request time.
func Load() (Config, error) {
	cfg := Config{
		Addr:            normalizeAddr(getEnv("PORT", DefaultAddr)),
		AllowedOrigins:  parseOrigins(getEnv("ALLOWED_ORIGINS", "*")),
		CacheTTL:        DefaultCacheTTL,
		RequestTimeout:  DefaultRequestTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
		MaxCities:       DefaultMaxCities,
	}

	var err error
	if cfg.CacheTTL, err = getEnvDuration("CACHE_TTL", DefaultCacheTTL); err != nil {
		return Config{}, err
	}
	if cfg.RequestTimeout, err = getEnvDuration("REQUEST_TIMEOUT", DefaultRequestTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = getEnvDuration("SHUTDOWN_TIMEOUT", DefaultShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.MaxCities, err = getEnvInt("MAX_CITIES", DefaultMaxCities); err != nil {
		return Config{}, err
	}
	if cfg.MaxCities < 1 {
		return Config{}, fmt.Errorf("MAX_CITIES must be at least 1, got %d", cfg.MaxCities)
	}

	return cfg, nil
}

// normalizeAddr ensures the listen address carries a port.
//
// App Runner and most PaaS providers inject PORT as a bare number such as
// "8080". net.Listen accepts that form, but relying on it is surprising to read
// and breaks if a host is ever included, so the colon is made explicit.
func normalizeAddr(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return DefaultAddr
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

// parseOrigins splits a comma-separated origin list, trimming whitespace and
// dropping empty entries. An empty result means "no origins configured", which
// the CORS middleware treats as deny-by-default rather than allow-all.
func parseOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}

	// Accept Go durations ("30s", "2m") and bare seconds ("45").
	if d, err := time.ParseDuration(raw); err == nil {
		return d, nil
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	return 0, fmt.Errorf("%s: %q is not a valid duration (use e.g. 30s or 2m)", key, raw)
}

func getEnvInt(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a valid integer", key, raw)
	}
	return n, nil
}
