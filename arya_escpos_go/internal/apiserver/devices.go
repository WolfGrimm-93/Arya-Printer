package apiserver

import (
	"net/http"
	"strconv"

	"aryaescpos/internal/contract"
)

// handleDevicesScan is GET /api/v1/devices/scan.
func (d Deps) handleDevicesScan(w http.ResponseWriter, r *http.Request) {
	resp, err := d.Scanner.Scan(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDeviceStatus is GET /api/v1/devices/{type}/{id}/status. Dispatch by
// deviceType (including the "network" SSRF-sensitive case) is entirely the
// DeviceScanner implementation's responsibility, per its contract doc
// comment — this handler is a thin pass-through.
func (d Deps) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	deviceType := r.PathValue("type")
	deviceID := r.PathValue("id")

	status, err := d.Scanner.Status(r.Context(), deviceType, deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// handleJobStatus is GET /api/v1/devices/windows/{printer}/jobs/{job_id}.
// printer name underscores are translated back to spaces, matching the
// Python service (device IDs use underscores in place of spaces so they
// survive as a single URL path segment).
func (d Deps) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	printerName := underscoresToSpaces(r.PathValue("printer"))
	jobID, err := strconv.Atoi(r.PathValue("job_id"))
	if err != nil {
		writeError(w, contract.BadRequest("invalid_job_id", "job_id must be an integer"))
		return
	}

	status, err := d.PrintQueue.JobStatus(r.Context(), printerName, jobID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func underscoresToSpaces(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			out[i] = ' '
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}
