"""
Windows Printer Adapter usando win32print
Permite usar impresoras instaladas con drivers nativos de Windows
"""

from typing import List, Dict, Optional
import win32print
import win32ui
from src.utils.logger import get_logger

class WindowsPrinterAdapter:
    """Adapter for Windows printers using win32print"""
    
    def __init__(self):
        self.printer_handle = None
        self.printer_name = None
        self.logger = get_logger()
    
    @staticmethod
    def find_printers() -> List[Dict]:
        """
        Find all printers installed in Windows
        
        Returns:
            List of printer device info dicts
        """
        logger = get_logger()
        printers = []
        
        try:
            # Get all printers from Windows
            printer_enum = win32print.EnumPrinters(
                win32print.PRINTER_ENUM_LOCAL | win32print.PRINTER_ENUM_CONNECTIONS
            )
            
            for flags, description, name, comment in printer_enum:
                printer_info = {
                    "name": name,
                    "description": description,
                    "comment": comment,
                    "type": "windows",
                    "connection_type": "local" if flags & win32print.PRINTER_ENUM_LOCAL else "network"
                }
                printers.append(printer_info)
                logger.debug(f"Found Windows printer: {name}")
                
        except Exception as e:
            logger.error(f"Error finding Windows printers: {e}")
        
        return printers
    
    def open(self, printer_name: str) -> bool:
        """
        Open connection to Windows printer
        
        Args:
            printer_name: Name of the printer as shown in Windows
            
        Returns:
            True if successful
        """
        try:
            self.printer_name = printer_name
            self.printer_handle = win32print.OpenPrinter(printer_name)
            self.logger.info(f"Opened Windows printer: {printer_name}")
            return True
        except Exception as e:
            self.logger.error(f"Error opening printer {printer_name}: {e}")
            return False
    
    def write(self, data: bytes) -> bool:
        """
        Write raw data to printer
        
        Args:
            data: ESC/POS commands as bytes
            
        Returns:
            True if successful
        """
        if not self.printer_handle:
            self.logger.error("Printer not opened")
            return False
        
        try:
            # Start a document
            job_info = ("Python RAW Print Job", None, "RAW")
            job_id = win32print.StartDocPrinter(self.printer_handle, 1, job_info)
            
            # Start a page
            win32print.StartPagePrinter(self.printer_handle)
            
            # Write raw data
            win32print.WritePrinter(self.printer_handle, data)
            
            # End page and document
            win32print.EndPagePrinter(self.printer_handle)
            win32print.EndDocPrinter(self.printer_handle)
            
            self.logger.debug(f"Wrote {len(data)} bytes to printer")
            return True
            
        except Exception as e:
            self.logger.error(f"Error writing to printer: {e}")
            return False
    
    def close(self):
        """Close printer connection"""
        if self.printer_handle:
            try:
                win32print.ClosePrinter(self.printer_handle)
                self.logger.info(f"Closed printer: {self.printer_name}")
            except Exception as e:
                self.logger.error(f"Error closing printer: {e}")
            finally:
                self.printer_handle = None
                self.printer_name = None
    
    def __enter__(self):
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
