package contract

import (
	"context"
	"time"
)

// DeviceScanner discovers devices across all enabled transports (windows,
// usb, serial) and reports status for a single device. Implemented in
// internal/printsvc (Agent B), consumed only by internal/apiserver (Agent A).
type DeviceScanner interface {
	Scan(ctx context.Context) (ScanResponse, error)
	// Status dispatches by deviceType ("windows"|"usb"|"network"|"serial").
	// deviceType == "network" is handled without ever going through
	// AdapterFactory for the read-only reachability check, but MUST still be
	// validated by internal/netguard first — this endpoint is an SSRF
	// oracle in the Python service today (audit finding, part of C3) and
	// must not become one here.
	Status(ctx context.Context, deviceType, deviceID string) (DeviceStatus, error)
}

// WindowsPrintQueue wraps the Windows Spooler (winspool.drv). Implemented by
// internal/winspool + internal/printsvc (Agent B). Consumed by
// internal/apiserver (Agent A) for scan/status/job-tracking endpoints and by
// internal/printsvc/document.go (Agent C) for document printing.
type WindowsPrintQueue interface {
	ScanPrinters(ctx context.Context) ([]DeviceInfo, error)
	PrinterStatus(ctx context.Context, printerName string) (DeviceStatus, error)
	JobStatus(ctx context.Context, printerName string, jobID int) (JobStatus, error)
	RawPrint(ctx context.Context, printerName string, data []byte, jobName string) error
	// FindRecentJob polls the print queue for a job matching docName within
	// timeout, returning nil (not an error) if none is found in time.
	FindRecentJob(ctx context.Context, printerName, docName string, timeout time.Duration) (*int, error)
}

// TicketPrinter prints ESC/POS thermal tickets (POST /api/v1/print).
// Implemented in internal/printsvc/ticket.go (Agent B), composing
// internal/escpos (command builder) with AdapterFactory/WindowsPrintQueue.
type TicketPrinter interface {
	Print(ctx context.Context, req PrintRequest) (bytesSent int, err error)
}

// MatrixPrinter prints ESC/P dot-matrix tickets (POST /api/v1/print/matrix).
// Implemented in internal/printsvc/matrix.go (Agent B), composing
// internal/escp (command builder) with AdapterFactory/WindowsPrintQueue.
type MatrixPrinter interface {
	Print(ctx context.Context, req MatrixPrintRequest) (bytesSent int, err error)
}

// DocumentPrinter converts and prints arbitrary documents
// (POST /api/v1/print/document). localFilePath is a temp file already saved
// by apiserver from the multipart upload; DocumentPrinter owns any
// intermediate temp files it creates (e.g. a converted PDF) and MUST clean
// them up itself, including on error paths, but does NOT delete
// localFilePath — the caller (apiserver) owns that file's lifecycle.
// Implemented in internal/printsvc/document.go (Agent C).
type DocumentPrinter interface {
	PrintDocument(ctx context.Context, localFilePath, originalFilename string, opts DocumentPrintOptions) (jobID *int, err error)
}

// HistoryStore records and queries in-memory print history: max 500
// entries, newest first, no persistence across restarts. Implemented in
// internal/history (Agent C).
type HistoryStore interface {
	// Record stores entry and returns it with any store-assigned fields
	// (ID, Timestamp) populated.
	Record(entry HistoryEntry) HistoryEntry
	// Query returns at most limit entries (caller must clamp to [1,500]),
	// optionally filtered by an exact, case-insensitive PrinterName match.
	Query(limit int, printerName string) []HistoryEntry
}
