package weather_test

import (
	"testing"

	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

func TestDescribeCodeKnown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{3, "Overcast"},
		{45, "Fog"},
		{61, "Slight rain"},
		{95, "Thunderstorm"},
		{99, "Thunderstorm with heavy hail"},
	}

	for _, tc := range tests {
		if got := weather.DescribeCode(tc.code); got != tc.want {
			t.Errorf("DescribeCode(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestDescribeCodeUnknownNeverEmpty(t *testing.T) {
	t.Parallel()

	// A code outside the WMO table must still yield something renderable
	// rather than an empty label in the UI.
	got := weather.DescribeCode(1234)
	if got == "" {
		t.Fatal("DescribeCode(1234) returned an empty string")
	}
	if want := "Unknown conditions (code 1234)"; got != want {
		t.Errorf("DescribeCode(1234) = %q, want %q", got, want)
	}
}

func TestCacheKeyNormalisation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"Singapore", "singapore"},
		{"  Tokyo  ", "tokyo"},
		{"LONDON", "london"},
		{"New York", "new york"},
		{"", ""},
	}

	for _, tc := range tests {
		if got := weather.CacheKey(tc.in); got != tc.want {
			t.Errorf("CacheKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// The point of normalisation: spellings that differ only by case or
	// padding must collapse to one key.
	if weather.CacheKey("Singapore") != weather.CacheKey("  singapore ") {
		t.Error("case and whitespace variants produced different cache keys")
	}
}
