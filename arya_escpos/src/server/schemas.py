"""Pydantic schemas for API request/response validation."""
from pydantic import BaseModel, field_validator, model_validator
from typing import List, Literal, Optional


def _validate_hex(v: Optional[str], field_name: str) -> Optional[str]:
    if v is not None:
        try:
            int(v, 16)
        except ValueError:
            raise ValueError(f"{field_name} must be a valid hex string (e.g. '04b8'), got '{v}'")
    return v


class DeviceInfo(BaseModel):
    id: str
    type: str
    name: str | None = None
    manufacturer: str | None = None
    description: str | None = None
    # USB
    vid: str | None = None
    pid: str | None = None
    serial: str | None = None
    # Network
    ip: str | None = None
    port: int | None = None
    # Bluetooth
    address: str | None = None
    channel: int | None = None
    # Serial
    com_port: str | None = None


class ScanResponse(BaseModel):
    found: int
    devices: List[DeviceInfo]


class PrintRequest(BaseModel):
    """Print ESC/POS content to a device.

    The client must specify how to reach the device:
    - type="windows" -> printer_name required
    - type="usb" -> vid + pid required (hex strings, e.g. "04b8", "0202")
    - type="network" -> ip required (port defaults to 9100)
    - type="serial" -> com_port required
    - type="bluetooth" -> address required
    """
    type: Literal["windows", "usb", "network", "serial", "bluetooth"]
    content: str
    # Optional base64-encoded image printed before text (logo, header)
    header_image: str | None = None
    # Max width in dots (576 for 80mm, 384 for 58mm)
    image_width: int = 576
    # Windows
    printer_name: str | None = None
    # USB (hex strings to match scan response)
    vid: str | None = None
    pid: str | None = None
    # Network
    ip: str | None = None
    port: int = 9100
    # Serial
    com_port: str | None = None
    # Bluetooth
    address: str | None = None

    @field_validator("vid", "pid", mode="before")
    @classmethod
    def validate_hex(cls, v, info):
        return _validate_hex(v, info.field_name)

    @model_validator(mode="after")
    def check_required_fields(self):
        if self.type == "windows" and not self.printer_name:
            raise ValueError("printer_name is required for type='windows'")
        if self.type == "usb" and (not self.vid or not self.pid):
            raise ValueError("vid and pid are required for type='usb'")
        if self.type == "network" and not self.ip:
            raise ValueError("ip is required for type='network'")
        if self.type == "serial" and not self.com_port:
            raise ValueError("com_port is required for type='serial'")
        if self.type == "bluetooth" and not self.address:
            raise ValueError("address is required for type='bluetooth'")
        return self


class ReportPrintRequest(BaseModel):
    printer_name: str
    title: str
    content: str


class BarcodeItem(BaseModel):
    """Barcode to print as ESC/P raster graphics."""
    data: str
    type: Literal["code39", "code128", "ean13", "ean8", "upca", "upc"] = "code39"
    width_dots: int = 300
    height_dots: int = 48


class MatrixPrintRequest(BaseModel):
    """Print plain text to a dot matrix (ESC/P) printer.

    Compatible printers (ESC/P / ESC/P2):
    - Epson LX-350, LX-300+II, LX-810, LX-1170
    - Epson FX-890II, FX-2190, DFX-9000
    - Oki Microline 320, 390, 5720
    - Cualquier matricial de 9 o 24 pines compatible con ESC/P

    Connection types:
    - type="windows" -> printer_name required (via driver Windows, recomendado)
    - type="usb"     -> vid + pid required (requiere WinUSB via Zadig)
    - type="serial"  -> com_port required (RS-232)
    """
    type: Literal["windows", "usb", "serial"]
    content: str
    encoding: str = "cp850"
    # Avance de pagina al finalizar (para papel continuo)
    form_feed: bool = True
    # Tipografia
    font: Literal["roman", "sans_serif"] = "roman"
    cpi: Literal[10, 12, 15] = 10
    # Codigos de barra (se imprimen despues del contenido de texto)
    barcodes: List[BarcodeItem] = []
    # Windows
    printer_name: Optional[str] = None
    # USB (hex strings)
    vid: Optional[str] = None
    pid: Optional[str] = None
    # Serial
    com_port: Optional[str] = None
    baud_rate: int = 9600

    @field_validator("vid", "pid", mode="before")
    @classmethod
    def validate_hex(cls, v, info):
        return _validate_hex(v, info.field_name)

    @model_validator(mode="after")
    def check_required_fields(self):
        if self.type == "windows" and not self.printer_name:
            raise ValueError("printer_name is required for type='windows'")
        if self.type == "usb" and (not self.vid or not self.pid):
            raise ValueError("vid and pid are required for type='usb'")
        if self.type == "serial" and not self.com_port:
            raise ValueError("com_port is required for type='serial'")
        return self
