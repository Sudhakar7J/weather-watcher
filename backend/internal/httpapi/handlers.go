// Package httpapi exposes the weather service over HTTP.
//
// The API is deliberately small: a health check and a single endpoint that
// resolves a comma-separated list of cities. Everything else is transport
// concerns, handled here rather than in the domain packages.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

// Server holds the dependencies and settings for the HTTP handlers.
type Server struct {
	service        *weather.Service
	maxCities      int
	requestTimeout time.Duration
	logger         *slog.Logger
	startedAt      time.Time
}

// Option configures a Server at construction time.
type Option func(*Server)

// WithLogger sets the structured logger. A nil logger is ignored.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithMaxCities caps how many cities a single request may ask for.
func WithMaxCities(n int) Option {
	return func(s *Server) {
		if n > 0 {
			s.maxCities = n
		}
	}
}

// WithRequestTimeout bounds the handling of a single request.
func WithRequestTimeout(d time.Duration) Option {
	return func(s *Server) {
		if d > 0 {
			s.requestTimeout = d
		}
	}
}

// Defaults applied when the corresponding Option is not supplied.
const (
	defaultMaxCities      = 20
	defaultRequestTimeout = 15 * time.Second
)

// New builds the complete HTTP handler, including routing, CORS and access
// logging. The result is ready to hand to http.Server.
func New(svc *weather.Service, allowedOrigins []string, opts ...Option) http.Handler {
	s := &Server{
		service:        svc,
		maxCities:      defaultMaxCities,
		requestTimeout: defaultRequestTimeout,
		logger:         slog.Default(),
		startedAt:      time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()
	// Method-and-path patterns are matched by ServeMux itself, so a wrong
	// method yields 405 without a router dependency.
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/weather", s.handleWeather)

	// Visiting the bare host used to land on a bare 404, which reads as a
	// broken deployment rather than a missing route. Point it at the index of
	// real endpoints so the URL is useful to paste and to open.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "weather-watcher-api",
			"endpoints": []string{
				"/api/health",
				"/api/weather?cities=London,Tokyo",
			},
		})
	})

	var h http.Handler = mux
	h = logRequests(s.logger, h)
	// Outermost, so the headers are present on every response including the
	// CORS preflight and any error path.
	h = securityHeaders(h)
	h = cors(allowedOrigins, h)
	return h
}

// healthResponse is the body of GET /api/health.
type healthResponse struct {
	Status        string    `json:"status"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	CachedCities  int       `json:"cached_cities"`
	Time          time.Time `json:"time"`
}

// handleHealth reports liveness. It deliberately touches no upstream service,
// so it stays meaningful even when the weather provider is down.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:        "ok",
		UptimeSeconds: time.Since(s.startedAt).Seconds(),
		CachedCities:  s.service.CachedCities(),
		Time:          time.Now().UTC(),
	})
}

// cityError describes a single city that could not be resolved.
type cityError struct {
	City    string `json:"city"`
	Message string `json:"message"`
}

// weatherResponse is the body of GET /api/weather.
//
// Results and Errors are separate so that one bad city does not fail the whole
// request. Both are always non-nil so clients never have to guard against null.
type weatherResponse struct {
	Results []weather.Report `json:"results"`
	Errors  []cityError      `json:"errors"`

	RequestedAt time.Time `json:"requested_at"`
	CachedCount int       `json:"cached_count"`
	Total       int       `json:"total"`
}

// handleWeather resolves weather for every requested city.
//
// Cities are fetched concurrently because each may require an uncached
// upstream round trip. Results are written back into a pre-sized slice by
// index, so the response preserves the order the caller asked for regardless of
// which goroutine finishes first.
func (s *Server) handleWeather(w http.ResponseWriter, r *http.Request) {
	cities, err := parseCities(r.URL.Query().Get("cities"), s.maxCities)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	reports := make([]weather.Report, len(cities))
	failures := make([]cityError, len(cities))

	var wg sync.WaitGroup
	for i, city := range cities {
		wg.Add(1)
		go func(i int, city string) {
			defer wg.Done()

			report, err := s.service.Get(ctx, city)
			if err != nil {
				// Log the real cause; the client gets a safe message.
				s.logger.WarnContext(ctx, "city lookup failed",
					"city", city,
					"error", err.Error(),
				)
				failures[i] = cityError{City: city, Message: publicMessage(err)}
				return
			}
			reports[i] = report
		}(i, city)
	}
	wg.Wait()

	resp := weatherResponse{
		Results:     make([]weather.Report, 0, len(cities)),
		Errors:      make([]cityError, 0, len(cities)),
		RequestedAt: time.Now().UTC(),
		Total:       len(cities),
	}
	for i := range cities {
		if failures[i].City != "" {
			resp.Errors = append(resp.Errors, failures[i])
			continue
		}
		if reports[i].FromCache {
			resp.CachedCount++
		}
		resp.Results = append(resp.Results, reports[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

// parseCities validates and normalises the "cities" query parameter.
//
// Cities are trimmed, de-duplicated case-insensitively while preserving the
// caller's spelling and order, and rejected if the request exceeds the
// configured limit. De-duplication happens before the limit check so that
// "Singapore,Singapore,Singapore" is one city rather than three.
func parseCities(raw string, max int) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New(`the "cities" query parameter is required, e.g. /api/weather?cities=Singapore,Tokyo`)
	}

	parts := strings.Split(raw, ",")
	cities := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, p := range parts {
		city := strings.TrimSpace(p)
		if city == "" {
			continue
		}

		key := strings.ToLower(city)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		cities = append(cities, city)

		if len(cities) > max {
			return nil, fmt.Errorf("too many cities: at most %d may be requested at once", max)
		}
	}

	if len(cities) == 0 {
		return nil, errors.New(`no valid city names were provided in "cities"`)
	}
	return cities, nil
}

// publicMessage maps an internal error to text safe to return to a client.
// Upstream details stay in the logs rather than in the response body.
func publicMessage(err error) string {
	if errors.Is(err, weather.ErrCityNotFound) {
		return "unknown city"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out fetching weather data"
	}
	if errors.Is(err, context.Canceled) {
		return "request cancelled"
	}
	return "weather data is temporarily unavailable"
}

// errorResponse is the body returned for a request-level failure.
type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// writeJSON writes v as the response body with the given status.
//
// Encoding happens into a buffer first: if marshalling fails partway through a
// streamed write, a half-written body would already be on the wire and the
// status code could no longer be corrected.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("encode response", "error", err.Error())
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed to encode response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
