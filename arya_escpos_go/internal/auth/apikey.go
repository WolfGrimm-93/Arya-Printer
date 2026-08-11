// Package auth manages the local API key used to authenticate requests to
// internal/apiserver. There is no equivalent in the Python service — it had
// no authentication at all, the most critical finding of the security
// audit.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// keyBytes is the size of the generated key before base64url encoding.
const keyBytes = 32

// LoadOrCreate returns the API key stored at path, generating and
// persisting a new random one if the file does not exist yet. Subsequent
// calls (e.g. across service restarts) reuse the same key instead of
// regenerating it, so previously issued keys keep working.
func LoadOrCreate(path string) (string, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		key := strings.TrimSpace(string(existing))
		if key != "" {
			return key, nil
		}
		// Fall through and regenerate if the file exists but is empty/blank.
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("auth: reading api key file %s: %w", path, err)
	}

	key, err := generate()
	if err != nil {
		return "", fmt.Errorf("auth: generating api key: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("auth: creating api key directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		return "", fmt.Errorf("auth: writing api key file %s: %w", path, err)
	}

	return key, nil
}

// generate produces a new random API key: 32 bytes from crypto/rand,
// base64url-encoded (unpadded, so it is safe to use as an HTTP header value
// or a URL path segment without escaping).
func generate() (string, error) {
	buf := make([]byte, keyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Valid reports whether candidate matches key using a constant-time
// comparison, so response timing cannot be used to leak the key
// byte-by-byte (a plain == string comparison short-circuits on the first
// mismatched byte and must never be used here).
func Valid(key, candidate string) bool {
	if key == "" || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(candidate)) == 1
}

// Show implements the --show-api-key CLI flag: it loads (or creates) the
// key at path and returns it for printing to stdout, without ever logging
// it through internal/logging.
func Show(path string) (string, error) {
	return LoadOrCreate(path)
}
