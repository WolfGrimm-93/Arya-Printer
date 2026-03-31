"""
Custom exceptions for Arya ESCPOS
"""


class AryaEscposError(Exception):
    """Base exception for all Arya ESCPOS errors"""
    pass


class DeviceError(AryaEscposError):
    """Base exception for device-related errors"""
    pass


class DeviceNotFoundError(DeviceError):
    """Raised when a device is not found"""
    pass


class DeviceConnectionError(DeviceError):
    """Raised when unable to connect to a device"""
    pass




class DeviceDisconnectedError(DeviceError):
    """Raised when device is unexpectedly disconnected"""
    pass


class AdapterError(AryaEscposError):
    """Base exception for adapter-related errors"""
    pass


class USBError(AdapterError):
    """USB-specific errors"""
    pass


class NetworkError(AdapterError):
    """Network-specific errors"""
    pass


class SerialError(AdapterError):
    """Serial port-specific errors"""
    pass


class BluetoothError(AdapterError):
    """Bluetooth-specific errors"""
    pass


class PrintError(AryaEscposError):
    """Printing-related errors"""
    pass


class CommandError(AryaEscposError):
    """ESC/POS command-related errors"""
    pass


class ConfigurationError(AryaEscposError):
    """Configuration-related errors"""
    pass
