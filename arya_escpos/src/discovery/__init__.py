"""
Discovery module placeholder
USB, Network, and Serial device discovery
"""
from ..adapters import USBAdapter, NetworkAdapter, SerialAdapter
from ..utils import get_logger

__all__ = [
    "USBAdapter",
    "NetworkAdapter",
    "SerialAdapter",
]
