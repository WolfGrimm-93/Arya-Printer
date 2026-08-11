package contract

import (
	"context"
	"time"
)

// OpenParams carries the connection parameters needed to open a hardware
// adapter, regardless of transport. Only the fields relevant to the chosen
// device type need to be set; adapters ignore fields outside their transport.
type OpenParams struct {
	// USB. Both VID and PID must be set together to target a specific
	// device; leave both nil to auto-detect the first USB printer-class
	// device found. This propagation is REQUIRED to work correctly in Go —
	// the Python service has a known bug (adapter_factory.py) where
	// vid/pid are accepted by create_adapter() but never actually reach
	// USBAdapter.open(), so it always auto-detects regardless of client
	// input. Do not replicate that bug.
	VID *uint16
	PID *uint16

	// Network (TCP). Host/Port must already be validated by internal/netguard
	// against the configured allowlist before this is passed to Open — the
	// adapter itself does not re-check.
	Host        string
	Port        int
	DialTimeout time.Duration

	// Serial. BaudRate MUST be honored — the Python service has a known bug
	// where adapter_factory.create_adapter("serial", ...) never forwards
	// baudrate to SerialAdapter.open(), so client-requested baud rates are
	// silently ignored and the port always runs at the constructor default
	// (9600). Do not replicate that bug.
	ComPort  string
	BaudRate int
	ByteSize int
	Parity   string // "N"|"E"|"O"
	StopBits int
}

// DeviceAdapter is the common interface implemented by every hardware
// transport (USB, network/TCP, serial). Bluetooth is out of scope for v1.
type DeviceAdapter interface {
	Open(ctx context.Context, p OpenParams) error
	Close() error
	Write(data []byte) (int, error)
	Read(size int) ([]byte, error)
	IsOpen() bool
}

// AdapterFactory constructs and opens a DeviceAdapter for the given device
// type ("usb"|"network"|"serial").
type AdapterFactory interface {
	Create(ctx context.Context, deviceType string, p OpenParams) (DeviceAdapter, error)
}
