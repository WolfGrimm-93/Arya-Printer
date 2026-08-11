package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aryaescpos/internal/apiserver"
	"aryaescpos/internal/config"
)

// TestGetConfig_NeverLeaksAPIKeyPath is a regression test requested in code
// review: sanitizedConfigView's exclusion of Security.APIKeyPath is
// currently correct but fragile-by-convention — a future change (e.g.
// someone swapping it for json.Marshal(cfg) directly) would reintroduce the
// leak with nothing to catch it. This exercises the REAL, fully-wired HTTP
// handler (auth included, same buildHandler runServer uses) rather than
// calling sanitizedConfigView in isolation, so it catches a regression
// anywhere in the chain: GET /api/v1/config with a valid API key must never
// return "api_key_path" at any level of the JSON body, nor the actual key
// path value.
func TestGetConfig_NeverLeaksAPIKeyPath(t *testing.T) {
	cfg := config.Default()
	const sentinelPath = "C:/definitely-secret/apikey.key"
	cfg.Security.APIKeyPath = sentinelPath
	cfg.Security.AuthEnabled = true

	const apiKey = "test-api-key"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	deps := apiserver.Deps{ConfigView: sanitizedConfigView(cfg)}
	handler := buildHandler(deps, cfg, apiKey, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	req.Header.Set("X-API-Key", apiKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if strings.Contains(body, sentinelPath) {
		t.Fatalf("response leaked the api_key_path value: %s", body)
	}

	var parsed any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if findKeyRecursive(parsed, "api_key_path") {
		t.Fatalf("response JSON contains an 'api_key_path' key at some level: %s", body)
	}
}

// TestGetConfig_RequiresAuth guards the same endpoint's other property:
// buildHandler's middleware chain must still require the API key for
// /api/v1/config like every other /api/v1/* route.
func TestGetConfig_RequiresAuth(t *testing.T) {
	cfg := config.Default()
	cfg.Security.AuthEnabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	deps := apiserver.Deps{ConfigView: sanitizedConfigView(cfg)}
	handler := buildHandler(deps, cfg, "test-api-key", logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", rr.Code, rr.Body.String())
	}
}

// findKeyRecursive walks an arbitrary decoded-JSON value (map[string]any /
// []any / scalars, the shapes encoding/json produces for `any`) looking for
// the given key at any nesting level.
func findKeyRecursive(v any, key string) bool {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == key {
				return true
			}
			if findKeyRecursive(val, key) {
				return true
			}
		}
	case []any:
		for _, item := range t {
			if findKeyRecursive(item, key) {
				return true
			}
		}
	}
	return false
}
