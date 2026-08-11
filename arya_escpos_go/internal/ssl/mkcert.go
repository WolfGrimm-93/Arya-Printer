// Package ssl wraps mkcert.exe to install a locally-trusted CA and generate a
// localhost TLS certificate, plus certificate-expiry inspection so the
// service can auto-renew before HTTPS breaks. This is a direct port of the
// Python service's utils/ssl_setup.py — mkcert.exe itself is unchanged and
// keeps being vendored next to the installed executable exactly as before;
// there is no reason to replace it.
package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func mkcertExePath(installDir string) string {
	return filepath.Join(installDir, "mkcert.exe")
}

// runMkcert invokes mkcert.exe (looked up next to installDir, same
// convention as the Python service's _mkcert_exe) with args, optionally in
// working directory cwd. Returns whether it succeeded and its combined
// stdout+stderr output.
func runMkcert(installDir, cwd string, args ...string) (ok bool, output string) {
	exePath := mkcertExePath(installDir)
	if _, err := os.Stat(exePath); err != nil {
		return false, fmt.Sprintf("mkcert.exe not found at %s", exePath)
	}
	cmd := exec.Command(exePath, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

// CAIsInstalled returns true if the mkcert CA is present in either the
// machine or user Root certificate store, matching ca_is_installed() in the
// Python service: it runs `certutil -store Root` and
// `certutil -user -store Root` and looks for a case-insensitive "mkcert"
// substring in either output. installDir is accepted for signature symmetry
// with the other functions in this file, but — same as in Python —
// certutil.exe is looked up on PATH and does not need it.
func CAIsInstalled(installDir string) bool {
	_ = installDir
	argSets := [][]string{
		{"-store", "Root"},
		{"-user", "-store", "Root"},
	}
	for _, args := range argSets {
		out, err := exec.Command("certutil", args...).CombinedOutput()
		if err == nil && strings.Contains(strings.ToLower(string(out)), "mkcert") {
			return true
		}
	}
	return false
}

// InstallCA installs the mkcert CA into the Windows, Chrome, Firefox and
// Edge trust stores (mkcert -install).
func InstallCA(installDir string) error {
	ok, out := runMkcert(installDir, "", "-install")
	if !ok {
		return fmt.Errorf("mkcert -install failed: %s", strings.TrimSpace(out))
	}
	return nil
}

// RemoveCA removes the mkcert CA from all trust stores (mkcert -uninstall).
func RemoveCA(installDir string) error {
	ok, out := runMkcert(installDir, "", "-uninstall")
	if !ok {
		return fmt.Errorf("mkcert -uninstall failed: %s", strings.TrimSpace(out))
	}
	return nil
}

// GenerateCerts generates server.key/server.crt for localhost + 127.0.0.1
// inside sslDir using mkcert, matching generate_certs() in the Python
// service. sslDir is created if it does not already exist.
func GenerateCerts(sslDir, installDir string) error {
	if err := os.MkdirAll(sslDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", sslDir, err)
	}
	ok, out := runMkcert(installDir, sslDir,
		"-key-file", "server.key", "-cert-file", "server.crt", "localhost", "127.0.0.1")
	if !ok {
		return fmt.Errorf("mkcert cert generation failed: %s", strings.TrimSpace(out))
	}
	return nil
}
