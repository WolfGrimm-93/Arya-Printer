package e2e

import (
	"context"
	"sync"
	"time"

	"aryaescpos/internal/contract"
)

// The fakes below implement every internal/contract interface with no real
// hardware, filesystem or network behavior. They exist so this package's
// tests can exercise the full apiserver+middleware HTTP stack end to end
// without depending on internal/printsvc, internal/winspool, internal/history
// or internal/document (Agents B/C) — those may not exist yet, or may still
// be changing, while this suite runs. Each fake records enough about the
// call it received for tests to assert on.

// fakeTicketPrinter implements contract.TicketPrinter.
type fakeTicketPrinter struct {
	mu        sync.Mutex
	calls     []contract.PrintRequest
	bytesSent int
	err       error
}

func (f *fakeTicketPrinter) Print(ctx context.Context, req contract.PrintRequest) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	return f.bytesSent, f.err
}

func (f *fakeTicketPrinter) lastCall() (contract.PrintRequest, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return contract.PrintRequest{}, false
	}
	return f.calls[len(f.calls)-1], true
}

// fakeMatrixPrinter implements contract.MatrixPrinter.
type fakeMatrixPrinter struct {
	mu        sync.Mutex
	calls     []contract.MatrixPrintRequest
	bytesSent int
	err       error
}

func (f *fakeMatrixPrinter) Print(ctx context.Context, req contract.MatrixPrintRequest) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	return f.bytesSent, f.err
}

// fakeDocumentPrinter implements contract.DocumentPrinter.
type fakeDocumentPrinter struct {
	jobID *int
	err   error
}

func (f *fakeDocumentPrinter) PrintDocument(ctx context.Context, localFilePath, originalFilename string, opts contract.DocumentPrintOptions) (*int, error) {
	return f.jobID, f.err
}

// fakeDeviceScanner implements contract.DeviceScanner.
type fakeDeviceScanner struct {
	scanResp   contract.ScanResponse
	scanErr    error
	statusResp contract.DeviceStatus
	statusErr  error
}

func (f *fakeDeviceScanner) Scan(ctx context.Context) (contract.ScanResponse, error) {
	return f.scanResp, f.scanErr
}

func (f *fakeDeviceScanner) Status(ctx context.Context, deviceType, deviceID string) (contract.DeviceStatus, error) {
	return f.statusResp, f.statusErr
}

// fakeWindowsPrintQueue implements contract.WindowsPrintQueue.
type fakeWindowsPrintQueue struct {
	printers  []contract.DeviceInfo
	status    contract.DeviceStatus
	jobStatus contract.JobStatus
	jobErr    error
	rawErr    error
	recentJob *int
}

func (f *fakeWindowsPrintQueue) ScanPrinters(ctx context.Context) ([]contract.DeviceInfo, error) {
	return f.printers, nil
}

func (f *fakeWindowsPrintQueue) PrinterStatus(ctx context.Context, printerName string) (contract.DeviceStatus, error) {
	return f.status, nil
}

func (f *fakeWindowsPrintQueue) JobStatus(ctx context.Context, printerName string, jobID int) (contract.JobStatus, error) {
	return f.jobStatus, f.jobErr
}

func (f *fakeWindowsPrintQueue) RawPrint(ctx context.Context, printerName string, data []byte, jobName string) error {
	return f.rawErr
}

func (f *fakeWindowsPrintQueue) FindRecentJob(ctx context.Context, printerName, docName string, timeout time.Duration) (*int, error) {
	return f.recentJob, nil
}

// fakeHistoryStore implements contract.HistoryStore, including the
// case-insensitive exact PrinterName filter documented on the interface
// (internal/contract/services.go) — real enough behavior to test the
// history endpoint's filtering, not just its plumbing.
type fakeHistoryStore struct {
	mu      sync.Mutex
	entries []contract.HistoryEntry
}

func (f *fakeHistoryStore) Record(entry contract.HistoryEntry) contract.HistoryEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append([]contract.HistoryEntry{entry}, f.entries...) // newest first
	return entry
}

func (f *fakeHistoryStore) Query(limit int, printerName string) []contract.HistoryEntry {
	f.mu.Lock()
	defer f.mu.Unlock()

	var matched []contract.HistoryEntry
	for _, e := range f.entries {
		if printerName != "" && !equalFold(e.PrinterName, printerName) {
			continue
		}
		matched = append(matched, e)
	}
	if limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}
	return matched
}

// equalFold avoids pulling in "strings" just for this one comparison in a
// tiny test fake.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
