package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
	"github.com/incubrix/weather-watcher/backend/internal/httpapi"
	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

// stubUpstream serves the geocoding and forecast endpoints. geo resolves any
// city to a fixed coordinate unless the name is registered in knownCities.
func stubUpstream(t *testing.T, geoCalls *atomic.Int64) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/geocode") {
			if geoCalls != nil {
				geoCalls.Add(1)
			}
			name := r.URL.Query().Get("name")
			if name == "" || name == "Unknown" {
				// Open-Meteo signals an unknown place with 200 and no results.
				_, _ = w.Write([]byte(`{"generationtime_ms":0.9}`))
				return
			}
			body, _ := json.Marshal(map[string]any{
				"results": []map[string]any{{
					"name":      name,
					"latitude":  1.28967,
					"longitude": 103.85007,
					"country":   "Testland",
				}},
			})
			_, _ = w.Write(body)
			return
		}

		_, _ = w.Write([]byte(`{
			"current": {
				"time": "2026-10-10T04:45",
				"temperature_2m": 31.2,
				"apparent_temperature": 36.7,
				"relative_humidity_2m": 64,
				"weather_code": 3,
				"wind_speed_10m": 12.2
			},
			"hourly": {
				"time": ["2026-10-10T04:00", "2026-10-10T05:00"],
				"temperature_2m": [31.0, 28.6]
			}
		}`))
	}
}

// newTestHandler wires a full stack against a stub upstream.
func newTestHandler(t *testing.T, upstream http.HandlerFunc, origins []string, opts ...httpapi.Option) http.Handler {
	t.Helper()

	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)

	client := weather.NewClient(
		weather.WithHTTPClient(srv.Client()),
		weather.WithEndpoints(srv.URL+"/geocode", srv.URL+"/forecast"),
	)
	svc := weather.NewService(client, cache.New[weather.Report](time.Hour))

	// Discard handler logs so test output stays readable.
	base := httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return httpapi.New(svc, origins, append([]httpapi.Option{base}, opts...)...)
}

func doGet(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// decodeBody unmarshals a JSON response body into v.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

type weatherBody struct {
	Results []struct {
		City      string `json:"city"`
		FromCache bool   `json:"from_cache"`
		Hourly    []any  `json:"hourly"`
	} `json:"results"`
	Errors []struct {
		City    string `json:"city"`
		Message string `json:"message"`
	} `json:"errors"`
	CachedCount int `json:"cached_count"`
	Total       int `json:"total"`
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	rec := doGet(t, h, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Status        string  `json:"status"`
		UptimeSeconds float64 `json:"uptime_seconds"`
		CachedCities  int     `json:"cached_cities"`
	}
	decodeBody(t, rec, &body)

	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds = %v, want >= 0", body.UptimeSeconds)
	}
}

func TestHealthWorksWhenUpstreamIsDown(t *testing.T) {
	t.Parallel()

	// Liveness must not depend on the weather provider being reachable.
	h := newTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}, []string{"*"})

	if rec := doGet(t, h, "/api/health"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 even when the upstream is failing", rec.Code)
	}
}

func TestWeatherReturnsResults(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	rec := doGet(t, h, "/api/weather?cities=Singapore,Tokyo,London")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}

	var body weatherBody
	decodeBody(t, rec, &body)

	if len(body.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(body.Results))
	}
	if len(body.Errors) != 0 {
		t.Errorf("len(Errors) = %d, want 0", len(body.Errors))
	}
	if body.Total != 3 {
		t.Errorf("total = %d, want 3", body.Total)
	}

	// Results must come back in the order the caller asked for, regardless of
	// which goroutine finished first.
	want := []string{"Singapore", "Tokyo", "London"}
	for i, w := range want {
		if body.Results[i].City != w {
			t.Errorf("Results[%d].City = %q, want %q", i, body.Results[i].City, w)
		}
		if len(body.Results[i].Hourly) != 2 {
			t.Errorf("Results[%d] has %d hourly points, want 2", i, len(body.Results[i].Hourly))
		}
	}
}

func TestWeatherSecondRequestIsServedFromCache(t *testing.T) {
	t.Parallel()

	var geoCalls atomic.Int64
	h := newTestHandler(t, stubUpstream(t, &geoCalls), []string{"*"})

	first := doGet(t, h, "/api/weather?cities=Singapore")
	var firstBody weatherBody
	decodeBody(t, first, &firstBody)
	if firstBody.CachedCount != 0 {
		t.Errorf("first request cached_count = %d, want 0", firstBody.CachedCount)
	}

	second := doGet(t, h, "/api/weather?cities=Singapore")
	var secondBody weatherBody
	decodeBody(t, second, &secondBody)
	if secondBody.CachedCount != 1 {
		t.Errorf("second request cached_count = %d, want 1", secondBody.CachedCount)
	}
	if n := geoCalls.Load(); n != 1 {
		t.Errorf("upstream geocoded %d times, want 1", n)
	}
}

func TestWeatherRequiresCitiesParameter(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	for _, target := range []string{"/api/weather", "/api/weather?cities=", "/api/weather?cities=%20%2C%20"} {
		rec := doGet(t, h, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestWeatherRejectsTooManyCities(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"}, httpapi.WithMaxCities(3))

	var sb strings.Builder
	for i := range 5 {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("City")
		sb.WriteString(string(rune('A' + i)))
	}

	rec := doGet(t, h, "/api/weather?cities="+sb.String())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "at most 3") {
		t.Errorf("body %q does not explain the limit", rec.Body.String())
	}
}

func TestWeatherDeduplicatesCities(t *testing.T) {
	t.Parallel()

	var geoCalls atomic.Int64
	h := newTestHandler(t, stubUpstream(t, &geoCalls), []string{"*"})

	// Repeats differing only by case and padding are one city, and must not
	// count against the per-request limit.
	rec := doGet(t, h, "/api/weather?cities=Singapore,singapore,%20Singapore%20,Tokyo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}

	var body weatherBody
	decodeBody(t, rec, &body)
	if len(body.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2 after de-duplication", len(body.Results))
	}
	if n := geoCalls.Load(); n != 2 {
		t.Errorf("upstream geocoded %d times, want 2", n)
	}
}

func TestWeatherIsolatesPerCityFailures(t *testing.T) {
	t.Parallel()

	// One unknown city must not blank the whole dashboard.
	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	rec := doGet(t, h, "/api/weather?cities=Singapore,Unknown,Tokyo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even with a bad city: body=%s", rec.Code, rec.Body.String())
	}

	var body weatherBody
	decodeBody(t, rec, &body)

	if len(body.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2 (the good cities must survive)", len(body.Results))
	}
	if len(body.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(body.Errors))
	}
	if body.Errors[0].City != "Unknown" {
		t.Errorf("Errors[0].City = %q, want %q", body.Errors[0].City, "Unknown")
	}
	if body.Errors[0].Message != "unknown city" {
		t.Errorf("Errors[0].Message = %q, want %q", body.Errors[0].Message, "unknown city")
	}
}

func TestWeatherErrorsDoNotLeakInternals(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		// An upstream failure detail that must not reach the client.
		http.Error(w, "connection to 10.0.0.5 refused: secret-token-abc", http.StatusBadGateway)
	}, []string{"*"})

	rec := doGet(t, h, "/api/weather?cities=Singapore")
	body := rec.Body.String()

	if strings.Contains(body, "secret-token-abc") || strings.Contains(body, "10.0.0.5") {
		t.Errorf("response leaked upstream internals: %s", body)
	}
	if !strings.Contains(body, "temporarily unavailable") {
		t.Errorf("response %q does not contain a safe user-facing message", body)
	}
}

func TestEmptyArraysAreSerialisedNotNull(t *testing.T) {
	t.Parallel()

	// Nothing resolves, so Results must be [] and not null, or clients have
	// to guard every access.
	h := newTestHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"generationtime_ms":0.9}`))
	}, []string{"*"})

	rec := doGet(t, h, "/api/weather?cities=Unknown")
	body := rec.Body.String()

	if strings.Contains(body, `"results":null`) {
		t.Errorf("results serialised as null: %s", body)
	}
	if !strings.Contains(body, `"results":[]`) {
		t.Errorf("results not serialised as []: %s", body)
	}
	if !strings.Contains(body, `"errors":[`) {
		t.Errorf("errors array missing: %s", body)
	}
}

func TestUnsupportedMethodReturns405(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/weather?cities=Singapore", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	if rec := doGet(t, h, "/api/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCORSWildcardAllowsAnyOrigin(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/weather?cities=Singapore", nil)
	req.Header.Set("Origin", "https://dashboard.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
}

func TestCORSAllowlistEchoesMatchingOrigin(t *testing.T) {
	t.Parallel()

	const allowed = "https://dashboard.example.com"
	h := newTestHandler(t, stubUpstream(t, nil), []string{allowed})

	req := httptest.NewRequest(http.MethodGet, "/api/weather?cities=Singapore", nil)
	req.Header.Set("Origin", allowed)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, allowed)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Error("an origin-dependent response must set Vary: Origin so caches do not cross-serve origins")
	}
}

func TestCORSOmitsHeaderForUnlistedOrigin(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"https://allowed.example.com"})

	req := httptest.NewRequest(http.MethodGet, "/api/weather?cities=Singapore", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// The request still runs; the browser is what enforces the policy, by
	// refusing a response that lacks the header.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for an unlisted origin", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	req := httptest.NewRequest(http.MethodOptions, "/api/weather", nil)
	req.Header.Set("Origin", "https://dashboard.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Errorf("Access-Control-Allow-Methods = %q, want it to include GET", got)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("preflight returned a body: %q", rec.Body.String())
	}
}

func TestCORSWithoutOriginIsUnaffected(t *testing.T) {
	t.Parallel()

	// Health checks and curl send no Origin; nothing should break.
	h := newTestHandler(t, stubUpstream(t, nil), []string{"https://only-this.example.com"})

	if rec := doGet(t, h, "/api/health"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestCitiesQueryIsURLDecoded(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	// "New York" must survive round-tripping through the query string.
	target := "/api/weather?cities=" + url.QueryEscape("New York")
	rec := doGet(t, h, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: body=%s", rec.Code, rec.Body.String())
	}

	var body weatherBody
	decodeBody(t, rec, &body)
	if len(body.Results) != 1 || body.Results[0].City != "New York" {
		t.Errorf("Results = %+v, want a single New York entry", body.Results)
	}
}

// Regression: behind a Lambda Function URL the edge sets its own CORS headers.
// If the app also emitted its own, the two combined into a single
// "*,*" header value and browsers rejected every response, which presented to
// the user as an unreachable backend rather than as a CORS failure.
func TestCORSDefersToLambdaFunctionURL(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, stubUpstream(t, nil), []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "https://dashboard.example.com")
	req.Header.Set("X-Amzn-Trace-Id", "Root=1-000-000")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Values("Access-Control-Allow-Origin"); len(got) != 0 {
		t.Fatalf("Access-Control-Allow-Origin = %q, want none when the edge already set it", got)
	}
}
