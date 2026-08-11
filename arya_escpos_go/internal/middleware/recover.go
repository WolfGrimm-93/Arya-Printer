package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"aryaescpos/internal/contract"
)

// Recover returns middleware that recovers panics from any downstream
// handler, logs the stack trace via logger, and responds with a generic
// 500 that never leaks the panic value or stack to the client.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"panic", rec,
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					writeAPIError(w, &contract.APIError{
						HTTPStatus: http.StatusInternalServerError,
						Code:       "internal_error",
						Message:    "internal server error",
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
