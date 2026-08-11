// Package config defines the Config type loaded from configs/settings.yaml.
// Types here are frozen as part of Phase 0; internal/config/load.go (Agent A)
// implements the actual YAML loading against this shape.
package config

// Config mirrors arya_escpos/config/settings.yaml's structure, with two
// deliberate divergences from the Python service: Server.Host defaults to
// 127.0.0.1 instead of 0.0.0.0 (critical audit finding), and Security is a
// new block with no Python equivalent.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Logging     LoggingConfig     `yaml:"logging"`
	Discovery   DiscoveryConfig   `yaml:"discovery"`
	Devices     DeviceConfig      `yaml:"devices"`
	Printers    PrinterConfig     `yaml:"printers"`
	NetworkScan NetworkScanConfig `yaml:"network_scan"`
	Security    SecurityConfig    `yaml:"security"`
}

type ServerConfig struct {
	Host string `yaml:"host"` // default "127.0.0.1" — Python defaults to "0.0.0.0", audit finding, do not replicate
	Port int    `yaml:"port"` // default 58181
}

type LoggingConfig struct {
	Level         string `yaml:"level"`          // default "INFO"
	LogDir        string `yaml:"log_dir"`        // default "logs"
	RetentionDays int    `yaml:"retention_days"` // default 15
}

type DiscoveryConfig struct {
	AutoScanInterval int  `yaml:"auto_scan_interval"` // seconds, default 30
	USBEnabled       bool `yaml:"usb_enabled"`        // default true
	NetworkEnabled   bool `yaml:"network_enabled"`    // default true
	SerialEnabled    bool `yaml:"serial_enabled"`     // default true
	// Bluetooth intentionally omitted: out of scope for this v1 (already
	// incomplete/non-functional in the Python service).
}

type DeviceConfig struct {
	AutoReconnect     bool `yaml:"auto_reconnect"`     // default true
	ConnectionTimeout int  `yaml:"connection_timeout"` // seconds, default 5
	RetryAttempts     int  `yaml:"retry_attempts"`     // default 3
}

type PrinterConfig struct {
	PaperWidth int `yaml:"paper_width"` // mm, default 80
	// DefaultEncoding intentionally omitted: dead config in the Python
	// service (never consulted anywhere). ESC/POS encoding is hardcoded to
	// cp850 same as Python; ESC/P encoding comes from the per-request
	// field, not global config.
}

// NetworkScanConfig bounds which host:port a "network" type PrintRequest may
// target. In the Python service this block exists in settings.yaml but is
// never enforced (SSRF gap, audit finding C3/hallazgo 1 consolidado). In Go,
// internal/netguard (Agent B) uses this as the actual allowlist checked
// before dialing — this is the first time the block has real effect.
type NetworkScanConfig struct {
	Enabled bool     `yaml:"enabled"` // default true
	Subnets []string `yaml:"subnets"` // CIDR strings, default ["192.168.1.0/24"]
	Ports   []int    `yaml:"ports"`   // default [9100, 9101]
	Timeout int      `yaml:"timeout"` // seconds, default 2
}

// SecurityConfig has no Python equivalent — it exists to hold the security
// fixes baked in from the audit (auth, upload limits). CORS itself has no
// config here: it accepts any origin, see internal/middleware/cors.go.
type SecurityConfig struct {
	AuthEnabled    bool   `yaml:"auth_enabled"`     // default true; API key required on all /api/v1/* routes except none (only GET / and GET /health are exempt, hardcoded, not configurable)
	APIKeyPath     string `yaml:"api_key_path"`     // default "secrets/apikey.key", relative to install dir
	MaxUploadBytes int64  `yaml:"max_upload_bytes"` // default 52428800 (50 MB); applies to POST /api/v1/print/document
	MaxImageBytes  int64  `yaml:"max_image_bytes"`  // default 5242880 (5 MB); applies to decoded PrintRequest.HeaderImage
}

// Default returns a Config populated with the same defaults the Python
// service ships in config/settings.yaml, except Server.Host (see above) and
// Security (new block, no Python equivalent). Load() (Agent A) must start
// from this and overlay whatever configs/settings.yaml provides, the same
// way the Python service's Config.from_yaml does per-section instantiation.
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 58181,
		},
		Logging: LoggingConfig{
			Level:         "INFO",
			LogDir:        "logs",
			RetentionDays: 15,
		},
		Discovery: DiscoveryConfig{
			AutoScanInterval: 30,
			USBEnabled:       true,
			NetworkEnabled:   true,
			SerialEnabled:    true,
		},
		Devices: DeviceConfig{
			AutoReconnect:     true,
			ConnectionTimeout: 5,
			RetryAttempts:     3,
		},
		Printers: PrinterConfig{
			PaperWidth: 80,
		},
		NetworkScan: NetworkScanConfig{
			Enabled: true,
			Subnets: []string{"192.168.1.0/24"},
			Ports:   []int{9100, 9101},
			Timeout: 2,
		},
		Security: SecurityConfig{
			AuthEnabled:    true,
			APIKeyPath:     "secrets/apikey.key",
			MaxUploadBytes: 50 * 1024 * 1024,
			MaxImageBytes:  5 * 1024 * 1024,
		},
	}
}
