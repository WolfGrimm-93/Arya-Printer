// Package netguard is the SSRF allowlist gate for "network" (TCP) device
// connections. The Python service (arya_escpos) has no equivalent: any
// client-supplied host:port is dialed directly, making the /print and
// /devices/network/{id}/status endpoints an SSRF oracle against the
// operator's LAN (and, if the host field ever accepted anything DNS could
// resolve, potentially further). Check is the fix — every "network" adapter
// open and every network status probe MUST call it before dialing.
package netguard

import (
	"fmt"
	"net"

	"aryaescpos/internal/config"
)

// reservedRanges are always rejected, even when a broader configured subnet
// would otherwise match, unless the operator has explicitly listed that
// exact reserved range (or a subset of it) in cfg.Subnets — see
// subnetIsExplicitReservedException.
var reservedRanges = mustParseReserved(
	"0.0.0.0/8",      // IPv4 "this network" / INADDR_ANY — resolves to loopback on connect() on Windows and Linux
	"127.0.0.0/8",    // IPv4 loopback
	"169.254.0.0/16", // IPv4 link-local (cloud metadata endpoints live here)
	"::1/128",        // IPv6 loopback
	"fe80::/10",      // IPv6 link-local
)

func mustParseReserved(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("netguard: invalid reserved CIDR literal " + c + ": " + err.Error())
		}
		nets = append(nets, n)
	}
	return nets
}

// Check validates that (host, port) may be dialed as a "network" device
// according to cfg.
//
//   - cfg.Enabled == false disables all checks (returns nil unconditionally).
//     This is an explicit operator opt-out, not a default — treat it as an
//     accepted risk when documenting deployments that set it.
//   - host must be a literal IP address. Hostnames are intentionally
//     rejected: resolving them here would open a DNS-rebinding
//     TOCTOU gap (the name could resolve to an allowed IP at check time and
//     a disallowed one — e.g. loopback or a cloud metadata address — at
//     dial time).
//   - The resolved IP must fall within at least one of cfg.Subnets.
//   - port must be one of cfg.Ports.
//   - Loopback (127.0.0.0/8, ::1/128), "this network"/INADDR_ANY (0.0.0.0/8
//     — connecting to 0.0.0.0 resolves to loopback on Windows and Linux) and
//     link-local (169.254.0.0/16, fe80::/10) addresses are rejected even if
//     a broader configured subnet happens to contain them (e.g. a careless
//     0.0.0.0/0 entry), UNLESS the operator has explicitly listed that
//     reserved range itself (or a subset of it) as one of cfg.Subnets — an
//     intentional, auditable exception.
func Check(cfg config.NetworkScanConfig, host string, port int) error {
	if !cfg.Enabled {
		return nil
	}

	if !portAllowed(port, cfg.Ports) {
		return fmt.Errorf("netguard: port %d is not in the allowed list %v", port, cfg.Ports)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("netguard: host %q is not a literal IP address (hostname resolution is intentionally unsupported)", host)
	}

	// matchedReserved is the *specific* reserved range ip falls in (nil if
	// none). The exception check below deliberately scopes to this one
	// range, not "any reserved range" — see isSubnetWithinReservedRange's
	// doc comment for why a looser check is unsafe.
	matchedReserved := reservedRangeContaining(ip)
	matchedSubnet := false
	matchedExplicitException := false

	for _, cidr := range cfg.Subnets {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue // malformed config entries are skipped, not fatal here
		}
		if network.Contains(ip) {
			matchedSubnet = true
		}
		if matchedReserved != nil && isSubnetWithinReservedRange(network, matchedReserved) {
			matchedExplicitException = true
		}
	}

	if matchedReserved != nil && !matchedExplicitException {
		return fmt.Errorf("netguard: host %q (%s) is a loopback/link-local address, which is always blocked unless explicitly whitelisted in network_scan.subnets", host, ip)
	}
	if !matchedSubnet {
		return fmt.Errorf("netguard: host %q (%s) is outside all configured subnets %v", host, ip, cfg.Subnets)
	}
	return nil
}

func portAllowed(port int, allowed []int) bool {
	for _, p := range allowed {
		if p == port {
			return true
		}
	}
	return false
}

// reservedRangeContaining returns the reserved range containing ip, or nil
// if ip isn't reserved.
func reservedRangeContaining(ip net.IP) *net.IPNet {
	for _, r := range reservedRanges {
		if r.Contains(ip) {
			return r
		}
	}
	return nil
}

// isSubnetWithinReservedRange reports whether subnet is entirely contained
// within reserved — i.e. every address subnet covers is also covered by
// reserved — which is the correct test for "the operator explicitly
// whitelisted (a subset of) this exact reserved range."
//
// This must NOT be loosened to "subnet's network address merely falls
// somewhere inside some reserved range" (an earlier version of this
// function did exactly that): a catch-all entry like "0.0.0.0/0" has network
// address 0.0.0.0, which — once 0.0.0.0/8 was added to reservedRanges below
// — is itself contained by 0.0.0.0/8. Checking containment of the network
// address alone, without also requiring the prefix to be at least as
// narrow, would make ANY "0.0.0.0/0" whitelist entry look like an
// intentional exception for every reserved category (loopback, link-local,
// all of it) instead of the operator-scoped exception it's meant to be — a
// regression that TestINADDRAnyRejectedEvenIfBroadSubnetWouldMatch and
// TestLoopbackRejectedEvenIfBroadSubnetWouldMatch both catch.
func isSubnetWithinReservedRange(subnet, reserved *net.IPNet) bool {
	if !reserved.Contains(subnet.IP) {
		return false
	}
	subnetOnes, subnetBits := subnet.Mask.Size()
	reservedOnes, reservedBits := reserved.Mask.Size()
	if subnetBits != reservedBits {
		return false // different address families
	}
	return subnetOnes >= reservedOnes
}
