package printsvc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	goserial "go.bug.st/serial"

	"aryaescpos/internal/config"
	"aryaescpos/internal/contract"
	"aryaescpos/internal/hwadapter"
)

type deviceScanner struct {
	wq     contract.WindowsPrintQueue
	netCfg config.NetworkScanConfig
}

// NewDeviceScanner returns a contract.DeviceScanner combining
// winspool-backed Windows printer enumeration, hwadapter's libusb-backed
// USB printer-class enumeration, and go.bug.st/serial's COM port listing —
// replicating GET /devices/scan and GET /devices/{type}/{id}/status from
// device_routes.py.
func NewDeviceScanner(wq contract.WindowsPrintQueue, netCfg config.NetworkScanConfig) contract.DeviceScanner {
	return &deviceScanner{wq: wq, netCfg: netCfg}
}

// Scan aggregates all three transports. Matching _scan_windows_printers()'s
// per-section error handling in the Python service: a failure in any one
// section (Windows/USB/serial enumeration) is swallowed and simply yields
// no devices for that section — it never aborts the whole scan.
func (s *deviceScanner) Scan(ctx context.Context) (contract.ScanResponse, error) {
	var devices []contract.DeviceInfo

	if winPrinters, err := s.wq.ScanPrinters(ctx); err == nil {
		devices = append(devices, winPrinters...)
	}

	if usbPrinters, err := hwadapter.FindUSBPrinters(); err == nil {
		for _, p := range usbPrinters {
			devices = append(devices, contract.DeviceInfo{
				ID:           fmt.Sprintf("usb-%s:%s", p.VID, p.PID),
				Type:         "usb",
				Name:         p.Product,
				Manufacturer: p.Manufacturer,
				VID:          p.VID,
				PID:          p.PID,
				Serial:       p.Serial,
			})
		}
	}

	if ports, err := goserial.GetPortsList(); err == nil {
		for _, port := range ports {
			devices = append(devices, contract.DeviceInfo{
				ID:      "serial-" + port,
				Type:    "serial",
				Name:    port,
				ComPort: port,
			})
		}
	}

	return contract.ScanResponse{Found: len(devices), Devices: devices}, nil
}

// Status dispatches by deviceType, mirroring get_device_status() in
// device_routes.py.
func (s *deviceScanner) Status(ctx context.Context, deviceType, deviceID string) (contract.DeviceStatus, error) {
	switch deviceType {
	case "windows":
		printerName := strings.ReplaceAll(deviceID, "_", " ")
		return s.wq.PrinterStatus(ctx, printerName)

	case "usb":
		return s.usbStatus(ctx, deviceID), nil

	case "network":
		host, port, err := parseNetworkDeviceID(deviceID)
		if err != nil {
			return contract.DeviceStatus{}, contract.BadRequest("invalid_device_id", err.Error())
		}
		// netguard.Check runs inside TestConnection — this is the read-only
		// status probe the security audit flagged as an unauthenticated SSRF
		// oracle in the Python service (no validation at all there); here it
		// is gated exactly like every other network dial.
		reachable, err := hwadapter.TestConnection(s.netCfg, host, port, 0)
		if err != nil {
			return contract.DeviceStatus{}, err
		}
		status := "Unreachable"
		if reachable {
			status = "Reachable"
		}
		return contract.DeviceStatus{
			DeviceType: "network",
			Host:       host,
			Port:       port,
			IsOnline:   reachable,
			Status:     status,
		}, nil

	case "serial":
		portName := strings.ReplaceAll(deviceID, "_", "/")
		ports, err := goserial.GetPortsList()
		if err != nil {
			return contract.DeviceStatus{}, contract.BadGateway("serial_list_failed", "failed to list serial ports", err.Error())
		}
		isOnline := false
		for _, p := range ports {
			if p == portName {
				isOnline = true
				break
			}
		}
		status := "Not found"
		if isOnline {
			status = "Available"
		}
		return contract.DeviceStatus{
			DeviceType: "serial",
			ComPort:    portName,
			IsOnline:   isOnline,
			Status:     status,
		}, nil

	default:
		return contract.DeviceStatus{}, contract.BadRequest("unsupported_device_type", fmt.Sprintf("unsupported device type: %s", deviceType))
	}
}

// usbStatus queries an ESC/POS-over-USB printer's real-time status via
// DLE EOT (0x10 0x04 0x01), matching _get_escpos_status()'s usb branch:
// bit 3 of the response byte = offline, bit 5 = waiting for online
// recovery, bit 6 = paper feed button pressed. Any failure along the way
// (bad device id, device not found, write/read error) degrades to a
// "Cannot communicate" status rather than propagating an error — same as
// the Python service's broad except-and-report-offline behavior, since this
// is a best-effort live probe, not a request that should ever 500.
func (s *deviceScanner) usbStatus(ctx context.Context, vidPidID string) contract.DeviceStatus {
	cannotCommunicate := contract.DeviceStatus{
		DeviceType: "usb",
		IsOnline:   false,
		Status:     "Cannot communicate",
		Errors:     []string{"Communication error"},
	}

	parts := strings.SplitN(vidPidID, ":", 2)
	if len(parts) != 2 {
		return cannotCommunicate
	}
	vid64, err1 := strconv.ParseUint(parts[0], 16, 16)
	pid64, err2 := strconv.ParseUint(parts[1], 16, 16)
	if err1 != nil || err2 != nil {
		return cannotCommunicate
	}
	vid, pid := uint16(vid64), uint16(pid64)

	adapter := hwadapter.NewUSBAdapter()
	if err := adapter.Open(ctx, contract.OpenParams{VID: &vid, PID: &pid}); err != nil {
		return cannotCommunicate
	}
	defer adapter.Close()

	if _, err := adapter.Write([]byte{0x10, 0x04, 0x01}); err != nil {
		return cannotCommunicate
	}
	time.Sleep(100 * time.Millisecond)
	resp, err := adapter.Read(1)
	if err != nil || len(resp) == 0 {
		return cannotCommunicate
	}

	statusByte := resp[0]
	var errs, warnings, details []string
	if statusByte&(1<<3) != 0 {
		errs = append(errs, "Offline")
	} else {
		details = append(details, "Online")
	}
	if statusByte&(1<<5) != 0 {
		warnings = append(warnings, "Waiting for online recovery")
	}
	if statusByte&(1<<6) != 0 {
		details = append(details, "Paper feed button pressed")
	}

	isOnline := statusByte&(1<<3) == 0
	var statusText string
	switch {
	case len(errs) > 0:
		statusText = "Error: " + strings.Join(errs, ", ")
	case len(warnings) > 0:
		statusText = "Warning: " + strings.Join(warnings, ", ")
	default:
		statusText = "Ready"
	}

	return contract.DeviceStatus{
		DeviceType: "usb",
		IsOnline:   isOnline,
		Status:     statusText,
		Errors:     errs,
		Warnings:   warnings,
		Details:    details,
	}
}

// parseNetworkDeviceID splits "host:port" (port optional, default 9100),
// matching device_routes.py's `device_id.split(":")` handling.
func parseNetworkDeviceID(deviceID string) (string, int, error) {
	parts := strings.SplitN(deviceID, ":", 2)
	host := parts[0]
	port := 9100
	if len(parts) > 1 {
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("invalid port in device id %q", deviceID)
		}
		port = p
	}
	return host, port, nil
}
