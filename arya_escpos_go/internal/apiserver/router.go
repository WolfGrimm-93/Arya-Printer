package apiserver

import "net/http"

// New registers every route on mux, dispatching to Deps' methods. It does
// not wrap mux with any middleware (auth, CORS, recovery, body limits,
// request logging) — that is cmd/aryaprinter/main.go's job, composing
// internal/middleware around the *http.ServeMux this function returns
// routes on, so this package stays free of any dependency beyond
// internal/contract and the standard library.
func New(deps Deps, mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", handleRoot)
	mux.HandleFunc("GET /health", handleHealth)

	mux.HandleFunc("GET /api/v1/devices/scan", deps.handleDevicesScan)
	mux.HandleFunc("GET /api/v1/devices/windows/{printer}/jobs/{job_id}", deps.handleJobStatus)
	mux.HandleFunc("GET /api/v1/devices/{type}/{id}/status", deps.handleDeviceStatus)

	mux.HandleFunc("POST /api/v1/print", deps.handlePrint)
	mux.HandleFunc("POST /api/v1/print/report", deps.handlePrintReport)
	mux.HandleFunc("POST /api/v1/print/document", deps.handlePrintDocument)
	mux.HandleFunc("POST /api/v1/print/matrix", deps.handlePrintMatrix)
	mux.HandleFunc("GET /api/v1/print/history", deps.handlePrintHistory)

	mux.HandleFunc("GET /api/v1/config", deps.handleConfig)
}
