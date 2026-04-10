"""ESC/P command builder for dot matrix printers.

Supports: Epson LX-350, LX-300+II, FX-890, DFX-9000, Oki Microline, and
any 9-pin or 24-pin printer compatible with ESC/P protocol.

ESC/P Reference:
    ESC @       Initialize printer
    ESC k n     Select typeface (0=Roman, 1=Sans Serif)
    ESC P       Set 10 cpi (pica)
    ESC M       Set 12 cpi (elite)
    ESC g       Set 15 cpi (condensed)
    ESC 2       Set 1/6 inch line spacing
    ESC * m ... Print raster graphics
    FF          Form feed (eject page)
    LF          Line feed
"""
import io
from typing import Optional


class MatrixCommandBuilder:
    """Builds raw ESC/P byte sequences for dot matrix printers."""

    ESC = b"\x1b"
    LF = b"\n"
    CR = b"\r"
    FF = b"\x0c"

    # ESC k n - Typeface selection
    FONTS = {
        "roman": b"\x00",
        "sans_serif": b"\x01",
    }

    # CPI commands
    CPI_COMMANDS = {
        10: b"\x1bP",   # ESC P = 10 cpi (pica)
        12: b"\x1bM",   # ESC M = 12 cpi (elite)
        15: b"\x1bg",   # ESC g = 15 cpi (condensed)
    }

    def __init__(self, encoding: str = "cp850"):
        self.buffer = bytearray()
        self.encoding = encoding

    # ── Initialization ─────────────────────────────────────────────────────

    def init(self) -> "MatrixCommandBuilder":
        """ESC @ — Initialize printer (reset all settings)."""
        self.buffer += self.ESC + b"@"
        return self

    # ── Typography ─────────────────────────────────────────────────────────

    def font(self, font_type: str = "roman") -> "MatrixCommandBuilder":
        """ESC k n — Select typeface.

        Args:
            font_type: "roman" or "sans_serif"
        """
        f = self.FONTS.get(font_type, b"\x00")
        self.buffer += self.ESC + b"k" + f
        return self

    def cpi(self, chars_per_inch: int = 10) -> "MatrixCommandBuilder":
        """Set characters per inch (10, 12, or 15).

        10 = pica (default), 12 = elite, 15 = condensed
        """
        cmd = self.CPI_COMMANDS.get(chars_per_inch)
        if cmd:
            self.buffer += cmd
        return self

    def line_spacing_sixth(self) -> "MatrixCommandBuilder":
        """ESC 2 — Set 1/6 inch line spacing (standard for most forms)."""
        self.buffer += self.ESC + b"2"
        return self

    def line_spacing_eighth(self) -> "MatrixCommandBuilder":
        """ESC 0 — Set 1/8 inch line spacing (compact)."""
        self.buffer += self.ESC + b"0"
        return self

    # ── Content ────────────────────────────────────────────────────────────

    def text(self, content: str) -> "MatrixCommandBuilder":
        """Add plain text content."""
        self.buffer += content.encode(self.encoding, errors="replace")
        return self

    def newline(self, count: int = 1) -> "MatrixCommandBuilder":
        """Add line feeds."""
        self.buffer += self.LF * count
        return self

    def form_feed(self) -> "MatrixCommandBuilder":
        """FF — Advance page (eject continuous paper)."""
        self.buffer += self.FF
        return self

    # ── Barcodes via raster graphics ───────────────────────────────────────

    def barcode(
        self,
        data: str,
        barcode_type: str = "code39",
        width_dots: int = 300,
        height_dots: int = 48,
    ) -> "MatrixCommandBuilder":
        """Print a barcode as ESC/P raster graphics.

        Renders the barcode using python-barcode + Pillow, then converts
        the image to ESC * double-density raster commands.

        Falls back to a plain text label if python-barcode is not installed.

        Args:
            data: Barcode content (digits/chars depending on type).
            barcode_type: "code39", "code128", "ean13", "ean8", "upca", "upc".
            width_dots: Horizontal size in printer dots (default 300).
            height_dots: Vertical size in printer dots (must be multiple of 8).
        """
        try:
            import barcode as bc
            from barcode.writer import ImageWriter
            from PIL import Image

            bc_class = bc.get_barcode_class(barcode_type.lower())
            writer = ImageWriter()

            fp = io.BytesIO()
            bc_class(data, writer=writer).write(
                fp,
                options={
                    "module_width": 10,
                    "module_height": height_dots / 10,
                    "quiet_zone": 6.5,
                    "font_size": 10,
                    "text_distance": 5.0,
                    "background": "white",
                    "foreground": "black",
                },
            )
            fp.seek(0)

            img = Image.open(fp).convert("1")   # 1-bit B&W
            img = img.resize((width_dots, height_dots), Image.LANCZOS)

            self.buffer += self._image_to_escp_raster(img)
            self.buffer += self.LF

        except (ImportError, Exception):
            # Fallback: plain text label (not scannable but visible)
            label = f"[{barcode_type.upper()}: {data}]"
            self.buffer += label.encode(self.encoding, errors="replace") + self.LF

        return self

    def _image_to_escp_raster(self, img) -> bytes:
        """Convert a 1-bit PIL image to ESC/P double-density raster commands.

        Uses ESC * 1 (120 dpi horizontal, 60 dpi vertical) which is
        supported by all ESC/P printers including the Epson LX-350.

        Each raster band is 8 pixels tall (one byte per column).
        """
        result = bytearray()
        width, height = img.size

        for row_start in range(0, height, 8):
            cols = bytearray()

            for x in range(width):
                byte = 0
                for bit in range(8):
                    y = row_start + bit
                    if y < height:
                        pixel = img.getpixel((x, y))
                        if pixel == 0:          # Black pixel = print dot
                            byte |= (1 << (7 - bit))
                cols.append(byte)

            nL = len(cols) & 0xFF
            nH = (len(cols) >> 8) & 0xFF

            # ESC * 1 nL nH — double density graphics (120 dpi H, 60 dpi V)
            result += self.ESC + b"*\x01" + bytes([nL, nH]) + bytes(cols)
            result += self.LF

        return bytes(result)

    # ── Output ─────────────────────────────────────────────────────────────

    def build(self) -> bytes:
        """Return the complete ESC/P byte sequence."""
        return bytes(self.buffer)
