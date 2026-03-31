"""Platform-specific availability flags."""

WINDOWS_PRINTER_AVAILABLE = False
try:
    import win32print
    import win32api
    import win32con
    import pywintypes
    WINDOWS_PRINTER_AVAILABLE = True
except ImportError:
    win32print = None
    win32api = None
    win32con = None
    pywintypes = None
