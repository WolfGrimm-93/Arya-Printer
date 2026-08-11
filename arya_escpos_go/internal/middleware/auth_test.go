package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_MissingKey_401(t *testing.T) {
	handler := Auth(true, func(candidate string) bool { return candidate == "secret" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuth_WrongKey_401(t *testing.T) {
	handler := Auth(true, func(candidate string) bool { return candidate == "secret" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set(apiKeyHeader, "wrong")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuth_CorrectKey_200(t *testing.T) {
	handler := Auth(true, func(candidate string) bool { return candidate == "secret" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	req.Header.Set(apiKeyHeader, "secret")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAuth_ExemptPaths_NoKeyNeeded(t *testing.T) {
	handler := Auth(true, func(candidate string) bool { return false })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	for _, path := range []string{"/", "/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("path %s: status = %d, want %d", path, rr.Code, http.StatusOK)
		}
	}
}

func TestAuth_Disabled_NoKeyNeeded(t *testing.T) {
	handler := Auth(false, func(candidate string) bool { return false })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/scan", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}
