"""
USB Adapter - Based on node-escpos and python-escpos USB implementations
"""
from typing import Optional, List, Dict
import platform

try:
    import usb.core
    import usb.util
    PYUSB_AVAILABLE = True
except ImportError:
    PYUSB_AVAILABLE = False

from core.base_adapter import BaseAdapter
from utils.exceptions import USBError
from utils.logger import get_logger


# USB Printer Class
USB_CLASS_PRINTER = 0x07


class USBAdapter(BaseAdapter):
    """
    USB Device Adapter
    Supports USB printers using PyUSB
    """
    
    def __init__(
        self,
        vid: Optional[int] = None,
        pid: Optional[int] = None,
        interface: int = 0,
        in_ep: int = 0x82,
        out_ep: int = 0x01,
    ):
        """
        Initialize USB adapter
        
        Args:
            vid: Vendor ID (None for auto-detect)
            pid: Product ID (None for auto-detect)
            interface: USB interface number
            in_ep: Input endpoint address
            out_ep: Output endpoint address
        """
        super().__init__()
        self.logger = get_logger()
        self.vid = vid
        self.pid = pid
        self.interface = interface
        self.in_ep = in_ep
        self.out_ep = out_ep
        self.device = None
        
    @staticmethod
    def find_printers() -> List[Dict]:
        """
        Find all USB printer devices
        
        Returns:
            List of printer device info dicts
        """
        logger = get_logger()
        printers = []
        
        try:
            devices = usb.core.find(find_all=True)
            
            for device in devices:
                try:
                    if device.bDeviceClass == USB_CLASS_PRINTER or \
                       any(intf.bInterfaceClass == USB_CLASS_PRINTER 
                           for cfg in device 
                           for intf in cfg):
                        
                        # Try to read device strings, but don't fail if Windows blocks access
                        manufacturer = None
                        product = None
                        serial = None
                        
                        try:
                            if device.iManufacturer:
                                manufacturer = usb.util.get_string(device, device.iManufacturer)
                        except (usb.core.USBError, ValueError):
                            pass
                            
                        try:
                            if device.iProduct:
                                product = usb.util.get_string(device, device.iProduct)
                        except (usb.core.USBError, ValueError):
                            pass
                            
                        try:
                            if device.iSerialNumber:
                                serial = usb.util.get_string(device, device.iSerialNumber)
                        except (usb.core.USBError, ValueError):
                            pass
                        
                        printer_info = {
                            "vid": f"{device.idVendor:04x}",
                            "pid": f"{device.idProduct:04x}",
                            "manufacturer": manufacturer,
                            "product": product,
                            "serial": serial,
                            "bus": device.bus,
                            "address": device.address,
                        }
                        printers.append(printer_info)
                        
                except (usb.core.USBError, ValueError) as e:
                    logger.debug(f"Error reading USB device: {e}")
                    continue
                    
        except Exception as e:
            logger.error(f"Error finding USB printers: {e}")
        
        return printers
    
    def open(self, **kwargs) -> bool:
        """
        Open connection to USB device
        
        Returns:
            True if successful
            
        Raises:
            USBError: If connection fails
        """
        try:
            # Find device
            if self.vid and self.pid:
                self.device = usb.core.find(idVendor=self.vid, idProduct=self.pid)
            else:
                # Auto-detect first printer
                printers = self.find_printers()
                if not printers:
                    raise USBError("No USB printers found")
                
                first_printer = printers[0]
                self.vid = int(first_printer["vid"], 16)
                self.pid = int(first_printer["pid"], 16)
                self.device = usb.core.find(idVendor=self.vid, idProduct=self.pid)
            
            if self.device is None:
                raise USBError(f"USB device not found (VID:{self.vid:04x}, PID:{self.pid:04x})")
            
            # Detach kernel driver if active (Linux)
            if platform.system() != "Windows":
                if self.device.is_kernel_driver_active(self.interface):
                    try:
                        self.device.detach_kernel_driver(self.interface)
                        self.logger.debug("Detached kernel driver")
                    except usb.core.USBError as e:
                        self.logger.warning(f"Could not detach kernel driver: {e}")
            
            # Set configuration
            try:
                self.device.set_configuration()
            except usb.core.USBError as e:
                self.logger.debug(f"Device already configured: {e}")
            
            # Claim interface
            usb.util.claim_interface(self.device, self.interface)
            
            self.is_connected = True
            self._trigger_connect()
            self.logger.info(f"USB device opened (VID:{self.vid:04x}, PID:{self.pid:04x})")
            return True
            
        except usb.core.USBError as e:
            raise USBError(f"USB connection error: {e}")
        except Exception as e:
            raise USBError(f"Unexpected error opening USB device: {e}")
    
    def close(self) -> bool:
        """Close USB connection"""
        if self.device:
            try:
                usb.util.release_interface(self.device, self.interface)
                usb.util.dispose_resources(self.device)
                self.device = None
                self.is_connected = False
                self._trigger_disconnect()
                self.logger.info("USB device closed")
                return True
            except Exception as e:
                self.logger.error(f"Error closing USB device: {e}")
                return False
        return True
    
    def write(self, data: bytes) -> int:
        """
        Write data to USB device
        
        Args:
            data: Bytes to write
            
        Returns:
            Number of bytes written
            
        Raises:
            USBError: If write fails
        """
        if not self.is_connected or not self.device:
            raise USBError("Device not connected")
        
        try:
            bytes_written = self.device.write(self.out_ep, data, self.interface)
            return bytes_written
        except usb.core.USBError as e:
            raise USBError(f"USB write error: {e}")
    
    def read(self, size: int = 1024) -> bytes:
        """
        Read data from USB device
        
        Args:
            size: Maximum bytes to read
            
        Returns:
            Bytes read
            
        Raises:
            USBError: If read fails
        """
        if not self.is_connected or not self.device:
            raise USBError("Device not connected")
        
        try:
            data = self.device.read(self.in_ep, size, timeout=1000)
            return bytes(data)
        except usb.core.USBError as e:
            raise USBError(f"USB read error: {e}")
