package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_DisallowedOrigin_NoHeaders(t *testing.T) {
	handler := CORS([]string{"https://market.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for a non-whitelisted origin", got)
	}
	// The request still reaches the handler — the browser is what enforces
	// the block by refusing to expose the response given the missing
	// header — so status should still be 200.
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCORS_AllowedOrigin_GetsHeaders(t *testing.T) {
	handler := CORS([]string{"https://market.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://market.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://market.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the whitelisted origin", got)
	}
}

func TestCORS_NeverEmitsWildcard(t *testing.T) {
	handler := CORS([]string{"https://market.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://market.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatalf("Access-Control-Allow-Origin must never be *, replicates the audited finding")
	}
}

func TestCORS_NeverEmitsPrivateNetworkHeader(t *testing.T) {
	handler := CORS([]string{"https://market.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://market.example.com")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Private-Network"); got != "" {
		t.Fatalf("Access-Control-Allow-Private-Network = %q, must never be emitted (audit finding)", got)
	}
}

func TestCORS_Preflight_DisallowedOrigin_204NoHeaders(t *testing.T) {
	handler := CORS([]string{"https://market.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
