package hwadapter

import (
	"context"
	"fmt"

	"aryaescpos/internal/config"
	"aryaescpos/internal/contract"
)

// Factory implements contract.AdapterFactory, dispatching to USBAdapter,
// NetworkAdapter or SerialAdapter based on deviceType and opening it before
// returning — same contract as the Python service's create_adapter().
// Bluetooth is intentionally not handled here (out of scope for this v1,
// see contract.OpenParams doc comment).
type Factory struct {
	NetworkGuardConfig config.NetworkScanConfig
}

// NewFactory returns a Factory that enforces netGuardCfg on every "network"
// device Create call.
func NewFactory(netGuardCfg config.NetworkScanConfig) *Factory {
	return &Factory{NetworkGuardConfig: netGuardCfg}
}

func (f *Factory) Create(ctx context.Context, deviceType string, p contract.OpenParams) (contract.DeviceAdapter, error) {
	switch deviceType {
	case "usb":
		a := NewUSBAdapter()
		if err := a.Open(ctx, p); err != nil {
			return nil, err
		}
		return a, nil

	case "network":
		a := NewNetworkAdapter(f.NetworkGuardConfig)
		if err := a.Open(ctx, p); err != nil {
			return nil, err
		}
		return a, nil

	case "serial":
		a := NewSerialAdapter()
		if err := a.Open(ctx, p); err != nil {
			return nil, err
		}
		return a, nil

	default:
		return nil, contract.BadRequest("unsupported_device_type", fmt.Sprintf("unsupported device type: %s", deviceType))
	}
}
