package middleware

import (
	"net/http"
)

// CORS returns middleware that allows cross-origin requests from any origin,
// echoing back whatever Origin header the browser sent. This intentionally
// does NOT restrict by an origin whitelist — the real access control is
// internal/middleware.Auth's X-API-Key check, which every /api/v1/* route
// already requires. A page that doesn't know a given installation's API key
// cannot successfully call it regardless of its origin, so gating by origin
// on top of that added operational cost (every new client domain — often a
// domain this operator doesn't control at all — required editing
// configs/settings.yaml and restarting the service) without a meaningful
// security improvement over the API key alone. This is a deliberate
// trade-off, not an oversight: if a given installation's API key is ever
// leaked, an origin whitelist would have limited which sites could abuse it;
// without one, any site knowing the leaked key could. Rotate the key
// (regenerate secrets/apikey.key) if that ever happens.
//
// Behavior:
//   - If the request has no Origin header (same-origin, curl, server-to-server),
//     it is passed through untouched — CORS headers are a browser-enforced
//     concept and irrelevant here.
//   - If Origin is present, the standard CORS response headers are set,
//     echoing back that specific origin (never a bare "*", so the browser
//     will still expose the response to the calling page even though this
//     agent uses an API key header, not cookies, so credentials are never
//     enabled).
//   - OPTIONS preflight requests are answered directly (204); net/http's
//     ServeMux does not do this on its own.
//
// Access-Control-Allow-Private-Network is intentionally never emitted.
// Omitting it is the safer default for v1: it simply means a page served
// over HTTPS cannot fetch from this agent over HTTP without the user first
// visiting an HTTP page on this host, which is acceptable for a local
// printing agent and avoids resurrecting the PNA bypass finding.
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			reqHeaders := r.Header.Get("Access-Control-Request-Headers")
			if reqHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			} else {
				w.Header().Set("Access-Control-Allow-Headers", apiKeyHeader+", Content-Type")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
