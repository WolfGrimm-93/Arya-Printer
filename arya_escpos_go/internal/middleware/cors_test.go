package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AnyOrigin_GetsHeaders(t *testing.T) {
	handler := CORS()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://any-domain-whatsoever.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://any-domain-whatsoever.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the request's own origin echoed back", got)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCORS_NoOriginHeader_PassesThroughUntouched(t *testing.T) {
	handler := CORS()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty when no Origin header was sent", got)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestCORS_NeverEmitsWildcard(t *testing.T) {
	handler := CORS()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://market.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatalf("Access-Control-Allow-Origin must never be a bare *, even though every origin is allowed — echoing the specific origin keeps the header meaningful for browsers")
	}
}

func TestCORS_NeverEmitsPrivateNetworkHeader(t *testing.T) {
	handler := CORS()(
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

func TestCORS_Preflight_204WithHeaders(t *testing.T) {
	handler := CORS()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices/scan", nil)
	req.Header.Set("Origin", "https://market.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://market.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the request's own origin echoed back on preflight too", got)
	}
}
