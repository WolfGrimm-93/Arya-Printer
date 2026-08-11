// Package contract holds the DTOs and interfaces shared by every package in
// this module. It is frozen after Phase 0: internal/apiserver only ever
// imports contract, never the concrete implementations in hwadapter,
// winspool, escpos, escp or document directly.
package contract

// DeviceInfo represents a single device returned by GET /api/v1/devices/scan.
// Field presence varies by Type, matching the Python service's per-type dicts:
//   - windows: ID, Type, Name, Manufacturer, Description
//   - usb:     ID, Type, Name, Manufacturer, VID, PID, Serial
//   - serial:  ID, Type, Name, ComPort
type DeviceInfo struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Description  string `json:"description,omitempty"`
	VID          string `json:"vid,omitempty"`
	PID          string `json:"pid,omitempty"`
	Serial       string `json:"serial,omitempty"`
	IP           string `json:"ip,omitempty"`
	Port         int    `json:"port,omitempty"`
	ComPort      string `json:"com_port,omitempty"`
}

// ScanResponse is the body of GET /api/v1/devices/scan.
type ScanResponse struct {
	Found   int          `json:"found"`
	Devices []DeviceInfo `json:"devices"`
}

// DeviceStatus is the response of GET /api/v1/devices/{type}/{id}/status.
// It is a superset shape covering windows/usb/network/serial; fields not
// applicable to a given device type must be left zero so omitempty drops
// them from the JSON, matching the Python service's distinct per-type dicts.
type DeviceStatus struct {
	DeviceType  string   `json:"device_type"`
	PrinterName string   `json:"printer_name,omitempty"` // windows only
	Host        string   `json:"host,omitempty"`         // network only
	Port        int      `json:"port,omitempty"`         // network only
	ComPort     string   `json:"com_port,omitempty"`     // serial only; added during review, Python always includes this key for serial status
	IsOnline    bool     `json:"is_online"`
	Status      string   `json:"status"`
	StatusCode  *int     `json:"status_code,omitempty"` // windows only
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Details     []string `json:"details,omitempty"`
	JobsInQueue *int     `json:"jobs_in_queue,omitempty"` // windows only
}

// JobStatus is the response of GET /api/v1/devices/windows/{printer}/jobs/{job_id}.
// StatusCode is the raw Windows JOB_STATUS_* value (0=queued .. 6=printed);
// Status is the same mapping used by the Python service (status_map dict),
// not the real spooler bitmask semantics — replicated as-is for contract fidelity.
type JobStatus struct {
	JobID           int    `json:"job_id"`
	PrinterName     string `json:"printer_name"`
	DocumentName    string `json:"document_name"`
	Status          string `json:"status"`
	StatusCode      int    `json:"status_code"`
	TotalPages      int    `json:"total_pages"`
	PagesPrinted    int    `json:"pages_printed"`
	PositionInQueue int    `json:"position_in_queue"`
	SubmittedTime   string `json:"submitted_time"`
	SizeInBytes     int    `json:"size_in_bytes"`
}

// PrintRequest is the body of POST /api/v1/print (ESC/POS thermal tickets).
// Type selects which of PrinterName/VID+PID/IP+Port/ComPort is required;
// validation lives in apiserver, mirroring PrintRequest.check_required_fields
// in the Python schemas.py. Bluetooth is accepted by the Type enum in Python
// but is out of scope for this v1 — apiserver rejects it with 400.
type PrintRequest struct {
	Type        string `json:"type"` // windows|usb|network|serial
	Content     string `json:"content"`
	HeaderImage string `json:"header_image,omitempty"`
	ImageWidth  int    `json:"image_width,omitempty"` // default 576
	PrinterName string `json:"printer_name,omitempty"`
	VID         string `json:"vid,omitempty"` // hex string, e.g. "04b8"
	PID         string `json:"pid,omitempty"`
	IP          string `json:"ip,omitempty"`
	Port        int    `json:"port,omitempty"` // default 9100
	ComPort     string `json:"com_port,omitempty"`
}

// ReportPrintRequest is the body of POST /api/v1/print/report.
type ReportPrintRequest struct {
	PrinterName string `json:"printer_name"`
	Title       string `json:"title"`
	Content     string `json:"content"`
}

// BarcodeItem is one entry of MatrixPrintRequest.Barcodes. If rendering
// fails for any reason (unsupported Type, invalid Data for the symbology,
// missing renderer), the printer MUST fall back to a plain-text label
// "[TYPE: data]" instead of failing the request — this is a preserved
// behavioral contract from matrix_command_builder.py's barcode(), not
// optional error handling.
type BarcodeItem struct {
	Data       string `json:"data"`
	Type       string `json:"type,omitempty"`        // code39|code128|ean13|ean8|upca|upc, default code39
	WidthDots  int    `json:"width_dots,omitempty"`  // default 300
	HeightDots int    `json:"height_dots,omitempty"` // default 48
}

// MatrixPrintRequest is the body of POST /api/v1/print/matrix (ESC/P dot matrix).
type MatrixPrintRequest struct {
	Type        string        `json:"type"` // windows|usb|serial (no network, no bluetooth)
	Content     string        `json:"content"`
	Encoding    string        `json:"encoding,omitempty"`  // default cp850
	FormFeed    *bool         `json:"form_feed,omitempty"` // default true; pointer so "false" is distinguishable from "absent"
	Font        string        `json:"font,omitempty"`      // roman|sans_serif, default roman
	CPI         int           `json:"cpi,omitempty"`       // 10|12|15, default 10
	Barcodes    []BarcodeItem `json:"barcodes,omitempty"`
	PrinterName string        `json:"printer_name,omitempty"`
	VID         string        `json:"vid,omitempty"`
	PID         string        `json:"pid,omitempty"`
	ComPort     string        `json:"com_port,omitempty"`
	BaudRate    int           `json:"baud_rate,omitempty"` // default 9600; MUST actually reach the serial port in Go (fixes the Python no-op bug)
}

// DocumentPrintOptions are the query params of POST /api/v1/print/document.
type DocumentPrintOptions struct {
	PrinterName string
	Orientation string // portrait|landscape, default portrait
	Copies      int    // default 1
	Color       string // color|grayscale, default color
	Duplex      string // simplex|duplex, default simplex
}

// HistoryEntry is one entry returned by GET /api/v1/print/history.
type HistoryEntry struct {
	ID          int64  `json:"id"`
	Timestamp   string `json:"timestamp"` // RFC3339-ish, seconds precision, matching Python's isoformat(timespec="seconds")
	PrinterName string `json:"printer_name"`
	Document    string `json:"document"`
	Protocol    string `json:"protocol"` // escpos|escp|document|report
	BytesSent   int    `json:"bytes_sent"`
	JobID       *int   `json:"job_id"`
	Status      string `json:"status"` // always "sent"
}

// HistoryResponse is the body of GET /api/v1/print/history.
type HistoryResponse struct {
	Total int            `json:"total"`
	Jobs  []HistoryEntry `json:"jobs"`
}

// PrintResult is the common response shape for /print, /print/matrix and /print/report.
type PrintResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	BytesSent int    `json:"bytes_sent,omitempty"`
}

// DocumentPrintResult is the response of POST /api/v1/print/document.
type DocumentPrintResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	FileSize      string `json:"file_size"` // human-readable, e.g. "122.49 KB"
	JobID         *int   `json:"job_id"`
	TrackURL      string `json:"track_url,omitempty"`
}
