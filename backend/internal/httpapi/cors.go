package httpapi

import (
	"net/http"
	"strings"
)

// cors wraps a handler with Cross-Origin Resource Sharing headers.
//
// The frontend is served from a different origin than this API, so browsers
// require both an Access-Control-Allow-Origin response header and a successful
// preflight for anything beyond a simple GET. Both are handled here.
//
// Credentials are never allowed. Combined with the "*" default that rules out
// the combination browsers reject, and the API has nothing to authenticate
// anyway.
func cors(allowedOrigins []string, next http.Handler) http.Handler {
	allowAll, exact := parseOriginSet(allowedOrigins)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Behind an AWS Lambda Function URL the edge already applied its own
		// CORS headers. Emitting a second set produced a combined
		// "Access-Control-Allow-Origin: *,*" which browsers reject outright,
		// so every call failed even though both halves were individually
		// correct. Skipping here keeps a single owner of the header.
		if origin != "" && r.Header.Get("X-Amzn-Trace-Id") != "" {
			next.ServeHTTP(w, r)
			return
		}

		_, originAllowed := exact[origin]

		switch {
		case origin == "":
			// Not a browser cross-origin request (curl, health check,
			// same-origin). Nothing to negotiate.
		case allowAll:
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case originAllowed:
			w.Header().Set("Access-Control-Allow-Origin", origin)
			// The response varies by origin, so caches must not serve one
			// origin's response to another.
			w.Header().Add("Vary", "Origin")
		default:
			// Origin not permitted. The request still runs; it simply gets no
			// CORS headers, so the browser blocks the response reaching the
			// page. Not writing a header here is deliberate: writing a
			// mismatched origin would be worse.
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// parseOriginSet separates the wildcard from any explicit origins.
//
// The distinction matters: "*" is a constant response header, whereas an
// explicit allowlist varies per origin and therefore needs Vary: Origin.
func parseOriginSet(origins []string) (allowAll bool, exact map[string]struct{}) {
	exact = make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		switch o {
		case "":
			// Ignore blank entries so a trailing comma in a config variable
			// does not turn into a deny-all.
		case "*":
			allowAll = true
		default:
			exact[o] = struct{}{}
		}
	}
	return allowAll, exact
}
