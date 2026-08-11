package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aryaescpos/internal/contract"
)

// fakeScanner/fakeQueue/etc. are minimal contract.* implementations used
// only to exercise apiserver's routing, request decoding and error
// translation — not real hardware behavior.

type fakeScanner struct {
	scanResp   contract.ScanResponse
	scanErr    error
	statusResp contract.DeviceStatus
	statusErr  error
}

func (f *fakeScanner) Scan(ctx context.Context) (contract.ScanResponse, error) {
	return f.scanResp, f.scanErr
}
func (f *fakeScanner) Status(ctx context.Context, deviceType, deviceID string) (contract.DeviceStatus, error) {
	return f.statusResp, f.statusErr
}

type fakeQueue struct {
	jobStatus contract.JobStatus
	jobErr    error
	rawErr    error
}

func (f *fakeQueue) ScanPrinters(ctx context.Context) ([]contract.DeviceInfo, error) { return nil, nil }
func (f *fakeQueue) PrinterStatus(ctx context.Context, printerName string) (contract.DeviceStatus, error) {
	return contract.DeviceStatus{}, nil
}
func (f *fakeQueue) JobStatus(ctx context.Context, printerName string, jobID int) (contract.JobStatus, error) {
	return f.jobStatus, f.jobErr
}
func (f *fakeQueue) RawPrint(ctx context.Context, printerName string, data []byte, jobName string) error {
	return f.rawErr
}
func (f *fakeQueue) FindRecentJob(ctx context.Context, printerName, docName string, timeout time.Duration) (*int, error) {
	return nil, nil
}

type fakeTicket struct {
	bytesSent int
	err       error
}

func (f *fakeTicket) Print(ctx context.Context, req contract.PrintRequest) (int, error) {
	return f.bytesSent, f.err
}

type fakeMatrix struct {
	bytesSent int
	err       error
}

func (f *fakeMatrix) Print(ctx context.Context, req contract.MatrixPrintRequest) (int, error) {
	return f.bytesSent, f.err
}

type fakeHistory struct {
	entries []contract.HistoryEntry
}

func (f *fakeHistory) Record(entry contract.HistoryEntry) contract.HistoryEntry { return entry }
func (f *fakeHistory) Query(limit int, printerName string) []contract.HistoryEntry {
	if limit < len(f.entries) {
		return f.entries[:limit]
	}
	return f.entries
}

func newTestMux(deps Deps) *http.ServeMux {
	mux := http.NewServeMux()
	New(deps, mux)
	return mux
}

func TestRoot_And_Health(t *testing.T) {
	mux := newTestMux(Deps{})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Arya ESCPOS") {
		t.Fatalf("GET / = %d %q", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "healthy") {
		t.Fatalf("GET /health = %d %q", rr.Code, rr.Body.String())
	}
}

func TestHandlePrint_ValidationError_400(t *testing.T) {
	mux := newTestMux(Deps{Ticket: &fakeTicket{}})

	body := strings.NewReader(`{"type":"windows","content":"hola"}`) // missing printer_name
	req := httptest.NewRequest(http.MethodPost, "/api/v1/print", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "detail") {
		t.Fatalf("body = %s, want a detail field", rr.Body.String())
	}
}

func TestHandlePrint_Success(t *testing.T) {
	mux := newTestMux(Deps{Ticket: &fakeTicket{bytesSent: 42}})

	body := strings.NewReader(`{"type":"windows","content":"hola","printer_name":"HP"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/print", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"bytes_sent":42`) {
		t.Fatalf("body = %s, want bytes_sent:42", rr.Body.String())
	}
}

func TestHandlePrint_UpstreamAPIError_TranslatesStatus(t *testing.T) {
	mux := newTestMux(Deps{Ticket: &fakeTicket{err: contract.BadGateway("write_failed", "could not write to device", "usb write timeout")}})

	body := strings.NewReader(`{"type":"windows","content":"hola","printer_name":"HP"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/print", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502, body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "usb write timeout") {
		t.Fatalf("body leaked internal Detail: %s", rr.Body.String())
	}
}

func TestHandlePrint_UntypedError_Generic500NoLeak(t *testing.T) {
	mux := newTestMux(Deps{Ticket: &fakeTicket{err: errPlain("dial tcp 10.0.0.5:9100: connection refused")}})

	body := strings.NewReader(`{"type":"windows","content":"hola","printer_name":"HP"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/print", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "10.0.0.5") {
		t.Fatalf("body leaked internal error detail: %s", rr.Body.String())
	}
}

type errPlain string

func (e errPlain) Error() string { return string(e) }

func TestHandlePrintHistory_ClampsLimit(t *testing.T) {
	entries := make([]contract.HistoryEntry, 10)
	for i := range entries {
		entries[i] = contract.HistoryEntry{ID: int64(i), Status: "sent"}
	}
	mux := newTestMux(Deps{History: &fakeHistory{entries: entries}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/print/history?limit=3", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"total":3`) {
		t.Fatalf("body = %s, want total:3", rr.Body.String())
	}
}

func TestHandleJobStatus_InvalidJobID_400(t *testing.T) {
	mux := newTestMux(Deps{PrintQueue: &fakeQueue{}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/windows/HP/jobs/not-a-number", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleConfig_ReturnsInjectedView(t *testing.T) {
	mux := newTestMux(Deps{ConfigView: map[string]string{"hello": "world"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "world") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "api_key_path") {
		t.Fatalf("config response must never include api_key_path: %s", rr.Body.String())
	}
}
