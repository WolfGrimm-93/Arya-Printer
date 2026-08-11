// Package hwadapter implements contract.DeviceAdapter for the three
// hardware transports the service supports directly (USB, network/TCP,
// serial), plus contract.AdapterFactory to dispatch between them.
package hwadapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"aryaescpos/internal/contract"
)

// usbClassPrinter is the USB Printer Class code (0x07), used to identify
// printer-class devices during auto-detect, matching the Python service's
// USB_CLASS_PRINTER constant.
const usbClassPrinter = 0x07

// --- libusb-1.0 ABI structs -------------------------------------------------
//
// These mirror the C structs in libusb.h field-for-field. libusb.h does not
// use #pragma pack for these types, so the platform's natural C alignment
// rules apply — which are the same rules the Go compiler applies to these
// struct definitions, so no manual padding is needed as long as field order
// and widths match exactly. Pointer-sized fields are declared as uintptr
// (not typed Go pointers) because they point into memory libusb owns; they
// are only ever read via unsafe.Pointer/unsafe.Slice, never passed back to
// Go's GC as live pointers.

type libusbDeviceDescriptor struct {
	BLength            uint8
	BDescriptorType    uint8
	BcdUSB             uint16
	BDeviceClass       uint8
	BDeviceSubClass    uint8
	BDeviceProtocol    uint8
	BMaxPacketSize0    uint8
	IdVendor           uint16
	IdProduct          uint16
	BcdDevice          uint16
	IManufacturer      uint8
	IProduct           uint8
	ISerialNumber      uint8
	BNumConfigurations uint8
}

type libusbInterfaceDescriptor struct {
	BLength            uint8
	BDescriptorType    uint8
	BInterfaceNumber   uint8
	BAlternateSetting  uint8
	BNumEndpoints      uint8
	BInterfaceClass    uint8
	BInterfaceSubClass uint8
	BInterfaceProtocol uint8
	IInterface         uint8
	Endpoint           uintptr // const struct libusb_endpoint_descriptor*
	Extra              uintptr // const unsigned char*
	ExtraLength        int32
}

type libusbInterface struct {
	// Altsetting and Interface (below) are declared as unsafe.Pointer rather
	// than uintptr — even though both are equally valid for matching the C
	// struct's layout — because both are dereferenced later via
	// unsafe.Slice. Storing them as unsafe.Pointer keeps every later access
	// a Pointer-to-Pointer conversion instead of a uintptr-to-Pointer one,
	// which is what `go vet`'s unsafeptr check (correctly) flags as
	// suspicious when the uintptr didn't come from an immediately-preceding
	// Pointer conversion in the same expression.
	Altsetting    unsafe.Pointer // const struct libusb_interface_descriptor*
	NumAltsetting int32
}

type libusbConfigDescriptor struct {
	BLength             uint8
	BDescriptorType     uint8
	WTotalLength        uint16
	BNumInterfaces      uint8
	BConfigurationValue uint8
	IConfiguration      uint8
	BmAttributes        uint8
	MaxPower            uint8
	Interface           unsafe.Pointer // const struct libusb_interface*
	Extra               uintptr
	ExtraLength         int32
}

// --- lazy DLL binding --------------------------------------------------

var (
	libusbDLL  *windows.LazyDLL
	libusbOnce sync.Once
	libusbErr  error

	procLibusbInit                      *windows.LazyProc
	procLibusbExit                      *windows.LazyProc
	procLibusbGetDeviceList             *windows.LazyProc
	procLibusbFreeDeviceList            *windows.LazyProc
	procLibusbGetDeviceDescriptor       *windows.LazyProc
	procLibusbGetActiveConfigDescriptor *windows.LazyProc
	procLibusbFreeConfigDescriptor      *windows.LazyProc
	procLibusbOpen                      *windows.LazyProc
	procLibusbClose                     *windows.LazyProc
	procLibusbClaimInterface            *windows.LazyProc
	procLibusbReleaseInterface          *windows.LazyProc
	procLibusbSetConfiguration          *windows.LazyProc
	procLibusbBulkTransfer              *windows.LazyProc
	procLibusbGetStringDescriptorAscii  *windows.LazyProc
)

// libusbDLLPath resolves libs/libusb-1.0.dll relative to the running
// executable first (the layout this service ships with); if that file
// doesn't exist, it falls back to the bare DLL name so the standard Windows
// DLL search order (PATH, System32, etc.) gets a chance too.
func libusbDLLPath() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "libs", "libusb-1.0.dll")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}
	return "libusb-1.0.dll"
}

func loadLibusb() error {
	libusbOnce.Do(func() {
		libusbDLL = windows.NewLazyDLL(libusbDLLPath())
		if err := libusbDLL.Load(); err != nil {
			libusbErr = fmt.Errorf("hwadapter: failed to load libusb-1.0.dll: %w", err)
			return
		}
		procLibusbInit = libusbDLL.NewProc("libusb_init")
		procLibusbExit = libusbDLL.NewProc("libusb_exit")
		procLibusbGetDeviceList = libusbDLL.NewProc("libusb_get_device_list")
		procLibusbFreeDeviceList = libusbDLL.NewProc("libusb_free_device_list")
		procLibusbGetDeviceDescriptor = libusbDLL.NewProc("libusb_get_device_descriptor")
		procLibusbGetActiveConfigDescriptor = libusbDLL.NewProc("libusb_get_active_config_descriptor")
		procLibusbFreeConfigDescriptor = libusbDLL.NewProc("libusb_free_config_descriptor")
		procLibusbOpen = libusbDLL.NewProc("libusb_open")
		procLibusbClose = libusbDLL.NewProc("libusb_close")
		procLibusbClaimInterface = libusbDLL.NewProc("libusb_claim_interface")
		procLibusbReleaseInterface = libusbDLL.NewProc("libusb_release_interface")
		procLibusbSetConfiguration = libusbDLL.NewProc("libusb_set_configuration")
		procLibusbBulkTransfer = libusbDLL.NewProc("libusb_bulk_transfer")
		procLibusbGetStringDescriptorAscii = libusbDLL.NewProc("libusb_get_string_descriptor_ascii")
	})
	return libusbErr
}

// --- USBAdapter --------------------------------------------------------

// USBAdapter implements contract.DeviceAdapter over USB using libusb-1.0
// via NewLazyDLL bindings (no CGO). Each adapter owns its own libusb
// "default context" session (libusb_init(NULL)/libusb_exit(NULL) are
// refcounted by libusb itself, so multiple adapters can coexist safely).
//
// Fixes the Python service's known bug (adapter_factory.py never forwards
// vid/pid into USBAdapter.open()): when both VID and PID are set in
// OpenParams, Open opens exactly that device, matched via
// libusb_get_device_descriptor — never a different auto-detected one.
type USBAdapter struct {
	mu           sync.Mutex
	handle       uintptr
	isOpen       bool
	vid, pid     uint16
	interfaceNum int
	inEP, outEP  byte
}

// NewUSBAdapter returns a USBAdapter using the same interface/endpoint
// defaults as the Python service's USBAdapter: interface 0, in_ep 0x82,
// out_ep 0x01.
func NewUSBAdapter() *USBAdapter {
	return &USBAdapter{interfaceNum: 0, inEP: 0x82, outEP: 0x01}
}

func (a *USBAdapter) Open(ctx context.Context, p contract.OpenParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := loadLibusb(); err != nil {
		return contract.ServiceUnavailable("usb_dll_unavailable", "libusb-1.0.dll could not be loaded", err.Error())
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isOpen {
		return fmt.Errorf("hwadapter: USB adapter already open")
	}

	r1, _, _ := procLibusbInit.Call(0)
	if int32(r1) != 0 {
		return contract.BadGateway("usb_init_failed", fmt.Sprintf("libusb_init failed (code %d)", int32(r1)), "")
	}

	handle, vid, pid, err := findAndOpenUSBDevice(p.VID, p.PID)
	if err != nil {
		procLibusbExit.Call(0)
		return err
	}

	// Best-effort: matches Python's try/except around set_configuration()
	// (device already configured is a common, harmless failure on Windows).
	procLibusbSetConfiguration.Call(handle, 1)

	r1, _, _ = procLibusbClaimInterface.Call(handle, uintptr(a.interfaceNum))
	if int32(r1) != 0 {
		procLibusbClose.Call(handle)
		procLibusbExit.Call(0)
		return contract.BadGateway("usb_claim_interface_failed", fmt.Sprintf("libusb_claim_interface failed (code %d)", int32(r1)), "")
	}

	a.handle = handle
	a.vid, a.pid = vid, pid
	a.isOpen = true
	return nil
}

// findAndOpenUSBDevice enumerates USB devices and opens the one matching
// vid/pid (both non-nil) or, if either is nil, the first USB printer-class
// device found — same auto-detect semantics as USBAdapter.find_printers()
// in the Python service, but only actually reached when the caller did NOT
// specify a target device.
func findAndOpenUSBDevice(vid, pid *uint16) (handle uintptr, foundVID, foundPID uint16, err error) {
	var list unsafe.Pointer
	n, _, _ := procLibusbGetDeviceList.Call(0, uintptr(unsafe.Pointer(&list)))
	count := int(int64(n))
	if count < 0 {
		return 0, 0, 0, contract.BadGateway("usb_enum_failed", fmt.Sprintf("libusb_get_device_list failed (code %d)", count), "")
	}
	defer procLibusbFreeDeviceList.Call(uintptr(list), 1)

	devices := unsafe.Slice((*uintptr)(list), count)
	targetSpecified := vid != nil && pid != nil

	for _, dev := range devices {
		var desc libusbDeviceDescriptor
		r1, _, _ := procLibusbGetDeviceDescriptor.Call(dev, uintptr(unsafe.Pointer(&desc)))
		if int32(r1) != 0 {
			continue
		}

		if targetSpecified {
			if desc.IdVendor != *vid || desc.IdProduct != *pid {
				continue
			}
		} else if !isPrinterClassDevice(dev, &desc) {
			continue
		}

		var h uintptr
		r1, _, _ = procLibusbOpen.Call(dev, uintptr(unsafe.Pointer(&h)))
		if int32(r1) != 0 || h == 0 {
			continue // try the next candidate rather than failing outright
		}
		return h, desc.IdVendor, desc.IdProduct, nil
	}

	if targetSpecified {
		return 0, 0, 0, contract.NotFound("usb_device_not_found", fmt.Sprintf("USB device VID:%04x PID:%04x not found", *vid, *pid))
	}
	return 0, 0, 0, contract.NotFound("usb_device_not_found", "no USB printer-class device found")
}

// isPrinterClassDevice reports whether dev is USB Printer Class (0x07),
// checking the device descriptor first and, if that doesn't declare a
// class (common: many printers declare the class per-interface instead),
// walking the active configuration's interfaces — same two-level check as
// the Python service's `device.bDeviceClass == USB_CLASS_PRINTER or
// any(intf.bInterfaceClass == USB_CLASS_PRINTER for cfg in device for intf
// in cfg)`.
func isPrinterClassDevice(dev uintptr, desc *libusbDeviceDescriptor) bool {
	if desc.BDeviceClass == usbClassPrinter {
		return true
	}

	var cfgPtr unsafe.Pointer
	r1, _, _ := procLibusbGetActiveConfigDescriptor.Call(dev, uintptr(unsafe.Pointer(&cfgPtr)))
	if int32(r1) != 0 || cfgPtr == nil {
		return false
	}
	defer procLibusbFreeConfigDescriptor.Call(uintptr(cfgPtr))

	cfg := (*libusbConfigDescriptor)(cfgPtr)
	if cfg.Interface == nil || cfg.BNumInterfaces == 0 {
		return false
	}

	interfaces := unsafe.Slice((*libusbInterface)(cfg.Interface), cfg.BNumInterfaces)
	for _, iface := range interfaces {
		if iface.Altsetting == nil || iface.NumAltsetting == 0 {
			continue
		}
		altsettings := unsafe.Slice((*libusbInterfaceDescriptor)(iface.Altsetting), iface.NumAltsetting)
		for _, alt := range altsettings {
			if alt.BInterfaceClass == usbClassPrinter {
				return true
			}
		}
	}
	return false
}

func (a *USBAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil
	}
	procLibusbReleaseInterface.Call(a.handle, uintptr(a.interfaceNum))
	procLibusbClose.Call(a.handle)
	procLibusbExit.Call(0)
	a.handle = 0
	a.isOpen = false
	return nil
}

func (a *USBAdapter) Write(data []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return 0, contract.BadGateway("usb_not_open", "USB device is not open", "")
	}
	if len(data) == 0 {
		return 0, nil
	}
	var transferred int32
	r1, _, _ := procLibusbBulkTransfer.Call(
		a.handle,
		uintptr(a.outEP),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(int32(len(data))),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(5000),
	)
	if int32(r1) != 0 {
		return int(transferred), contract.BadGateway("usb_write_failed", fmt.Sprintf("libusb_bulk_transfer (write) failed (code %d)", int32(r1)), "")
	}
	return int(transferred), nil
}

// Read is best-effort: many USB printers never answer bulk IN reads unless
// they have data queued (status bytes, etc.), so a timeout here is a normal
// outcome, not necessarily a fatal error for the caller.
func (a *USBAdapter) Read(size int) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isOpen {
		return nil, contract.BadGateway("usb_not_open", "USB device is not open", "")
	}
	if size <= 0 {
		size = 1024
	}
	buf := make([]byte, size)
	var transferred int32
	r1, _, _ := procLibusbBulkTransfer.Call(
		a.handle,
		uintptr(a.inEP),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(int32(size)),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(1000),
	)
	if int32(r1) != 0 {
		return nil, contract.BadGateway("usb_read_failed", fmt.Sprintf("libusb_bulk_transfer (read) failed (code %d)", int32(r1)), "")
	}
	return buf[:transferred], nil
}

func (a *USBAdapter) IsOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isOpen
}

// USBPrinterInfo mirrors one entry of the Python service's
// USBAdapter.find_printers() dicts, used by internal/printsvc/scan.go to
// populate GET /api/v1/devices/scan.
type USBPrinterInfo struct {
	VID          string // 4 lowercase hex digits, e.g. "04b8"
	PID          string
	Manufacturer string
	Product      string
	Serial       string
}

// FindUSBPrinters enumerates all USB printer-class (0x07) devices currently
// attached, best-effort reading their manufacturer/product/serial string
// descriptors (a device that refuses those reads — permissions, claimed by
// another driver — is still listed, just with those fields blank, same as
// Python's try/except around usb.util.get_string()).
func FindUSBPrinters() ([]USBPrinterInfo, error) {
	if err := loadLibusb(); err != nil {
		return nil, err
	}

	r1, _, _ := procLibusbInit.Call(0)
	if int32(r1) != 0 {
		return nil, fmt.Errorf("hwadapter: libusb_init failed (code %d)", int32(r1))
	}
	defer procLibusbExit.Call(0)

	var list unsafe.Pointer
	n, _, _ := procLibusbGetDeviceList.Call(0, uintptr(unsafe.Pointer(&list)))
	count := int(int64(n))
	if count < 0 {
		return nil, fmt.Errorf("hwadapter: libusb_get_device_list failed (code %d)", count)
	}
	defer procLibusbFreeDeviceList.Call(uintptr(list), 1)

	devices := unsafe.Slice((*uintptr)(list), count)
	var printers []USBPrinterInfo

	for _, dev := range devices {
		var desc libusbDeviceDescriptor
		r1, _, _ := procLibusbGetDeviceDescriptor.Call(dev, uintptr(unsafe.Pointer(&desc)))
		if int32(r1) != 0 {
			continue
		}
		if !isPrinterClassDevice(dev, &desc) {
			continue
		}

		info := USBPrinterInfo{
			VID: fmt.Sprintf("%04x", desc.IdVendor),
			PID: fmt.Sprintf("%04x", desc.IdProduct),
		}

		var handle uintptr
		r1, _, _ = procLibusbOpen.Call(dev, uintptr(unsafe.Pointer(&handle)))
		if int32(r1) == 0 && handle != 0 {
			if desc.IManufacturer != 0 {
				info.Manufacturer = getUSBStringDescriptorASCII(handle, desc.IManufacturer)
			}
			if desc.IProduct != 0 {
				info.Product = getUSBStringDescriptorASCII(handle, desc.IProduct)
			}
			if desc.ISerialNumber != 0 {
				info.Serial = getUSBStringDescriptorASCII(handle, desc.ISerialNumber)
			}
			procLibusbClose.Call(handle)
		}

		printers = append(printers, info)
	}

	return printers, nil
}

func getUSBStringDescriptorASCII(handle uintptr, index uint8) string {
	buf := make([]byte, 256)
	r1, _, _ := procLibusbGetStringDescriptorAscii.Call(
		handle,
		uintptr(index),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	n := int32(r1)
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}
