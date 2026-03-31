"""
ESC/POS Command Builder
Based on node-escpos commands and python-escpos constants
"""
import base64
import io
from typing import Literal
from enum import Enum

from PIL import Image


class ESC:
    """ESC/POS Control Characters"""
    LF = b'\x0a'      # Line Feed
    FF = b'\x0c'      # Form Feed
    CR = b'\x0d'      # Carriage Return
    HT = b'\x09'      # Horizontal Tab
    VT = b'\x0b'      # Vertical Tab
    ESC = b'\x1b'     # Escape
    FS = b'\x1c'      # Field Separator
    GS = b'\x1d'      # Group Separator
    DLE = b'\x10'     # Data Link Escape
    EOT = b'\x04'     # End of Transmission


class TextAlign(Enum):
    """Text alignment options"""
    LEFT = b'\x1b\x61\x00'
    CENTER = b'\x1b\x61\x01'
    RIGHT = b'\x1b\x61\x02'


class TextFont(Enum):
    """Font selection"""
    A = b'\x1b\x4d\x00'
    B = b'\x1b\x4d\x01'
    C = b'\x1b\x4d\x02'


class TextStyle(Enum):
    """Text style modifiers"""
    NORMAL = b'\x1b\x21\x00'
    BOLD_ON = b'\x1b\x45\x01'
    BOLD_OFF = b'\x1b\x45\x00'
    UNDERLINE_ON = b'\x1b\x2d\x01'
    UNDERLINE_OFF = b'\x1b\x2d\x00'
    ITALIC_ON = b'\x1b\x34'
    ITALIC_OFF = b'\x1b\x35'


class BarcodeType(Enum):
    """Barcode types"""
    UPC_A = b'\x1d\x6b\x00'
    UPC_E = b'\x1d\x6b\x01'
    EAN13 = b'\x1d\x6b\x02'
    EAN8 = b'\x1d\x6b\x03'
    CODE39 = b'\x1d\x6b\x04'
    ITF = b'\x1d\x6b\x05'
    CODE128 = b'\x1d\x6b\x06'


class CommandBuilder:
    """
    ESC/POS Command Builder
    Provides methods to construct printer commands
    """
    
    def __init__(self, encoding: str = "utf-8"):
        """
        Initialize command builder
        
        Args:
            encoding: Character encoding for text
        """
        self.encoding = encoding
        self.buffer = bytearray()
    
    def reset(self) -> "CommandBuilder":
        """Clear the command buffer"""
        self.buffer = bytearray()
        return self
    
    def init(self) -> "CommandBuilder":
        """Initialize printer"""
        self.buffer.extend(b'\x1b\x40')
        return self
    
    def initialize(self) -> "CommandBuilder":
        """Initialize printer (alias for init())"""
        return self.init()
    
    def text(self, text: str) -> "CommandBuilder":
        """
        Add text to buffer
        
        Args:
            text: Text to print
        """
        self.buffer.extend(text.encode(self.encoding))
        return self
    
    def newline(self, lines: int = 1) -> "CommandBuilder":
        """Add line feed(s)"""
        self.buffer.extend(ESC.LF * lines)
        return self
    
    def align(self, alignment: Literal["left", "center", "right"]) -> "CommandBuilder":
        """
        Set text alignment
        
        Args:
            alignment: Text alignment (left, center, right)
        """
        align_map = {
            "left": TextAlign.LEFT,
            "center": TextAlign.CENTER,
            "right": TextAlign.RIGHT,
        }
        self.buffer.extend(align_map[alignment].value)
        return self
    
    def font(self, font: Literal["a", "b", "c"] = "a") -> "CommandBuilder":
        """
        Set font type
        
        Args:
            font: Font type (a, b, c)
        """
        font_map = {
            "a": TextFont.A,
            "b": TextFont.B,
            "c": TextFont.C,
        }
        self.buffer.extend(font_map[font].value)
        return self
    
    def bold(self, enabled: bool = True) -> "CommandBuilder":
        """Enable/disable bold text"""
        style = TextStyle.BOLD_ON if enabled else TextStyle.BOLD_OFF
        self.buffer.extend(style.value)
        return self
    
    def underline(self, enabled: bool = True) -> "CommandBuilder":
        """Enable/disable underline"""
        style = TextStyle.UNDERLINE_ON if enabled else TextStyle.UNDERLINE_OFF
        self.buffer.extend(style.value)
        return self
    
    def size(self, width: int = 1, height: int = 1) -> "CommandBuilder":
        """
        Set text size
        
        Args:
            width: Width multiplier (1-8)
            height: Height multiplier (1-8)
        """
        width = max(1, min(8, width))
        height = max(1, min(8, height))
        size_byte = ((width - 1) << 4) | (height - 1)
        self.buffer.extend(b'\x1d\x21' + bytes([size_byte]))
        return self
    
    def cut(self, partial: bool = False) -> "CommandBuilder":
        """
        Cut paper
        
        Args:
            partial: True for partial cut, False for full cut
        """
        cmd = b'\x1d\x56\x01' if partial else b'\x1d\x56\x00'
        self.buffer.extend(cmd)
        return self
    
    def barcode(self, data: str, barcode_type: str = "CODE128", height: int = 50) -> "CommandBuilder":
        """
        Print barcode
        
        Args:
            data: Barcode data
            barcode_type: Type of barcode
            height: Barcode height in dots
        """
        # Set barcode height
        self.buffer.extend(b'\x1d\x68' + bytes([height]))
        
        # Get barcode type command
        type_map = {
            "UPC_A": BarcodeType.UPC_A,
            "UPC_E": BarcodeType.UPC_E,
            "EAN13": BarcodeType.EAN13,
            "EAN8": BarcodeType.EAN8,
            "CODE39": BarcodeType.CODE39,
            "ITF": BarcodeType.ITF,
            "CODE128": BarcodeType.CODE128,
        }
        
        if barcode_type in type_map:
            self.buffer.extend(type_map[barcode_type].value)
            self.buffer.extend(data.encode(self.encoding))
            self.buffer.extend(b'\x00')  # Null terminator
        
        return self
    
    def qr(self, data: str, size: int = 3) -> "CommandBuilder":
        """
        Print QR code
        
        Args:
            data: QR code data
            size: QR code module size (1-8)
        """
        # QR code commands for ESC/POS
        size = max(1, min(8, size))
        
        # Model
        self.buffer.extend(b'\x1d\x28\x6b\x04\x00\x31\x41\x32\x00')
        
        # Size
        self.buffer.extend(b'\x1d\x28\x6b\x03\x00\x31\x43' + bytes([size]))
        
        # Error correction level (M)
        self.buffer.extend(b'\x1d\x28\x6b\x03\x00\x31\x45\x31')
        
        # Store data
        data_bytes = data.encode(self.encoding)
        data_len = len(data_bytes) + 3
        pL = data_len & 0xFF
        pH = (data_len >> 8) & 0xFF
        self.buffer.extend(b'\x1d\x28\x6b' + bytes([pL, pH]) + b'\x31\x50\x30')
        self.buffer.extend(data_bytes)
        
        # Print
        self.buffer.extend(b'\x1d\x28\x6b\x03\x00\x31\x51\x30')
        
        return self
    
    def image(self, img_source, max_width: int = 576) -> "CommandBuilder":
        """
        Print a raster bit image using GS v 0.

        Args:
            img_source: PIL Image, bytes, base64 string, or file path.
            max_width: Max width in dots (576 for 80mm, 384 for 58mm).
        """
        # Resolve source to PIL Image
        if isinstance(img_source, str):
            # base64 string (may have data URI prefix)
            if img_source.startswith("data:"):
                img_source = img_source.split(",", 1)[1]
            try:
                raw = base64.b64decode(img_source)
                img = Image.open(io.BytesIO(raw))
            except Exception:
                img = Image.open(img_source)
        elif isinstance(img_source, (bytes, bytearray)):
            img = Image.open(io.BytesIO(img_source))
        elif isinstance(img_source, Image.Image):
            img = img_source
        else:
            raise TypeError(f"Unsupported image source type: {type(img_source)}")

        # Resize to fit printer width, keep aspect ratio
        if img.width > max_width:
            ratio = max_width / img.width
            img = img.resize(
                (max_width, int(img.height * ratio)), Image.LANCZOS
            )

        # Width must be multiple of 8 for byte alignment
        new_w = (img.width + 7) // 8 * 8
        if new_w != img.width:
            padded = Image.new("RGB", (new_w, img.height), (255, 255, 255))
            padded.paste(img, (0, 0))
            img = padded

        # Convert to 1-bit: black pixels = 1, white = 0
        img = img.convert("1")

        width_bytes = img.width // 8
        height = img.height

        xL = width_bytes & 0xFF
        xH = (width_bytes >> 8) & 0xFF
        yL = height & 0xFF
        yH = (height >> 8) & 0xFF

        # GS v 0  m  xL xH yL yH  d1..dk
        self.buffer.extend(b'\x1d\x76\x30\x00')
        self.buffer.extend(bytes([xL, xH, yL, yH]))

        pixels = img.load()
        for y in range(height):
            for x_byte in range(width_bytes):
                byte = 0
                for bit in range(8):
                    px = x_byte * 8 + bit
                    # PIL "1" mode: 0 = black, 255 = white
                    if pixels[px, y] == 0:
                        byte |= (0x80 >> bit)
                self.buffer.extend(bytes([byte]))

        return self

    def cash_drawer(self, pin: int = 2) -> "CommandBuilder":
        """
        Trigger cash drawer
        
        Args:
            pin: Pin number (2 or 5)
        """
        if pin == 2:
            self.buffer.extend(b'\x1b\x70\x00\x19\x78')
        elif pin == 5:
            self.buffer.extend(b'\x1b\x70\x01\x19\x78')
        return self
    
    def build(self) -> bytes:
        """
        Get the built command bytes
        
        Returns:
            Command bytes
        """
        return bytes(self.buffer)
    
    def get_bytes(self) -> bytes:
        """Get the built command bytes (alias for build())"""
        return self.build()
    
    def clear(self) -> "CommandBuilder":
        """Clear buffer (alias for reset)"""
        return self.reset()
