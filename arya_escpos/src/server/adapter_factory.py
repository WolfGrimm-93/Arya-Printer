"""Shared adapter creation logic for all device types."""
from adapters.usb_adapter import USBAdapter
from adapters.network_adapter import NetworkAdapter
from adapters.serial_adapter import SerialAdapter
from adapters.bluetooth_adapter import BluetoothAdapter, PYBLUEZ_AVAILABLE
from server.compat import WINDOWS_PRINTER_AVAILABLE, win32print
from utils.logger import get_logger

logger = get_logger()


def create_adapter(device_type: str, **kwargs):
    """Create and open the appropriate adapter for a device type.

    Args:
        device_type: One of "usb", "network", "serial", "bluetooth"
        **kwargs: Connection parameters passed to adapter.open()

    Returns:
        An opened adapter instance

    Raises:
        ValueError: If device type is unsupported or missing params
        RuntimeError: If connection fails
    """
    if device_type == "usb":
        adapter = USBAdapter()
        if not adapter.open(vid=kwargs.get("vid"), pid=kwargs.get("pid")):
            raise RuntimeError("Failed to open USB device")
        return adapter

    if device_type == "network":
        adapter = NetworkAdapter()
        if not adapter.open(host=kwargs.get("host"), port=kwargs.get("port", 9100)):
            raise RuntimeError(f"Failed to connect to {kwargs.get('host')}:{kwargs.get('port', 9100)}")
        return adapter

    if device_type == "serial":
        adapter = SerialAdapter()
        if not adapter.open(port=kwargs.get("port")):
            raise RuntimeError(f"Failed to open serial port: {kwargs.get('port')}")
        return adapter

    if device_type == "bluetooth":
        if not PYBLUEZ_AVAILABLE:
            raise RuntimeError("Bluetooth support not available")
        adapter = BluetoothAdapter()
        if not adapter.open(address=kwargs.get("address")):
            raise RuntimeError("Failed to connect to Bluetooth device")
        return adapter

    raise ValueError(f"Unsupported device type: {device_type}")


def send_raw_to_windows(printer_name: str, data: bytes, job_name: str = "Arya ESCPOS Print Job"):
    """Send raw bytes to a Windows printer via the Spooler."""
    if not WINDOWS_PRINTER_AVAILABLE:
        raise RuntimeError("Windows printer support not available")

    printer_handle = win32print.OpenPrinter(printer_name)
    try:
        win32print.StartDocPrinter(printer_handle, 1, (job_name, None, "RAW"))
        win32print.StartPagePrinter(printer_handle)
        win32print.WritePrinter(printer_handle, data)
        win32print.EndPagePrinter(printer_handle)
        win32print.EndDocPrinter(printer_handle)
        logger.info(f"Printed to Windows printer: {printer_name}")
    finally:
        win32print.ClosePrinter(printer_handle)
