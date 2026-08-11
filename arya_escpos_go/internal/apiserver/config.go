package apiserver

import "net/http"

// handleConfig is GET /api/v1/config. It returns whatever ConfigView main.go
// injected into Deps — a JSON-serializable snapshot of config.Config with
// every secret (Security.APIKeyPath first and foremost) already stripped
// by the caller, unlike the Python service, which returned the entire
// config dict verbatim.
func (d Deps) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, d.ConfigView)
}
