// Package e2e exercises the real HTTP surface of the Arya Printer agent:
// internal/apiserver's routes wrapped by the exact internal/middleware chain
// cmd/aryaprinter/main.go builds in runServer (BodyLimit, CORS, Auth,
// Logging, Recover) — not just the bare mux internal/apiserver's own unit
// tests hit. It runs against httptest.NewServer (a real loopback listener),
// so a request without an X-API-Key header is rejected by the actual
// middleware, not by a hand-rolled substitute.
//
// Every dependency is a fake implementing internal/contract's interfaces
// (see fakes_test.go), written independently of Agents B/C's concrete
// packages (internal/printsvc, internal/winspool, internal/history,
// internal/document) — this suite must run and pass whether or not those
// packages exist yet.
package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aryaescpos/internal/apiserver"
	"aryaescpos/internal/contract"
	"aryaescpos/internal/middleware"
)

const testAPIKey = "test-api-key-e2e-0123456789"

// newE2EServer builds the same handler chain as cmd/aryaprinter/main.go's
// buildHandler: apiserver.New(deps, mux), wrapped by BodyLimit -> Auth ->
// CORS -> Logging -> Recover, then serves it on a real loopback listener via
// httptest.NewServer. Auth must be applied before CORS (i.e. CORS wraps
// Auth, so CORS runs first) so an OPTIONS preflight — sent without
// X-API-Key, browsers never include it in preflight — reaches CORS's 204
// short-circuit before Auth would otherwise reject it with 401 and no CORS
// headers; getting this order wrong silently breaks every cross-origin call
// (see cmd/aryaprinter/main.go's buildHandler for the full explanation of a
// real bug this exact ordering fixed). The caller must Close() the returned
// server (t.Cleanup does it here so callers don't have to remember).
func newE2EServer(t *testing.T, deps apiserver.Deps, authEnabled bool) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	apiserver.New(deps, mux)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var handler http.Handler = mux
	handler = middleware.BodyLimit(50 * 1024 * 1024)(handler)
	handler = middleware.Auth(authEnabled, func(candidate string) bool {
		return candidate == testAPIKey
	})(handler)
	handler = middleware.CORS()(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.Recover(logger)(handler)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func doJSON(t *testing.T, srv *httptest.Server, method, path, apiKey string, body any) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return string(b)
}

// --- GET /health ---

func TestHealth_NoAuthRequired(t *testing.T) {
	srv := newE2EServer(t, apiserver.Deps{}, true /* authEnabled */)

	resp := doJSON(t, srv, http.MethodGet, "/health", "" /* no api key */, nil)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health without X-API-Key = %d, want 200 (health must be exempt from auth); body=%s", resp.StatusCode, body)
	}
	if !bytes.Contains([]byte(body), []byte("healthy")) {
		t.Fatalf("body = %s, want it to mention healthy", body)
	}
}

func TestRoot_NoAuthRequired(t *testing.T) {
	srv := newE2EServer(t, apiserver.Deps{}, true)

	resp := doJSON(t, srv, http.MethodGet, "/", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / without X-API-Key = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- POST /api/v1/print ---

func TestPrint_MissingAPIKey_401(t *testing.T) {
	srv := newE2EServer(t, apiserver.Deps{Ticket: &fakeTicketPrinter{}}, true)

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/print", "" /* no key */, map[string]any{
		"type":         "windows",
		"printer_name": "HP LaserJet",
		"content":      "hola",
	})
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /api/v1/print without X-API-Key = %d, want 401; body=%s", resp.StatusCode, body)
	}
}

func TestPrint_WithValidKey_200AndFakeRecordsCall(t *testing.T) {
	ticket := &fakeTicketPrinter{bytesSent: 245}
	srv := newE2EServer(t, apiserver.Deps{Ticket: ticket}, true)

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/print", testAPIKey, map[string]any{
		"type":         "windows",
		"printer_name": "HP LaserJet",
		"content":      "FACTURA #001\nTotal: $100",
	})
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/v1/print with valid key = %d, want 200; body=%s", resp.StatusCode, body)
	}

	var result struct {
		Success   bool `json:"success"`
		BytesSent int  `json:"bytes_sent"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decoding response body %s: %v", body, err)
	}
	if !result.Success || result.BytesSent != 245 {
		t.Fatalf("body = %s, want success=true bytes_sent=245", body)
	}

	call, ok := ticket.lastCall()
	if !ok {
		t.Fatal("fakeTicketPrinter.Print was never called")
	}
	if call.Type != "windows" || call.PrinterName != "HP LaserJet" || call.Content != "FACTURA #001\nTotal: $100" {
		t.Fatalf("fake recorded call = %+v, want type=windows printer_name='HP LaserJet' content='FACTURA #001\\nTotal: $100'", call)
	}
}

func TestPrint_USBWithoutVidPid_400(t *testing.T) {
	srv := newE2EServer(t, apiserver.Deps{Ticket: &fakeTicketPrinter{}}, true)

	resp := doJSON(t, srv, http.MethodPost, "/api/v1/print", testAPIKey, map[string]any{
		"type":    "usb",
		"content": "hola",
		// vid/pid intentionally omitted
	})
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/print type=usb without vid/pid = %d, want 400; body=%s", resp.StatusCode, body)
	}
	if !bytes.Contains([]byte(body), []byte("detail")) {
		t.Fatalf("body = %s, want a detail field explaining the missing vid/pid", body)
	}
}

// --- GET /api/v1/print/history ---

func TestPrintHistory_FilterByPrinterName(t *testing.T) {
	history := &fakeHistoryStore{}
	history.Record(contract.HistoryEntry{PrinterName: "HP LaserJet", Protocol: "escpos", Status: "sent"})
	history.Record(contract.HistoryEntry{PrinterName: "Epson LX-350", Protocol: "escp", Status: "sent"})
	history.Record(contract.HistoryEntry{PrinterName: "hp laserjet", Protocol: "document", Status: "sent"}) // case-insensitive match on filter

	srv := newE2EServer(t, apiserver.Deps{History: history}, true)

	resp := doJSON(t, srv, http.MethodGet, "/api/v1/print/history?printer_name=HP+LaserJet", testAPIKey, nil)
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/print/history?printer_name=... = %d, want 200; body=%s", resp.StatusCode, body)
	}

	var result struct {
		Total int `json:"total"`
		Jobs  []struct {
			PrinterName string `json:"printer_name"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decoding response body %s: %v", body, err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2 (both entries whose printer_name matches 'HP LaserJet' case-insensitively); body=%s", result.Total, body)
	}
	for _, job := range result.Jobs {
		if !equalFold(job.PrinterName, "HP LaserJet") {
			t.Fatalf("job with printer_name=%q leaked into a filtered query; body=%s", job.PrinterName, body)
		}
	}
}
