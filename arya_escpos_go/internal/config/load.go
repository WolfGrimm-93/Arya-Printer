package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Every raw* type below mirrors one Config section with every field turned
// into a pointer, so yaml.v3 can tell "key absent from the YAML" (nil) apart
// from "key present with its zero value" (non-nil, pointing at zero).
//
// This is a field-level merge, not a section-level one: an earlier version
// of this file did `if raw.Security != nil { cfg.Security = *raw.Security }`,
// which replaced the ENTIRE section as soon as any one of its keys was
// present. An operator editing configs/settings.yaml to change only
// security.allowed_origins, without repeating auth_enabled, would silently
// zero out auth_enabled (Go's bool zero value is false) — disabling
// authentication with no error or warning, reintroducing audit finding C1.
// Merging field by field means an omitted key always keeps Default()'s
// value, no matter which of its siblings are present.
type rawServerConfig struct {
	Host *string `yaml:"host"`
	Port *int    `yaml:"port"`
}

type rawLoggingConfig struct {
	Level         *string `yaml:"level"`
	LogDir        *string `yaml:"log_dir"`
	RetentionDays *int    `yaml:"retention_days"`
}

type rawDiscoveryConfig struct {
	AutoScanInterval *int  `yaml:"auto_scan_interval"`
	USBEnabled       *bool `yaml:"usb_enabled"`
	NetworkEnabled   *bool `yaml:"network_enabled"`
	SerialEnabled    *bool `yaml:"serial_enabled"`
}

type rawDeviceConfig struct {
	AutoReconnect     *bool `yaml:"auto_reconnect"`
	ConnectionTimeout *int  `yaml:"connection_timeout"`
	RetryAttempts     *int  `yaml:"retry_attempts"`
}

type rawPrinterConfig struct {
	PaperWidth *int `yaml:"paper_width"`
}

type rawNetworkScanConfig struct {
	Enabled *bool     `yaml:"enabled"`
	Subnets *[]string `yaml:"subnets"`
	Ports   *[]int    `yaml:"ports"`
	Timeout *int      `yaml:"timeout"`
}

type rawSecurityConfig struct {
	AuthEnabled    *bool   `yaml:"auth_enabled"`
	APIKeyPath     *string `yaml:"api_key_path"`
	MaxUploadBytes *int64  `yaml:"max_upload_bytes"`
	MaxImageBytes  *int64  `yaml:"max_image_bytes"`
}

// rawConfig mirrors Config with every section (and every field within each
// section) turned into a pointer, so Load can overlay configs/settings.yaml
// onto config.Default() one field at a time.
type rawConfig struct {
	Server      *rawServerConfig      `yaml:"server"`
	Logging     *rawLoggingConfig     `yaml:"logging"`
	Discovery   *rawDiscoveryConfig   `yaml:"discovery"`
	Devices     *rawDeviceConfig      `yaml:"devices"`
	Printers    *rawPrinterConfig     `yaml:"printers"`
	NetworkScan *rawNetworkScanConfig `yaml:"network_scan"`
	Security    *rawSecurityConfig    `yaml:"security"`
}

// Load reads and parses the YAML file at path, overlaying it field by field
// onto config.Default(). Any key missing from the YAML — whether its whole
// section is absent or just that one key within a present section — keeps
// its default value untouched.
//
// Unlike the Python service, a missing file is not an error: Load returns
// Default() so the agent can still start with safe defaults rather than
// refuse to boot.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}

	mergeServer(&cfg.Server, raw.Server)
	mergeLogging(&cfg.Logging, raw.Logging)
	mergeDiscovery(&cfg.Discovery, raw.Discovery)
	mergeDevices(&cfg.Devices, raw.Devices)
	mergePrinters(&cfg.Printers, raw.Printers)
	mergeNetworkScan(&cfg.NetworkScan, raw.NetworkScan)
	mergeSecurity(&cfg.Security, raw.Security)

	return cfg, nil
}

func mergeServer(dst *ServerConfig, raw *rawServerConfig) {
	if raw == nil {
		return
	}
	if raw.Host != nil {
		dst.Host = *raw.Host
	}
	if raw.Port != nil {
		dst.Port = *raw.Port
	}
}

func mergeLogging(dst *LoggingConfig, raw *rawLoggingConfig) {
	if raw == nil {
		return
	}
	if raw.Level != nil {
		dst.Level = *raw.Level
	}
	if raw.LogDir != nil {
		dst.LogDir = *raw.LogDir
	}
	if raw.RetentionDays != nil {
		dst.RetentionDays = *raw.RetentionDays
	}
}

func mergeDiscovery(dst *DiscoveryConfig, raw *rawDiscoveryConfig) {
	if raw == nil {
		return
	}
	if raw.AutoScanInterval != nil {
		dst.AutoScanInterval = *raw.AutoScanInterval
	}
	if raw.USBEnabled != nil {
		dst.USBEnabled = *raw.USBEnabled
	}
	if raw.NetworkEnabled != nil {
		dst.NetworkEnabled = *raw.NetworkEnabled
	}
	if raw.SerialEnabled != nil {
		dst.SerialEnabled = *raw.SerialEnabled
	}
}

func mergeDevices(dst *DeviceConfig, raw *rawDeviceConfig) {
	if raw == nil {
		return
	}
	if raw.AutoReconnect != nil {
		dst.AutoReconnect = *raw.AutoReconnect
	}
	if raw.ConnectionTimeout != nil {
		dst.ConnectionTimeout = *raw.ConnectionTimeout
	}
	if raw.RetryAttempts != nil {
		dst.RetryAttempts = *raw.RetryAttempts
	}
}

func mergePrinters(dst *PrinterConfig, raw *rawPrinterConfig) {
	if raw == nil {
		return
	}
	if raw.PaperWidth != nil {
		dst.PaperWidth = *raw.PaperWidth
	}
}

func mergeNetworkScan(dst *NetworkScanConfig, raw *rawNetworkScanConfig) {
	if raw == nil {
		return
	}
	if raw.Enabled != nil {
		dst.Enabled = *raw.Enabled
	}
	if raw.Subnets != nil {
		dst.Subnets = *raw.Subnets
	}
	if raw.Ports != nil {
		dst.Ports = *raw.Ports
	}
	if raw.Timeout != nil {
		dst.Timeout = *raw.Timeout
	}
}

func mergeSecurity(dst *SecurityConfig, raw *rawSecurityConfig) {
	if raw == nil {
		return
	}
	if raw.AuthEnabled != nil {
		dst.AuthEnabled = *raw.AuthEnabled
	}
	if raw.APIKeyPath != nil {
		dst.APIKeyPath = *raw.APIKeyPath
	}
	if raw.MaxUploadBytes != nil {
		dst.MaxUploadBytes = *raw.MaxUploadBytes
	}
	if raw.MaxImageBytes != nil {
		dst.MaxImageBytes = *raw.MaxImageBytes
	}
}
