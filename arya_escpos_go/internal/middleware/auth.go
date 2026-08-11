// Package middleware provides the HTTP middleware chain wrapped around
// internal/apiserver's mux: authentication, CORS, panic recovery, request
// body size limiting and access logging.
package middleware

import (
	"encoding/json"
	"net/http"

	"aryaescpos/internal/contract"
)

// apiKeyHeader is the header clients must send the API key in.
const apiKeyHeader = "X-API-Key"

// exemptPaths lists the routes that never require an API key, regardless
// of Security.AuthEnabled: the root info endpoint and the health check, so
// monitoring/liveness probes work without a key. Every other route,
// including all of /api/v1/*, is protected when auth is enabled.
var exemptPaths = map[string]bool{
	"/":       true,
	"/health": true,
}

// ValidateFunc compares a candidate API key against the configured one
// using a constant-time comparison (see internal/auth.Valid). It is passed
// in rather than importing internal/auth directly so this package stays
// easily testable with a fake.
type ValidateFunc func(candidate string) bool

// Auth returns middleware that requires header X-API-Key to match on every
// request, except GET / and GET /health, and except entirely when enabled
// is false (a development-only escape hatch driven by
// Security.AuthEnabled).
func Auth(enabled bool, validate ValidateFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || exemptPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(apiKeyHeader)
			if key == "" || !validate(key) {
				writeAPIError(w, contract.Unauthorized("missing or invalid API key"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeAPIError renders a *contract.APIError as {"detail": message} with
// its HTTP status, matching the shape apiserver uses for every other
// handler-level error so clients see one consistent error format
// regardless of which layer rejected the request.
func writeAPIError(w http.ResponseWriter, err *contract.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": err.Message})
}
