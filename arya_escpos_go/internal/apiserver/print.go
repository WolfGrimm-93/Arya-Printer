package apiserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aryaescpos/internal/contract"
)

// handlePrint is POST /api/v1/print (ESC/POS thermal tickets).
func (d Deps) handlePrint(w http.ResponseWriter, r *http.Request) {
	var req contract.PrintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := ValidatePrintRequest(&req); err != nil {
		writeError(w, err)
		return
	}
	if err := validateHeaderImageSize(req.HeaderImage, d.MaxImageBytes); err != nil {
		writeError(w, err)
		return
	}

	bytesSent, err := d.Ticket.Print(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, contract.PrintResult{
		Success:   true,
		Message:   fmt.Sprintf("Print job sent (%s)", req.Type),
		BytesSent: bytesSent,
	})
}

// handlePrintReport is POST /api/v1/print/report — a formatted plain-text
// report sent RAW to the Windows spooler, matching print_routes.py's
// print_report byte-for-byte layout (centered title, "=" and "-" rules).
func (d Deps) handlePrintReport(w http.ResponseWriter, r *http.Request) {
	var req contract.ReportPrintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	lines := []string{
		centerText(req.Title, 80),
		strings.Repeat("=", 80),
		"",
		req.Content,
		"",
		strings.Repeat("-", 80),
	}
	textBytes := []byte(strings.Join(lines, "\n"))

	if err := d.PrintQueue.RawPrint(r.Context(), req.PrinterName, textBytes, "Arya Report"); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Report sent to %s", req.PrinterName),
	})
}

// centerText mirrors Python's str.center(width): pads s with spaces on
// both sides so it is centered within width, with the extra space (if the
// padding is odd) going on the right, same as CPython's str.center.
func centerText(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	total := width - n
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// handlePrintMatrix is POST /api/v1/print/matrix (ESC/P dot-matrix tickets).
func (d Deps) handlePrintMatrix(w http.ResponseWriter, r *http.Request) {
	var req contract.MatrixPrintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := ValidateMatrixPrintRequest(&req); err != nil {
		writeError(w, err)
		return
	}

	bytesSent, err := d.Matrix.Print(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, contract.PrintResult{
		Success:   true,
		Message:   fmt.Sprintf("Matrix print job sent (%s)", req.Type),
		BytesSent: bytesSent,
	})
}

// handlePrintHistory is GET /api/v1/print/history.
func (d Deps) handlePrintHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}

	printerName := r.URL.Query().Get("printer_name")

	jobs := d.History.Query(limit, printerName)
	writeJSON(w, http.StatusOK, contract.HistoryResponse{
		Total: len(jobs),
		Jobs:  jobs,
	})
}
