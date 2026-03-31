"""
Network Adapter - Based on node-escpos network implementation
"""
import socket
from typing import Optional

from core.base_adapter import BaseAdapter
from utils.exceptions import NetworkError
from utils.logger import get_logger


class NetworkAdapter(BaseAdapter):
    """
    Network Adapter for TCP/IP printers
    Default port 9100 (common for network printers)
    """
    
    def __init__(self, host: Optional[str] = None, port: int = 9100, timeout: int = 5):
        super().__init__()
        self.logger = get_logger()
        self.host = host
        self.port = port
        self.timeout = timeout
        self.socket: Optional[socket.socket] = None

    def open(self, **kwargs) -> bool:
        """Open TCP connection to printer."""
        self.host = kwargs.get("host", self.host)
        self.port = kwargs.get("port", self.port)
        self.timeout = kwargs.get("timeout", self.timeout)

        if not self.host:
            raise NetworkError("Host address is required")

        try:
            self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.socket.settimeout(self.timeout)
            self.socket.connect((self.host, self.port))
            
            self.device = self.socket
            self.is_connected = True
            self._trigger_connect()
            self.logger.info(f"Network connection established to {self.host}:{self.port}")
            return True
            
        except socket.timeout:
            raise NetworkError(f"Connection timeout to {self.host}:{self.port}")
        except socket.error as e:
            raise NetworkError(f"Socket error: {e}")
        except Exception as e:
            raise NetworkError(f"Unexpected error: {e}")
    
    def close(self) -> bool:
        """Close network connection"""
        if self.socket:
            try:
                self.socket.close()
                self.socket = None
                self.device = None
                self.is_connected = False
                self._trigger_disconnect()
                self.logger.info(f"Network connection closed to {self.host}:{self.port}")
                return True
            except Exception as e:
                self.logger.error(f"Error closing socket: {e}")
                return False
        return True
    
    def write(self, data: bytes) -> int:
        """
        Write data to network printer
        
        Args:
            data: Bytes to send
            
        Returns:
            Number of bytes sent
            
        Raises:
            NetworkError: If send fails
        """
        if not self.is_connected or not self.socket:
            raise NetworkError("Not connected")
        
        try:
            return self.socket.send(data)
        except socket.error as e:
            raise NetworkError(f"Send error: {e}")
    
    def read(self, size: int = 1024) -> bytes:
        """
        Read data from network printer
        
        Args:
            size: Maximum bytes to read
            
        Returns:
            Bytes received
            
        Raises:
            NetworkError: If receive fails
        """
        if not self.is_connected or not self.socket:
            raise NetworkError("Not connected")
        
        try:
            data = self.socket.recv(size)
            return data
        except socket.error as e:
            raise NetworkError(f"Receive error: {e}")
    
    @staticmethod
    def test_connection(host: str, port: int = 9100, timeout: int = 2) -> bool:
        """
        Test if a network printer is reachable
        
        Args:
            host: IP address or hostname
            port: TCP port
            timeout: Connection timeout
            
        Returns:
            True if reachable, False otherwise
        """
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(timeout)
            result = sock.connect_ex((host, port))
            sock.close()
            return result == 0
        except OSError:
            return False
