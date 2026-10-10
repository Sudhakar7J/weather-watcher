package config_test

import (
	"testing"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/config"
)

// clearEnv unsets every variable Load consults so a test starts from a known
// state regardless of the developer's shell.
//
// t.Setenv is used deliberately rather than t.Parallel: these tests mutate
// process-wide state and must not run concurrently.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"PORT", "ALLOWED_ORIGINS", "CACHE_TTL",
		"REQUEST_TIMEOUT", "SHUTDOWN_TIMEOUT", "MAX_CITIES",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Addr != config.DefaultAddr {
		t.Errorf("Addr = %q, want %q", cfg.Addr, config.DefaultAddr)
	}
	if cfg.CacheTTL != time.Hour {
		t.Errorf("CacheTTL = %v, want 1h", cfg.CacheTTL)
	}
	if cfg.MaxCities != config.DefaultMaxCities {
		t.Errorf("MaxCities = %d, want %d", cfg.MaxCities, config.DefaultMaxCities)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("AllowedOrigins = %v, want [*]", cfg.AllowedOrigins)
	}
}

func TestLoadPortOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
}

func TestLoadOriginParsing(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  []string
		count int
	}{
		{"single", "https://a.example.com", []string{"https://a.example.com"}, 1},
		{"multiple", "https://a.example.com,https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}, 2},
		{"trims whitespace", "  https://a.example.com , https://b.example.com  ", []string{"https://a.example.com", "https://b.example.com"}, 2},
		{"drops empty entries", "https://a.example.com,,", []string{"https://a.example.com"}, 1},
		{"blank means none", "   ", nil, 0},
		{"wildcard", "*", []string{"*"}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("ALLOWED_ORIGINS", tc.raw)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(cfg.AllowedOrigins) != tc.count {
				t.Fatalf("AllowedOrigins = %v, want %d entries", cfg.AllowedOrigins, tc.count)
			}
			for i, want := range tc.want {
				if cfg.AllowedOrigins[i] != want {
					t.Errorf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], want)
				}
			}
		})
	}
}

func TestLoadDurationParsing(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"go duration", "30m", 30 * time.Minute},
		{"seconds suffix", "90s", 90 * time.Second},
		{"bare seconds", "45", 45 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("CACHE_TTL", tc.raw)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CacheTTL != tc.want {
				t.Errorf("CacheTTL = %v, want %v", cfg.CacheTTL, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
		raw  string
	}{
		{"bad duration", "CACHE_TTL", "not-a-duration"},
		{"bad request timeout", "REQUEST_TIMEOUT", "soon"},
		{"bad shutdown timeout", "SHUTDOWN_TIMEOUT", "later"},
		{"non-numeric max cities", "MAX_CITIES", "many"},
		{"zero max cities", "MAX_CITIES", "0"},
		{"negative max cities", "MAX_CITIES", "-5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tc.key, tc.raw)

			// A typo in a deployment variable must fail loudly at startup
			// rather than silently falling back to a default.
			if _, err := config.Load(); err == nil {
				t.Fatalf("Load accepted %s=%q, want an error", tc.key, tc.raw)
			}
		})
	}
}

func TestLoadAcceptsEveryOverrideTogether(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", ":7000")
	t.Setenv("ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com")
	t.Setenv("CACHE_TTL", "15m")
	t.Setenv("REQUEST_TIMEOUT", "20s")
	t.Setenv("SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("MAX_CITIES", "5")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Addr != ":7000" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.CacheTTL != 15*time.Minute {
		t.Errorf("CacheTTL = %v", cfg.CacheTTL)
	}
	if cfg.RequestTimeout != 20*time.Second {
		t.Errorf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if cfg.ShutdownTimeout != 25*time.Second {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
	if cfg.MaxCities != 5 {
		t.Errorf("MaxCities = %d", cfg.MaxCities)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
}
