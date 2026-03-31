"""
Serial Adapter - Based on python-escpos serial implementation
"""
import serial
from typing import Optional

from core.base_adapter import BaseAdapter
from utils.exceptions import SerialError
from utils.logger import get_logger


class SerialAdapter(BaseAdapter):
    """
    Serial Port Adapter
    Supports COM/tty serial printers
    """
    
    def __init__(
        self,
        port: Optional[str] = None,
        baudrate: int = 9600,
        bytesize: int = 8,
        parity: str = "N",
        stopbits: int = 1,
        timeout: int = 1,
        xonxoff: bool = False,
        rtscts: bool = False,
        dsrdtr: bool = True,
    ):
        super().__init__()
        self.logger = get_logger()
        self.port = port
        self.baudrate = baudrate
        self.bytesize = bytesize
        self.parity = parity
        self.stopbits = stopbits
        self.timeout = timeout
        self.xonxoff = xonxoff
        self.rtscts = rtscts
        self.dsrdtr = dsrdtr
        self.serial_port: Optional[serial.Serial] = None

    def open(self, **kwargs) -> bool:
        """Open serial port connection."""
        self.port = kwargs.get("port", self.port)

        if not self.port:
            raise SerialError("Serial port is required")

        try:
            self.serial_port = serial.Serial(
                port=self.port,
                baudrate=self.baudrate,
                bytesize=self.bytesize,
                parity=self.parity,
                stopbits=self.stopbits,
                timeout=self.timeout,
                xonxoff=self.xonxoff,
                rtscts=self.rtscts,
                dsrdtr=self.dsrdtr,
            )
            
            self.device = self.serial_port
            self.is_connected = True
            self._trigger_connect()
            self.logger.info(f"Serial port opened: {self.port} @ {self.baudrate} baud")
            return True
            
        except serial.SerialException as e:
            raise SerialError(f"Serial port error: {e}")
        except Exception as e:
            raise SerialError(f"Unexpected error: {e}")
    
    def close(self) -> bool:
        """Close serial port"""
        if self.serial_port and self.serial_port.is_open:
            try:
                self.serial_port.close()
                self.serial_port = None
                self.device = None
                self.is_connected = False
                self._trigger_disconnect()
                self.logger.info(f"Serial port closed: {self.port}")
                return True
            except Exception as e:
                self.logger.error(f"Error closing serial port: {e}")
                return False
        return True
    
    def write(self, data: bytes) -> int:
        """
        Write data to serial port
        
        Args:
            data: Bytes to write
            
        Returns:
            Number of bytes written
            
        Raises:
            SerialError: If write fails
        """
        if not self.is_connected or not self.serial_port:
            raise SerialError("Serial port not connected")
        
        try:
            bytes_written = self.serial_port.write(data)
            self.serial_port.flush()  # Ensure data is sent
            return bytes_written
        except serial.SerialException as e:
            raise SerialError(f"Write error: {e}")
    
    def read(self, size: int = 1024) -> bytes:
        """
        Read data from serial port
        
        Args:
            size: Maximum bytes to read
            
        Returns:
            Bytes read
            
        Raises:
            SerialError: If read fails
        """
        if not self.is_connected or not self.serial_port:
            raise SerialError("Serial port not connected")
        
        try:
            data = self.serial_port.read(size)
            return data
        except serial.SerialException as e:
            raise SerialError(f"Read error: {e}")
    
    @staticmethod
    def list_ports() -> list:
        """
        List available serial ports
        
        Returns:
            List of available port names
        """
        from serial.tools import list_ports
        return [port.device for port in list_ports.comports()]
