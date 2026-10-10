// Package weather resolves a city name to current conditions and a short
// hourly trend using the free Open-Meteo API.
//
// Open-Meteo is used deliberately because it requires no API key. That removes
// any credential from the deployment: there is nothing to store in Secrets
// Manager, nothing to rotate, and nothing to leak in a log or a CI transcript.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ErrCityNotFound reports that the upstream geocoder did not recognise the
// requested city. Callers should surface this to the user as a 4xx rather than
// treating it as a server fault.
var ErrCityNotFound = errors.New("city not found")

const (
	defaultGeocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"
	defaultForecastURL = "https://api.open-meteo.com/v1/forecast"
	defaultTimeout     = 10 * time.Second
	defaultUserAgent   = "weather-watcher/1.0 (+https://github.com/incubrix/weather-watcher)"
	defaultTimeZone    = "UTC"

	// maxBodyBytes caps how much of an upstream response is read, so a
	// malfunctioning or hostile endpoint cannot exhaust memory.
	maxBodyBytes = 1 << 20

	// hourlyWindow is how many hours of trend data to request for the chart.
	hourlyWindow = 24
)

// Client fetches weather data from Open-Meteo.
//
// A Client is safe for concurrent use; the underlying http.Client is.
type Client struct {
	http        *http.Client
	geocodeURL  string
	forecastURL string
	userAgent   string
	now         func() time.Time
}

// Option configures a Client at construction time.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client, primarily for tests or for callers
// that need custom transport settings. A nil value is ignored.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// WithEndpoints overrides the upstream base URLs. Intended for tests that
// point at an httptest server.
func WithEndpoints(geocode, forecast string) Option {
	return func(c *Client) {
		if geocode != "" {
			c.geocodeURL = geocode
		}
		if forecast != "" {
			c.forecastURL = forecast
		}
	}
}

// WithClock overrides the time source, for deterministic tests.
func WithClock(fn func() time.Time) Option {
	return func(c *Client) {
		if fn != nil {
			c.now = fn
		}
	}
}

// NewClient returns a Client with sensible defaults for production use.
func NewClient(opts ...Option) *Client {
	c := &Client{
		http:        &http.Client{Timeout: defaultTimeout},
		geocodeURL:  defaultGeocodeURL,
		forecastURL: defaultForecastURL,
		userAgent:   defaultUserAgent,
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// geocodeResult is one candidate location from the geocoding endpoint.
type geocodeResult struct {
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
}

type geocodeResponse struct {
	Results []geocodeResult `json:"results"`
}

// forecastResponse mirrors the subset of the Open-Meteo forecast payload the
// application consumes.
type forecastResponse struct {
	Current struct {
		Time                string  `json:"time"`
		Temperature2m       float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		RelativeHumidity2m  float64 `json:"relative_humidity_2m"`
		WeatherCode         int     `json:"weather_code"`
		WindSpeed10m        float64 `json:"wind_speed_10m"`
	} `json:"current"`

	Hourly struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
	} `json:"hourly"`
}

// Fetch resolves city and returns its current conditions plus an hourly trend.
func (c *Client) Fetch(ctx context.Context, city string) (Report, error) {
	loc, err := c.geocode(ctx, city)
	if err != nil {
		return Report{}, err
	}
	return c.forecast(ctx, loc)
}

// geocode resolves a city name to coordinates.
func (c *Client) geocode(ctx context.Context, city string) (geocodeResult, error) {
	q := url.Values{}
	q.Set("name", city)
	q.Set("count", "1")
	q.Set("language", "en")
	q.Set("format", "json")

	var out geocodeResponse
	if err := c.getJSON(ctx, c.geocodeURL+"?"+q.Encode(), &out); err != nil {
		return geocodeResult{}, fmt.Errorf("geocode %q: %w", city, err)
	}

	// Open-Meteo answers an unknown place with HTTP 200 and no "results" key,
	// so an empty slice is the success path, not an error.
	if len(out.Results) == 0 {
		return geocodeResult{}, fmt.Errorf("%w: %q", ErrCityNotFound, city)
	}
	return out.Results[0], nil
}

// forecast retrieves current conditions and the hourly trend for a location.
func (c *Client) forecast(ctx context.Context, loc geocodeResult) (Report, error) {
	q := url.Values{}
	q.Set("latitude", formatFloat(loc.Latitude))
	q.Set("longitude", formatFloat(loc.Longitude))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m")
	q.Set("hourly", "temperature_2m")
	q.Set("forecast_hours", fmt.Sprint(hourlyWindow))
	q.Set("timezone", defaultTimeZone)

	var out forecastResponse
	if err := c.getJSON(ctx, c.forecastURL+"?"+q.Encode(), &out); err != nil {
		return Report{}, fmt.Errorf("forecast for %q: %w", loc.Name, err)
	}

	report := Report{
		City:      loc.Name,
		Country:   loc.Country,
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
		FetchedAt: c.now().UTC(),
		Current: Current{
			TemperatureC: out.Current.Temperature2m,
			FeelsLikeC:   out.Current.ApparentTemperature,
			HumidityPct:  out.Current.RelativeHumidity2m,
			WindSpeedKph: out.Current.WindSpeed10m,
			Code:         out.Current.WeatherCode,
			Description:  DescribeCode(out.Current.WeatherCode),
		},
	}

	observedAt, err := parseAPITime(out.Current.Time)
	if err != nil {
		return Report{}, fmt.Errorf("forecast for %q: current time: %w", loc.Name, err)
	}
	report.Current.ObservedAt = observedAt

	report.Hourly = buildHourly(out.Hourly.Time, out.Hourly.Temperature2m)

	return report, nil
}

// buildHourly pairs the hourly timestamps with temperatures.
//
// Open-Meteo returns two parallel arrays. If they ever disagree in length the
// extra entries are dropped rather than panicking; a truncated trend is far
// better than a crashed request.
func buildHourly(times []string, temps []float64) []HourlyPoint {
	n := min(len(times), len(temps))
	if n == 0 {
		return []HourlyPoint{}
	}

	points := make([]HourlyPoint, 0, n)
	for i := range n {
		t, err := parseAPITime(times[i])
		if err != nil {
			// Skip an unparseable stamp but keep the rest of the series.
			continue
		}
		points = append(points, HourlyPoint{Time: t, TemperatureC: temps[i]})
	}
	return points
}

// apiTimeLayout is the format Open-Meteo uses when timezone=UTC is requested:
// "2006-01-02T15:04", with no zone suffix.
const apiTimeLayout = "2006-01-02T15:04"

// parseAPITime parses an Open-Meteo timestamp as UTC.
func parseAPITime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(apiTimeLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", s, err)
	}
	return t, nil
}

// getJSON performs a GET request and decodes a JSON body into out.
func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	// Decode straight from a bounded reader rather than buffering the body.
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// formatFloat renders a coordinate without scientific notation, which some
// strict query parsers reject.
func formatFloat(f float64) string {
	return fmt.Sprintf("%.4f", f)
}
