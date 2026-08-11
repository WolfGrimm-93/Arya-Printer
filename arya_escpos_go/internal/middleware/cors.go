package middleware

import (
	"net/http"
)

// CORS returns middleware that enforces a real origin whitelist
// (Security.AllowedOrigins), unlike the Python service, which set
// allow_origins=["*"] and unconditionally forced
// Access-Control-Allow-Private-Network: true on every response — the
// critical audit finding that let any browser tab print to any device
// reachable from this agent.
//
// Behavior:
//   - If the request has no Origin header (same-origin, curl, server-to-server),
//     it is passed through untouched — CORS headers are a browser-enforced
//     concept and irrelevant here.
//   - If Origin is present and NOT in allowedOrigins, no CORS headers are
//     added at all. The request still reaches the handler (Go's stdlib
//     http.ServeMux has no notion of blocking a request outright), but the
//     browser will refuse to expose the response to the calling page
//     because Access-Control-Allow-Origin is absent — this is what actually
//     stops the cross-origin read, the same mechanism the Python service
//     defeated by allowing "*".
//   - If Origin is present and IS in allowedOrigins, the standard CORS
//     response headers are set, echoing back that specific origin (never
//     "*"), with credentials disabled (this agent uses an API key header,
//     not cookies).
//   - OPTIONS preflight requests are answered directly (204) when the
//     origin is allowed; net/http's ServeMux does not do this on its own.
//
// Access-Control-Allow-Private-Network is intentionally never emitted.
// Omitting it is the safer default for v1: it simply means a page served
// over HTTPS cannot fetch from this agent over HTTP without the user first
// visiting an HTTP page on this host, which is acceptable for a local
// printing agent and avoids resurrecting the PNA bypass finding.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" || !allowed[origin] {
				if r.Method == http.MethodOptions {
					// No CORS headers for a disallowed/absent origin: the
					// browser's own preflight check will fail closed.
					w.WriteHeader(http.StatusNoContent)
					return
				}
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
