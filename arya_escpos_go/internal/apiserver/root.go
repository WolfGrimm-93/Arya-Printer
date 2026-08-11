package apiserver

import "net/http"

// serviceVersion is reported by GET / — bumped independently from the
// Python service's version string since this is a full rewrite.
// var (not const) so CI can override it at build time via
// -ldflags "-X 'aryaescpos/internal/apiserver.serviceVersion=X.Y.Z'";
// local/dev builds without that flag keep this fallback.
var serviceVersion = "2.0.0"

func handleRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "Arya ESCPOS",
		"version": serviceVersion,
		"status":  "running",
		"docs":    "/docs",
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
