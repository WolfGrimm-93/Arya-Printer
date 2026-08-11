package middleware

import (
	"net/http"

	"aryaescpos/internal/contract"
)

// BodyLimit returns middleware that caps r.Body at maxBytes using
// http.MaxBytesReader, so a handler that calls r.Body.Read /
// r.ParseMultipartForm gets a clean error once the limit is exceeded
// instead of the process reading an unbounded upload into memory or disk.
//
// http.MaxBytesReader only makes the *read* fail past the limit; it cannot
// itself produce the 413 response (the handler's read fails first). To
// guarantee a 413 even for handlers that don't specifically check for a
// MaxBytesError, this also pre-checks Content-Length when the client sends
// one, rejecting oversized requests before any handler code runs.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeAPIError(w, contract.PayloadTooLarge("request body exceeds the configured upload limit"))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
