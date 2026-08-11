package apiserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aryaescpos/internal/contract"
)

// handlePrintDocument is POST /api/v1/print/document: a multipart file
// upload, converted and printed via Deps.Document. The uploaded content is
// always written to a temp file first, and that temp file is ALWAYS
// removed before returning (success or error) — Deps.Document owns any
// intermediate files *it* creates, but never this one (see
// contract.DocumentPrinter's doc comment).
func (d Deps) handlePrintDocument(w http.ResponseWriter, r *http.Request) {
	printerName := r.URL.Query().Get("printer_name")
	if printerName == "" {
		writeError(w, contract.BadRequest("missing_field", "printer_name is required"))
		return
	}

	opts := contract.DocumentPrintOptions{
		PrinterName: printerName,
		Orientation: queryOrDefault(r, "orientation", "portrait"),
		Copies:      queryIntOrDefault(r, "copies", 1),
		Color:       queryOrDefault(r, "color", "color"),
		Duplex:      queryOrDefault(r, "duplex", "simplex"),
	}
	if opts.Copies < 1 {
		opts.Copies = 1
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		if isMaxBytesError(err) {
			writeError(w, contract.PayloadTooLarge("uploaded file exceeds the configured upload limit"))
			return
		}
		writeError(w, contract.BadRequest("missing_file", "a 'file' multipart field is required"))
		return
	}
	defer file.Close()

	suffix := filepath.Ext(header.Filename)
	if suffix == "" {
		suffix = ".pdf"
	}

	tmp, err := os.CreateTemp("", "aryaescpos-upload-*"+suffix)
	if err != nil {
		writeError(w, fmt.Errorf("apiserver: creating temp file: %w", err))
		return
	}
	tempPath := tmp.Name()
	// ALWAYS clean up the temp file we created, on every return path.
	defer os.Remove(tempPath)

	size, copyErr := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if copyErr != nil {
		if isMaxBytesError(copyErr) {
			writeError(w, contract.PayloadTooLarge("uploaded file exceeds the configured upload limit"))
			return
		}
		writeError(w, fmt.Errorf("apiserver: writing temp file: %w", copyErr))
		return
	}
	if closeErr != nil {
		writeError(w, fmt.Errorf("apiserver: closing temp file: %w", closeErr))
		return
	}

	// Best-effort defense in depth: mark the temp file as having come from
	// the internet (NTFS Zone.Identifier Alternate Data Stream), the same
	// signal Windows/Office/LibreOffice use to decide whether to trust a
	// file's macros. The upload arrived over HTTP and gets written straight
	// to disk with no such marker by default, so without this LibreOffice
	// would treat it as a fully-trusted local file. This is deliberately
	// non-fatal — a failure here (e.g. a non-NTFS temp volume) must not
	// block printing, and internal/document's conversion path independently
	// forces a high macro security level regardless of this marker.
	markAsInternetZone(tempPath)

	jobID, err := d.Document.PrintDocument(r.Context(), tempPath, header.Filename, opts)
	if err != nil {
		writeError(w, err)
		return
	}

	var trackURL string
	if jobID != nil {
		trackURL = fmt.Sprintf("/api/v1/devices/windows/%s/jobs/%d", strings.ReplaceAll(printerName, " ", "_"), *jobID)
	}

	writeJSON(w, http.StatusOK, contract.DocumentPrintResult{
		Success:       true,
		Message:       fmt.Sprintf("Document '%s' sent to %s", header.Filename, printerName),
		FileSizeBytes: size,
		FileSize:      humanFileSize(size),
		JobID:         jobID,
		TrackURL:      trackURL,
	})
}

// markAsInternetZone stamps path with an NTFS Zone.Identifier Alternate
// Data Stream containing ZoneId=3 ("Internet zone") — the same marker
// Windows itself writes on files downloaded via a browser. Errors are
// intentionally swallowed: this is a defense-in-depth signal for
// LibreOffice/Office's macro-security heuristics, not a correctness
// requirement, and failing to write it (e.g. a non-NTFS temp directory)
// must never block the print request.
func markAsInternetZone(path string) {
	const zoneIdentifier = "[ZoneTransfer]\r\nZoneId=3\r\n"
	_ = os.WriteFile(path+":Zone.Identifier", []byte(zoneIdentifier), 0o600)
}

// isMaxBytesError reports whether err (or one it wraps) is the error
// returned by an http.MaxBytesReader once the configured
// Security.MaxUploadBytes limit (enforced by middleware.BodyLimit around
// r.Body) has been exceeded.
func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// humanFileSize mirrors print_routes.py's inline formatting exactly:
// bytes under 1 KiB, two-decimal KB under 1 MiB, two-decimal MB above.
func humanFileSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d bytes", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	}
}

func queryOrDefault(r *http.Request, key, def string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return def
}

func queryIntOrDefault(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
