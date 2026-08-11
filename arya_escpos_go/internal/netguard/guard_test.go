package netguard

import (
	"testing"

	"aryaescpos/internal/config"
)

func baseCfg() config.NetworkScanConfig {
	return config.NetworkScanConfig{
		Enabled: true,
		Subnets: []string{"192.168.1.0/24"},
		Ports:   []int{9100, 9101},
		Timeout: 2,
	}
}

func TestAllowedSubnetAndPort(t *testing.T) {
	if err := Check(baseCfg(), "192.168.1.50", 9100); err != nil {
		t.Fatalf("Check() unexpected error for allowed host/port: %v", err)
	}
}

func TestHostOutsideSubnetRejected(t *testing.T) {
	if err := Check(baseCfg(), "10.0.0.5", 9100); err == nil {
		t.Fatal("Check() expected error for host outside configured subnets, got nil")
	}
}

func TestPortNotAllowedRejected(t *testing.T) {
	if err := Check(baseCfg(), "192.168.1.50", 12345); err == nil {
		t.Fatal("Check() expected error for disallowed port, got nil")
	}
}

func TestLoopbackRejectedEvenIfBroadSubnetWouldMatch(t *testing.T) {
	cfg := baseCfg()
	// A careless operator config that would otherwise cover the whole
	// internet, including loopback -- must still reject loopback explicitly.
	cfg.Subnets = []string{"0.0.0.0/0"}
	if err := Check(cfg, "127.0.0.1", 9100); err == nil {
		t.Fatal("Check() expected error for loopback host despite matching a broad subnet, got nil")
	}
}

func TestLoopbackAllowedWhenExplicitlyWhitelisted(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"127.0.0.0/8"}
	if err := Check(cfg, "127.0.0.1", 9100); err != nil {
		t.Fatalf("Check() unexpected error for explicitly whitelisted loopback: %v", err)
	}
}

func TestLinkLocalRejectedEvenIfBroadSubnetWouldMatch(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"169.0.0.0/8"} // covers 169.254.0.0/16 incidentally
	if err := Check(cfg, "169.254.169.254", 9100); err == nil {
		t.Fatal("Check() expected error for link-local host despite matching a broad subnet, got nil")
	}
}

func TestIPv6LoopbackRejected(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"::/0"}
	if err := Check(cfg, "::1", 9100); err == nil {
		t.Fatal("Check() expected error for ::1 despite matching a broad IPv6 subnet, got nil")
	}
}

func TestINADDRAnyRejectedEvenIfBroadSubnetWouldMatch(t *testing.T) {
	// connect() to 0.0.0.0 resolves to loopback on Windows and Linux, so a
	// careless 0.0.0.0/0 whitelist entry must not let 0.0.0.0 itself through
	// any more than it lets 127.0.0.1 through.
	cfg := baseCfg()
	cfg.Subnets = []string{"0.0.0.0/0"}
	if err := Check(cfg, "0.0.0.0", 9100); err == nil {
		t.Fatal("Check() expected error for 0.0.0.0 despite matching a broad subnet, got nil")
	}
}

func TestINADDRAnyAllowedWhenExplicitlyWhitelisted(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"0.0.0.0/8"}
	if err := Check(cfg, "0.0.0.0", 9100); err != nil {
		t.Fatalf("Check() unexpected error for explicitly whitelisted 0.0.0.0/8: %v", err)
	}
}

func TestIPv6LinkLocalRejectedEvenIfBroadSubnetWouldMatch(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"::/0"} // covers fe80::/10 incidentally
	if err := Check(cfg, "fe80::1", 9100); err == nil {
		t.Fatal("Check() expected error for IPv6 link-local host despite matching a broad subnet, got nil")
	}
}

func TestIPv6LinkLocalAllowedWhenExplicitlyWhitelisted(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"fe80::/10"}
	if err := Check(cfg, "fe80::1", 9100); err != nil {
		t.Fatalf("Check() unexpected error for explicitly whitelisted fe80::/10: %v", err)
	}
}

func TestDisabledGuardAllowsEverything(t *testing.T) {
	cfg := baseCfg()
	cfg.Enabled = false
	if err := Check(cfg, "127.0.0.1", 1); err != nil {
		t.Fatalf("Check() with Enabled=false unexpected error: %v", err)
	}
}

func TestHostnameRejected(t *testing.T) {
	if err := Check(baseCfg(), "printer.local", 9100); err == nil {
		t.Fatal("Check() expected error for a non-literal-IP host, got nil")
	}
}

func TestMalformedSubnetEntryIsSkippedNotFatal(t *testing.T) {
	cfg := baseCfg()
	cfg.Subnets = []string{"not-a-cidr", "192.168.1.0/24"}
	if err := Check(cfg, "192.168.1.50", 9100); err != nil {
		t.Fatalf("Check() unexpected error with one malformed subnet entry present: %v", err)
	}
}
