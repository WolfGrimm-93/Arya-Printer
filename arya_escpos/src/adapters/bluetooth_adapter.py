"""
Bluetooth Adapter - PyBluez (Bluetooth Classic/RFCOMM)

STATUS: INCOMPLETE - PyBluez is not compatible with Python 3.12+ on Windows.

Alternatives:
  - bleak: BLE only, async, cross-platform (pip install bleak)
  - pybluez2: maintained fork, may fail on Windows (pip install pybluez2)
  - bleak-serial: serial over BLE for modern printers
  - Windows Bluetooth API: native via ctypes/win32, Windows only

Works with traditional POS printers that use Bluetooth Classic (RFCOMM).
"""

from typing import Optional, List, Dict, Any
import threading
from core.base_adapter import BaseAdapter
from utils.logger import get_logger
from utils.exceptions import DeviceConnectionError, AdapterError

logger = get_logger()

try:
    import bluetooth
    PYBLUEZ_AVAILABLE = True
except ImportError:
    PYBLUEZ_AVAILABLE = False
    logger.warning("PyBluez is not installed. Bluetooth adapter will not be available.")


class BluetoothAdapter(BaseAdapter):
    """
    Bluetooth Classic Adapter using PyBluez (RFCOMM).

    - Scans nearby Bluetooth devices
    - Finds the Serial Port Profile (SPP/RFCOMM) channel
    - Connects via RFCOMM socket

    Args:
        address: Bluetooth MAC address (e.g. "00:11:22:33:44:55")
        channel: RFCOMM channel (1-30, auto-detected if None)
        timeout: Timeout for operations in seconds
    """
    
    def __init__(self, address: Optional[str] = None, channel: Optional[int] = None, timeout: int = 10):
        super().__init__()

        if not PYBLUEZ_AVAILABLE:
            raise AdapterError("PyBluez is not installed. Install with: pip install pybluez")

        self.address = address
        self.channel = channel
        self.timeout = timeout
        self.socket: Optional[bluetooth.BluetoothSocket] = None
        self._lock = threading.Lock()

    def open(self, **kwargs) -> bool:
        """
        Open Bluetooth RFCOMM connection.

        Args:
            address: Bluetooth MAC address (overrides constructor value)
            channel: RFCOMM channel (auto-detected if None)

        Returns:
            True if successful

        Raises:
            DeviceConnectionError: If connection fails
        """
        self.address = kwargs.get("address", self.address)
        self.channel = kwargs.get("channel", self.channel)

        if self.is_connected:
            logger.warning(f"Bluetooth already connected to {self.address}")
            return True

        try:
            if self.channel is None:
                logger.info(f"Searching RFCOMM channel for {self.address}...")
                self.channel = self._find_rfcomm_channel(self.address)
                if self.channel is None:
                    raise DeviceConnectionError(f"No RFCOMM service found on {self.address}")
                logger.info(f"RFCOMM channel detected: {self.channel}")

            self.socket = bluetooth.BluetoothSocket(bluetooth.RFCOMM)

            logger.info(f"Connecting to {self.address}:{self.channel}...")
            self.socket.connect((self.address, self.channel))
            self.socket.settimeout(self.timeout)

            self.device = self.socket
            self.is_connected = True
            self._trigger_connect()
            logger.info(f"Bluetooth connected: {self.address}:{self.channel}")
            return True

        except bluetooth.BluetoothError as e:
            self.is_connected = False
            error_msg = f"Bluetooth connection error: {e}"
            logger.error(error_msg)
            self._trigger_error(DeviceConnectionError(error_msg))
            raise DeviceConnectionError(error_msg)
        except Exception as e:
            self.is_connected = False
            error_msg = f"Unexpected error: {e}"
            logger.error(error_msg)
            self._trigger_error(DeviceConnectionError(error_msg))
            raise DeviceConnectionError(error_msg)

    def close(self) -> bool:
        """Close Bluetooth connection.

        Returns:
            True if successful
        """
        if not self.is_connected:
            return True

        try:
            if self.socket:
                self.socket.close()
                self.socket = None

            self.device = None
            self.is_connected = False
            self._trigger_disconnect()
            logger.info(f"Bluetooth disconnected: {self.address}")
            return True

        except Exception as e:
            logger.error(f"Error closing Bluetooth: {e}")
            return False

    def write(self, data: bytes) -> int:
        """
        Write data to Bluetooth device.

        Args:
            data: Bytes to send

        Returns:
            Number of bytes written

        Raises:
            DeviceConnectionError: If not connected or write fails
        """
        if not self.is_connected or not self.socket:
            raise DeviceConnectionError("Bluetooth is not connected")

        try:
            with self._lock:
                bytes_sent = self.socket.send(data)
                logger.debug(f"Bluetooth TX: {bytes_sent} bytes")
                return bytes_sent

        except bluetooth.BluetoothError as e:
            error_msg = f"Write error: {e}"
            logger.error(error_msg)
            self._trigger_error(DeviceConnectionError(error_msg))
            raise DeviceConnectionError(error_msg)

    def read(self, size: int = 1024) -> bytes:
        """
        Read data from Bluetooth device.

        Args:
            size: Maximum bytes to read

        Returns:
            Bytes read

        Raises:
            DeviceConnectionError: If not connected or read fails
        """
        if not self.is_connected or not self.socket:
            raise DeviceConnectionError("Bluetooth is not connected")

        try:
            with self._lock:
                data = self.socket.recv(size)
                logger.debug(f"Bluetooth RX: {len(data)} bytes")
                return data

        except bluetooth.BluetoothError as e:
            error_msg = f"Read error: {e}"
            logger.error(error_msg)
            self._trigger_error(DeviceConnectionError(error_msg))
            raise DeviceConnectionError(error_msg)
    
    @staticmethod
    def _find_rfcomm_channel(address: str) -> Optional[int]:
        """Find the RFCOMM channel for a device via SDP lookup.

        Args:
            address: Bluetooth MAC address

        Returns:
            RFCOMM channel number or None
        """
        try:
            services = bluetooth.find_service(address=address)

            for service in services:
                if service.get("protocol") == "RFCOMM":
                    return service["port"]
                if "port" in service:
                    return service["port"]

            logger.warning(f"No SDP record found, falling back to channel 1")
            return 1

        except bluetooth.BluetoothError as e:
            logger.error(f"Error finding RFCOMM channel: {e}")
            return None

    @staticmethod
    def find_printers(duration: int = 8) -> List[Dict[str, Any]]:
        """Scan for nearby Bluetooth printers.

        Args:
            duration: Scan duration in seconds

        Returns:
            List of printer info dicts with address, name, channel, type
        """
        if not PYBLUEZ_AVAILABLE:
            logger.error("PyBluez is not installed")
            return []

        printers = []

        try:
            logger.info(f"Scanning Bluetooth devices ({duration}s)...")
            devices = bluetooth.discover_devices(
                duration=duration, lookup_names=True, flush_cache=True
            )
            logger.info(f"Found {len(devices)} Bluetooth devices")

            for address, name in devices:
                logger.debug(f"Checking {name} ({address})...")
                channel = BluetoothAdapter._find_rfcomm_channel(address)

                if channel is not None:
                    printers.append({
                        "address": address,
                        "name": name or "Unknown",
                        "channel": channel,
                        "type": "bluetooth",
                    })
                    logger.info(f"Bluetooth printer found: {name} ({address}:{channel})")

            logger.info(f"Total Bluetooth printers: {len(printers)}")
            return printers

        except bluetooth.BluetoothError as e:
            logger.error(f"Bluetooth scan error: {e}")
            return []
        except Exception as e:
            logger.error(f"Unexpected scan error: {e}")
            return []

    @staticmethod
    def get_device(address: str, channel: Optional[int] = None, timeout: int = 10) -> "BluetoothAdapter":
        """Create and connect a BluetoothAdapter.

        Args:
            address: Bluetooth MAC address
            channel: RFCOMM channel (auto-detected if None)
            timeout: Timeout in seconds

        Returns:
            Connected BluetoothAdapter instance
        """
        adapter = BluetoothAdapter(address, channel, timeout)
        adapter.open()
        return adapter
    
    def __repr__(self) -> str:
        status = "connected" if self.is_connected else "disconnected"
        return f"<BluetoothAdapter {self.address}:{self.channel} ({status})>"
