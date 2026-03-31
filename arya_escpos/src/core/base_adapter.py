"""
Base adapter class for all device types
Inspired by node-escpos adapter pattern
"""
from abc import ABC, abstractmethod
from typing import Optional, Callable
from utils.exceptions import AdapterError


class BaseAdapter(ABC):
    """
    Abstract base adapter for device communication
    All device adapters (USB, Network, Serial, Bluetooth) inherit from this
    """
    
    def __init__(self):
        """Initialize adapter"""
        self.device = None
        self.is_connected = False
        self._on_connect_callback: Optional[Callable] = None
        self._on_disconnect_callback: Optional[Callable] = None
        self._on_error_callback: Optional[Callable] = None
    
    @abstractmethod
    def open(self, **kwargs) -> bool:
        """
        Open connection to device
        
        Returns:
            True if successful, False otherwise
            
        Raises:
            AdapterError: If connection fails
        """
        pass
    
    @abstractmethod
    def close(self) -> bool:
        """
        Close connection to device
        
        Returns:
            True if successful, False otherwise
        """
        pass
    
    @abstractmethod
    def write(self, data: bytes) -> int:
        """
        Write data to device
        
        Args:
            data: Raw bytes to write
            
        Returns:
            Number of bytes written
            
        Raises:
            AdapterError: If write fails
        """
        pass
    
    @abstractmethod
    def read(self, size: int = 1024) -> bytes:
        """
        Read data from device
        
        Args:
            size: Maximum number of bytes to read
            
        Returns:
            Bytes read from device
            
        Raises:
            AdapterError: If read fails
        """
        pass
    
    def is_open(self) -> bool:
        """Check if connection is open"""
        return self.is_connected and self.device is not None
    
    def on_connect(self, callback: Callable):
        """Register callback for connection event"""
        self._on_connect_callback = callback
        return self
    
    def on_disconnect(self, callback: Callable):
        """Register callback for disconnection event"""
        self._on_disconnect_callback = callback
        return self
    
    def on_error(self, callback: Callable):
        """Register callback for error event"""
        self._on_error_callback = callback
        return self
    
    def _trigger_connect(self):
        """Trigger connect callback"""
        if self._on_connect_callback:
            self._on_connect_callback(self)
    
    def _trigger_disconnect(self):
        """Trigger disconnect callback"""
        if self._on_disconnect_callback:
            self._on_disconnect_callback(self)
    
    def _trigger_error(self, error: Exception):
        """Trigger error callback"""
        if self._on_error_callback:
            self._on_error_callback(error)
    
    def __enter__(self):
        """Context manager entry"""
        self.open()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit"""
        self.close()
    
    def __repr__(self):
        return f"<{self.__class__.__name__}(connected={self.is_connected})>"
