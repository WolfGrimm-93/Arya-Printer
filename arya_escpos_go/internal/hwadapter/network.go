package hwadapter

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"aryaescpos/internal/config"
	"aryaescpos/internal/contract"
	"aryaescpos/internal/netguard"
)

// defaultNetworkPort matches the Python service's NetworkAdapter default
// (9100 is the de facto standard raw-TCP printing port).
const defaultNetworkPort = 9100

const defaultDialTimeout = 5 * time.Second

// NetworkAdapter implements contract.DeviceAdapter over plain TCP.
//
// Unlike the Python service's NetworkAdapter, Open ALWAYS calls
// internal/netguard.Check before dialing. The Python service dials any
// client-supplied host:port unconditionally — a critical SSRF finding from
// the security audit (a compromised or malicious Arya Market client could
// use this service as a network scanner/proxy against the operator's LAN).
// If the guard rejects the target, Open returns a 403 *contract.APIError
// without ever touching the network.
type NetworkAdapter struct {
	mu       sync.Mutex
	conn     net.Conn
	isOpen   bool
	guardCfg config.NetworkScanConfig
}

// NewNetworkAdapter returns a NetworkAdapter that enforces guardCfg on every
// Open call.
func NewNetworkAdapter(guardCfg config.NetworkScanConfig) *NetworkAdapter {
	return &NetworkAdapter{guardCfg: guardCfg}
}

func (a *NetworkAdapter) Open(ctx context.Context, p contract.OpenParams) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isOpen {
		return fmt.Errorf("hwadapter: network adapter already open")
	}
	if p.Host == "" {
		return contract.BadRequest("network_host_required", "host is required for network device")
	}

	port := p.Port
	if port == 0 {
		port = defaultNetworkPort
	}

	if err := netguard.Check(a.guardCfg, p.Host, port); err != nil {
		return contract.Forbidden("network_guard_rejected", err.Error())
	}

	timeout := p.DialTimeout
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", p.Host, port))
	if err != nil {
		return contract.BadGateway("network_connect_failed", fmt.Sprintf("failed to connect to %s:%d", p.Host, port), err.Error())
	}

	a.conn = conn
	a.isOpen = true
	return nil
}

func (a *NetworkAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil
	}
	err := a.conn.Close()
	a.conn = nil
	a.isOpen = false
	return err
}

func (a *NetworkAdapter) Write(data []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return 0, contract.BadGateway("network_not_open", "network device is not connected", "")
	}
	n, err := a.conn.Write(data)
	if err != nil {
		return n, contract.BadGateway("network_write_failed", "failed to write to network device", err.Error())
	}
	return n, nil
}

func (a *NetworkAdapter) Read(size int) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil, contract.BadGateway("network_not_open", "network device is not connected", "")
	}
	if size <= 0 {
		size = 1024
	}
	buf := make([]byte, size)
	n, err := a.conn.Read(buf)
	if err != nil {
		return nil, contract.BadGateway("network_read_failed", "failed to read from network device", err.Error())
	}
	return buf[:n], nil
}

func (a *NetworkAdapter) IsOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isOpen
}

// TestConnection reports whether host:port accepts a TCP connection within
// timeout, mirroring NetworkAdapter.test_connection() in the Python
// service. Unlike the Python original — which is called directly from the
// read-only GET /devices/network/{id}/status route with no validation at
// all, making that route an unauthenticated SSRF probe — this ALWAYS runs
// the target past netguard.Check first and returns a 403 *contract.APIError
// if it's rejected, instead of quietly reporting reachability for arbitrary
// hosts.
func TestConnection(cfg config.NetworkScanConfig, host string, port int, timeout time.Duration) (bool, error) {
	if err := netguard.Check(cfg, host, port); err != nil {
		return false, contract.Forbidden("network_guard_rejected", err.Error())
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false, nil
	}
	conn.Close()
	return true, nil
}
