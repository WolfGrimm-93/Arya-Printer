package hwadapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	goserial "go.bug.st/serial"

	"aryaescpos/internal/contract"
)

// defaultBaudRate matches the Python service's SerialAdapter constructor
// default.
const defaultBaudRate = 9600

// defaultSerialReadTimeout matches pyserial's SerialAdapter(timeout=1)
// default: Read blocks for at most this long before returning whatever
// bytes (possibly zero) have arrived.
const defaultSerialReadTimeout = 1 * time.Second

// SerialAdapter implements contract.DeviceAdapter over a COM/tty serial
// port using go.bug.st/serial.
//
// Fixes the Python service's known bug (adapter_factory.py passes baudrate=
// kwargs.get("baudrate") to create_adapter("serial", ...), but
// SerialAdapter.open() never reads it — the port always opens at the
// constructor default of 9600 regardless of what the client requested):
// OpenParams.BaudRate is honored for real here.
type SerialAdapter struct {
	mu      sync.Mutex
	port    goserial.Port
	isOpen  bool
	comPort string
}

// NewSerialAdapter returns an unopened SerialAdapter.
func NewSerialAdapter() *SerialAdapter {
	return &SerialAdapter{}
}

func (a *SerialAdapter) Open(ctx context.Context, p contract.OpenParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isOpen {
		return fmt.Errorf("hwadapter: serial adapter already open")
	}
	if p.ComPort == "" {
		return contract.BadRequest("serial_port_required", "com_port is required for serial device")
	}

	baud := p.BaudRate
	if baud <= 0 {
		baud = defaultBaudRate
	}
	dataBits := p.ByteSize
	if dataBits <= 0 {
		dataBits = 8
	}

	mode := &goserial.Mode{
		BaudRate: baud,
		DataBits: dataBits,
		Parity:   parseSerialParity(p.Parity),
		StopBits: parseSerialStopBits(p.StopBits),
	}

	port, err := goserial.Open(p.ComPort, mode)
	if err != nil {
		return contract.BadGateway("serial_open_failed", fmt.Sprintf("failed to open serial port %s", p.ComPort), err.Error())
	}
	// Best-effort: not all platforms/drivers support a read timeout, but
	// pyserial's default (timeout=1s) is what the Python service always
	// used, so this keeps Read() behavior comparable.
	_ = port.SetReadTimeout(defaultSerialReadTimeout)

	a.port = port
	a.comPort = p.ComPort
	a.isOpen = true
	return nil
}

func parseSerialParity(p string) goserial.Parity {
	switch p {
	case "E":
		return goserial.EvenParity
	case "O":
		return goserial.OddParity
	default:
		return goserial.NoParity
	}
}

func parseSerialStopBits(sb int) goserial.StopBits {
	if sb == 2 {
		return goserial.TwoStopBits
	}
	return goserial.OneStopBit
}

func (a *SerialAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil
	}
	err := a.port.Close()
	a.port = nil
	a.isOpen = false
	return err
}

func (a *SerialAdapter) Write(data []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return 0, contract.BadGateway("serial_not_open", "serial port is not open", "")
	}
	n, err := a.port.Write(data)
	if err != nil {
		return n, contract.BadGateway("serial_write_failed", "failed to write to serial port", err.Error())
	}
	return n, nil
}

func (a *SerialAdapter) Read(size int) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil, contract.BadGateway("serial_not_open", "serial port is not open", "")
	}
	if size <= 0 {
		size = 1024
	}
	buf := make([]byte, size)
	n, err := a.port.Read(buf)
	if err != nil {
		return nil, contract.BadGateway("serial_read_failed", "failed to read from serial port", err.Error())
	}
	return buf[:n], nil
}

func (a *SerialAdapter) IsOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isOpen
}
