// Command lambda runs the Weather Watcher API as an AWS Lambda function.
//
// The HTTP layer is unchanged from the server build: this is a thin adapter
// that translates a Function URL request into an *http.Request, hands it to
// the same http.Handler the server uses, and writes the response back. Any
// behaviour fix therefore applies to both entrypoints at once.
package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
	"github.com/incubrix/weather-watcher/backend/internal/config"
	"github.com/incubrix/weather-watcher/backend/internal/httpapi"
	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config failed", "error", err)
		os.Exit(1)
	}

	client := weather.NewClient()
	cacheLayer := cache.New[weather.Report](cfg.CacheTTL)
	service := weather.NewService(client, cacheLayer)

	handler := httpapi.New(service, cfg.AllowedOrigins,
		httpapi.WithLogger(logger),
		httpapi.WithMaxCities(cfg.MaxCities),
		httpapi.WithRequestTimeout(cfg.RequestTimeout),
	)

	logger.Info("starting weather watcher lambda",
		"cache_ttl", cfg.CacheTTL.String(), "max_cities", cfg.MaxCities)

	lambda.Start(func(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
		return serve(handler, req)
	})
}

// capture collects the handler's response. httptest is deliberately not used
// so the adapter does not pull test-only machinery into the runtime image.
type capture struct {
	status int
	header http.Header
	body   []byte
}

func (c *capture) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}

func (c *capture) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	return len(b), nil
}

func (c *capture) WriteHeader(status int) { c.status = status }

func serve(handler http.Handler, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	target := req.RawPath
	if target == "" {
		target = "/"
	}
	// Function URL payload format 2.0 exposes the query as a decoded map
	// rather than a raw string, so it has to be re-encoded here.
	if raw := req.QueryStringParameters; len(raw) > 0 {
		q := url.Values{}
		for k, v := range raw {
			q.Set(k, v)
		}
		target += "?" + q.Encode()
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Body:       `{"error":"malformed request"}`,
			Headers:    jsonHeaders(),
		}, nil
	}

	method := req.RequestContext.HTTP.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader = http.NoBody
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	r := &http.Request{
		Method:        method,
		URL:           parsed,
		Header:        flatten(req.Headers),
		Body:          io.NopCloser(body),
		ContentLength: int64(len(req.Body)),
		RemoteAddr:    "lambda",
	}

	out := &capture{}
	handler.ServeHTTP(out, r)

	status := out.status
	if status == 0 {
		// The handler never called WriteHeader, which means 200 by default.
		status = http.StatusOK
	}

	headers := make(map[string]string, len(out.header))
	for k, v := range out.header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	// API Gateway and Lambda both add their own content type; setting it twice
	// produces a duplicate header, so force the value we actually sent.
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: status,
		Headers:    headers,
		Body:       string(out.body),
	}, nil
}

// flatten normalises header casing. The handler looks up canonical MIME keys,
// and a raw lowercased map would silently miss every lookup.
func flatten(in map[string]string) http.Header {
	out := http.Header{}
	for k, v := range in {
		out.Set(k, v)
	}
	return out
}

func jsonHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/json"}
}