package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder captures the status code written by a handler so it can be
// logged.
//
// It wraps http.ResponseWriter directly, which means the optional interfaces
// (http.Flusher, http.Hijacker, io.ReaderFrom) are not forwarded. That is
// acceptable here because every endpoint returns a small JSON body, but it
// would need revisiting before adding streaming or WebSocket support.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	// A handler that writes a body without an explicit WriteHeader implicitly
	// sent 200; record that so the log line is accurate.
	if !r.written {
		r.status = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

// logRequests emits one structured line per request.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
