package weather_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

const (
	geocodeBody = `{
		"results": [{
			"id": 1880252,
			"name": "Singapore",
			"latitude": 1.28967,
			"longitude": 103.85007,
			"country": "Singapore",
			"country_code": "SG"
		}],
		"generationtime_ms": 0.96
	}`

	forecastBody = `{
		"current": {
			"time": "2026-10-10T04:45",
			"interval": 900,
			"temperature_2m": 31.2,
			"apparent_temperature": 36.7,
			"relative_humidity_2m": 64,
			"weather_code": 3,
			"wind_speed_10m": 12.2
		},
		"hourly": {
			"time": ["2026-10-10T04:00", "2026-10-10T05:00", "2026-10-10T06:00"],
			"temperature_2m": [31.0, 31.0, 28.6]
		}
	}`
)

// newTestClient points a Client at a local test server. The server is closed
// automatically when the test finishes.
func newTestClient(t *testing.T, handler http.Handler) *weather.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithEndpoints(srv.URL+"/geocode", srv.URL+"/forecast"),
	)
}

// okHandler serves the canned successful payloads.
func okHandler(t *testing.T) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/geocode"):
			_, _ = w.Write([]byte(geocodeBody))
		case strings.HasSuffix(r.URL.Path, "/forecast"):
			_, _ = w.Write([]byte(forecastBody))
		default:
			http.NotFound(w, r)
		}
	})
}

func TestFetchMapsUpstreamFields(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, okHandler(t))

	report, err := client.Fetch(context.Background(), "Singapore")
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}

	if report.City != "Singapore" {
		t.Errorf("City = %q, want %q", report.City, "Singapore")
	}
	if report.Country != "Singapore" {
		t.Errorf("Country = %q, want %q", report.Country, "Singapore")
	}
	if math.Abs(report.Latitude-1.28967) > 1e-5 {
		t.Errorf("Latitude = %v, want 1.28967", report.Latitude)
	}
	if math.Abs(report.Longitude-103.85007) > 1e-5 {
		t.Errorf("Longitude = %v, want 103.85007", report.Longitude)
	}

	cur := report.Current
	if math.Abs(cur.TemperatureC-31.2) > 1e-9 {
		t.Errorf("TemperatureC = %v, want 31.2", cur.TemperatureC)
	}
	if math.Abs(cur.FeelsLikeC-36.7) > 1e-9 {
		t.Errorf("FeelsLikeC = %v, want 36.7", cur.FeelsLikeC)
	}
	if math.Abs(cur.HumidityPct-64) > 1e-9 {
		t.Errorf("HumidityPct = %v, want 64", cur.HumidityPct)
	}
	if math.Abs(cur.WindSpeedKph-12.2) > 1e-9 {
		t.Errorf("WindSpeedKph = %v, want 12.2", cur.WindSpeedKph)
	}
	if cur.Code != 3 {
		t.Errorf("Code = %d, want 3", cur.Code)
	}
	if cur.Description != "Overcast" {
		t.Errorf("Description = %q, want %q", cur.Description, "Overcast")
	}

	wantObserved := time.Date(2026, 10, 10, 4, 45, 0, 0, time.UTC)
	if !cur.ObservedAt.Equal(wantObserved) {
		t.Errorf("ObservedAt = %v, want %v", cur.ObservedAt, wantObserved)
	}

	if len(report.Hourly) != 3 {
		t.Fatalf("len(Hourly) = %d, want 3", len(report.Hourly))
	}
	if got, want := report.Hourly[0].Time.UTC().Format(time.RFC3339), "2026-10-10T04:00:00Z"; got != want {
		t.Errorf("Hourly[0].Time = %s, want %s", got, want)
	}
	if math.Abs(report.Hourly[2].TemperatureC-28.6) > 1e-9 {
		t.Errorf("Hourly[2].TemperatureC = %v, want 28.6", report.Hourly[2].TemperatureC)
	}

	if report.FetchedAt.IsZero() {
		t.Error("FetchedAt was not set")
	}
	if report.FromCache {
		t.Error("FromCache = true on a direct client fetch")
	}
}

func TestFetchSendsExpectedQueryParameters(t *testing.T) {
	t.Parallel()

	var geoQuery, forecastQuery string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			geoQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		forecastQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(forecastBody))
	}))

	if _, err := client.Fetch(context.Background(), "Singapore"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// The chart needs a full 24-hour window in UTC.
	for _, want := range []string{"forecast_hours=24", "timezone=UTC"} {
		if !strings.Contains(forecastQuery, want) {
			t.Errorf("forecast query %q missing %q", forecastQuery, want)
		}
	}
	for _, want := range []string{"temperature_2m", "relative_humidity_2m", "weather_code", "wind_speed_10m"} {
		if !strings.Contains(forecastQuery, want) {
			t.Errorf("forecast query %q missing current field %q", forecastQuery, want)
		}
	}
	if !strings.Contains(geoQuery, "name=Singapore") {
		t.Errorf("geocode query %q missing city name", geoQuery)
	}
}

func TestFetchUnknownCity(t *testing.T) {
	t.Parallel()

	// Open-Meteo answers an unknown place with HTTP 200 and no "results" key.
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"generationtime_ms":0.99}`))
	}))

	_, err := client.Fetch(context.Background(), "Notarealcityxyz")
	if !errors.Is(err, weather.ErrCityNotFound) {
		t.Fatalf("Fetch error = %v, want ErrCityNotFound", err)
	}
}

func TestFetchEmptyResultsArrayIsNotFound(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))

	if _, err := client.Fetch(context.Background(), "Nowhere"); !errors.Is(err, weather.ErrCityNotFound) {
		t.Fatalf("Fetch error = %v, want ErrCityNotFound", err)
	}
}

func TestFetchUpstreamErrorStatus(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	_, err := client.Fetch(context.Background(), "Singapore")
	if err == nil {
		t.Fatal("Fetch succeeded, want an error for a 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not mention the upstream status", err)
	}
}

func TestFetchMalformedJSON(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [ this is not json`))
	}))

	if _, err := client.Fetch(context.Background(), "Singapore"); err == nil {
		t.Fatal("Fetch succeeded, want a decode error")
	}
}

func TestFetchInvalidCurrentTimestamp(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		_, _ = w.Write([]byte(`{"current":{"time":"not-a-timestamp","temperature_2m":20}}`))
	}))

	if _, err := client.Fetch(context.Background(), "Singapore"); err == nil {
		t.Fatal("Fetch succeeded, want a timestamp parse error")
	}
}

func TestFetchTruncatesMismatchedHourlyArrays(t *testing.T) {
	t.Parallel()

	// Three timestamps but two temperatures. The extra timestamp must be
	// dropped rather than causing an index-out-of-range panic.
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		_, _ = w.Write([]byte(`{
			"current": {"time": "2026-10-10T04:45", "temperature_2m": 20},
			"hourly": {
				"time": ["2026-10-10T04:00", "2026-10-10T05:00", "2026-10-10T06:00"],
				"temperature_2m": [19.5, 20.5]
			}
		}`))
	}))

	report, err := client.Fetch(context.Background(), "Singapore")
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if len(report.Hourly) != 2 {
		t.Errorf("len(Hourly) = %d, want 2", len(report.Hourly))
	}
}

func TestFetchSkipsUnparseableHourlyTimestamps(t *testing.T) {
	t.Parallel()

	// One bad stamp in the middle of the series must not discard the rest.
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		_, _ = w.Write([]byte(`{
			"current": {"time": "2026-10-10T04:45", "temperature_2m": 20},
			"hourly": {
				"time": ["2026-10-10T04:00", "garbage", "2026-10-10T06:00"],
				"temperature_2m": [19.5, 20.0, 20.5]
			}
		}`))
	}))

	report, err := client.Fetch(context.Background(), "Singapore")
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if len(report.Hourly) != 2 {
		t.Fatalf("len(Hourly) = %d, want 2 (the bad stamp should be skipped)", len(report.Hourly))
	}
	if got, want := report.Hourly[1].Time.UTC().Format(time.RFC3339), "2026-10-10T06:00:00Z"; got != want {
		t.Errorf("Hourly[1].Time = %s, want %s", got, want)
	}
}

func TestFetchEmptyHourlyIsNotNil(t *testing.T) {
	t.Parallel()

	// A nil slice would marshal to JSON null and break the frontend chart.
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/geocode") {
			_, _ = w.Write([]byte(geocodeBody))
			return
		}
		_, _ = w.Write([]byte(`{"current":{"time":"2026-10-10T04:45","temperature_2m":20}}`))
	}))

	report, err := client.Fetch(context.Background(), "Singapore")
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if report.Hourly == nil {
		t.Fatal("Hourly is nil; it must be an empty slice so JSON renders [] not null")
	}
	if len(report.Hourly) != 0 {
		t.Errorf("len(Hourly) = %d, want 0", len(report.Hourly))
	}
}

func TestFetchHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(geocodeBody))
	}))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := client.Fetch(ctx, "Singapore")
	close(release)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch error = %v, want context.Canceled", err)
	}
}

func TestNewClientDefaultsAreSane(t *testing.T) {
	t.Parallel()

	// Nil arguments must be ignored rather than panicking or producing a
	// client that cannot make requests.
	c := weather.NewClient(
		weather.WithHTTPClient(nil),
		weather.WithEndpoints("", ""),
		weather.WithClock(nil),
	)
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}
