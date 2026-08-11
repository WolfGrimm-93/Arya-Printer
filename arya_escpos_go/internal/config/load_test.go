package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoad_MissingFile verifies that a nonexistent path returns Default()
// with no error — a deliberate divergence from the Python service, which
// raised ConfigurationError in this case.
func TestLoad_MissingFile(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("Load() = %+v, want Default() = %+v", got, Default())
	}
}

// TestLoad_FullDefaultYAML writes out the repo's configs/settings.yaml
// equivalent (regenerated from Default()) and checks it round-trips.
func TestLoad_FullDefaultYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	writeYAML(t, path, `
server:
  host: "127.0.0.1"
  port: 58181
logging:
  level: "INFO"
  log_dir: "logs"
  retention_days: 15
discovery:
  auto_scan_interval: 30
  usb_enabled: true
  network_enabled: true
  serial_enabled: true
devices:
  auto_reconnect: true
  connection_timeout: 5
  retry_attempts: 3
printers:
  paper_width: 80
network_scan:
  enabled: true
  subnets:
    - "192.168.1.0/24"
  ports:
    - 9100
    - 9101
  timeout: 2
security:
  auth_enabled: true
  api_key_path: "secrets/apikey.key"
  max_upload_bytes: 52428800
  max_image_bytes: 5242880
`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("Load() = %+v, want Default() = %+v", got, Default())
	}
}

// TestLoad_PartialYAML verifies that sections absent from the YAML keep
// their default values, matching the Python service's per-section
// instantiation in Config.from_yaml, while sections present override
// entirely.
func TestLoad_PartialYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.yaml")
	writeYAML(t, path, `
server:
  host: "0.0.0.0"
  port: 9999
security:
  auth_enabled: false
  api_key_path: "secrets/apikey.key"
  max_upload_bytes: 1024
  max_image_bytes: 512
`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	want := Default()
	want.Server = ServerConfig{Host: "0.0.0.0", Port: 9999}
	want.Security = SecurityConfig{
		AuthEnabled:    false,
		APIKeyPath:     "secrets/apikey.key",
		MaxUploadBytes: 1024,
		MaxImageBytes:  512,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}

	// Sections not present in the YAML (logging, discovery, devices,
	// printers, network_scan) must remain at their Default() values.
	def := Default()
	if !reflect.DeepEqual(got.Logging, def.Logging) {
		t.Errorf("Logging = %+v, want default %+v", got.Logging, def.Logging)
	}
	if !reflect.DeepEqual(got.Discovery, def.Discovery) {
		t.Errorf("Discovery = %+v, want default %+v", got.Discovery, def.Discovery)
	}
	if !reflect.DeepEqual(got.Devices, def.Devices) {
		t.Errorf("Devices = %+v, want default %+v", got.Devices, def.Devices)
	}
	if !reflect.DeepEqual(got.Printers, def.Printers) {
		t.Errorf("Printers = %+v, want default %+v", got.Printers, def.Printers)
	}
	if !reflect.DeepEqual(got.NetworkScan, def.NetworkScan) {
		t.Errorf("NetworkScan = %+v, want default %+v", got.NetworkScan, def.NetworkScan)
	}
}

// TestLoad_PartialSecuritySection_KeepsAuthEnabledDefault is a regression
// test for a real bug found in code review: an earlier Load() replaced a
// whole section as soon as ANY one of its keys was present in the YAML
// (`if raw.Security != nil { cfg.Security = *raw.Security }`). An operator
// editing configs/settings.yaml to touch only security.allowed_origins,
// without repeating auth_enabled, would silently zero it out (Go's bool
// zero value is false) — disabling authentication with no error or
// warning, reintroducing audit finding C1. Load must merge field by field:
// a key genuinely absent from a present section keeps its Default() value.
func TestLoad_PartialSecuritySection_KeepsAuthEnabledDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial-security.yaml")
	writeYAML(t, path, `
security:
  max_upload_bytes: 1024
`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if !got.Security.AuthEnabled {
		t.Fatal("Security.AuthEnabled = false, want true (default) — a partial security: block must not silently disable auth")
	}
	if got.Security.APIKeyPath != Default().Security.APIKeyPath {
		t.Errorf("Security.APIKeyPath = %q, want default %q", got.Security.APIKeyPath, Default().Security.APIKeyPath)
	}
	if got.Security.MaxImageBytes != Default().Security.MaxImageBytes {
		t.Errorf("Security.MaxImageBytes = %d, want default %d", got.Security.MaxImageBytes, Default().Security.MaxImageBytes)
	}
	// The one field actually present in the YAML must still take effect.
	if got.Security.MaxUploadBytes != 1024 {
		t.Errorf("Security.MaxUploadBytes = %d, want 1024", got.Security.MaxUploadBytes)
	}

	// Every other section, entirely absent from the YAML, must be untouched.
	def := Default()
	if got.Server != def.Server {
		t.Errorf("Server = %+v, want default %+v", got.Server, def.Server)
	}
}

// TestLoad_PartialServerSection_KeepsPortDefault mirrors the same
// regression for a non-security section: overriding only server.host must
// not zero out server.port.
func TestLoad_PartialServerSection_KeepsPortDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial-server.yaml")
	writeYAML(t, path, `
server:
  host: "0.0.0.0"
`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if got.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want %q", got.Server.Host, "0.0.0.0")
	}
	if got.Server.Port != Default().Server.Port {
		t.Errorf("Server.Port = %d, want default %d (must not be zeroed out by a partial server: block)", got.Server.Port, Default().Server.Port)
	}
}

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s): %v", path, err)
	}
}
