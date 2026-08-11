// Package printsvc composes internal/escpos, internal/escp, internal/hwadapter
// and internal/winspool into the contract.TicketPrinter, contract.MatrixPrinter
// and contract.DeviceScanner implementations that internal/apiserver consumes.
package printsvc

import (
	"fmt"
	"strconv"
)

// parseHexUint16 parses a VID/PID hex string (e.g. "04b8") into a *uint16,
// or returns (nil, nil) for an empty string — the "not specified" case that
// lets hwadapter.USBAdapter fall back to auto-detect.
func parseHexUint16(s string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid hex value %q: %w", s, err)
	}
	r := uint16(v)
	return &r, nil
}
