"""Tests para el endpoint ESC/P de impresoras matriciales y MatrixCommandBuilder."""

import pytest
from pydantic import ValidationError
from utils.matrix_command_builder import MatrixCommandBuilder
from server.schemas import MatrixPrintRequest, BarcodeItem

ESC = b"\x1b"
FF = b"\x0c"
LF = b"\n"


# ── Schema: MatrixPrintRequest ─────────────────────────────────────────────────

class TestMatrixPrintRequestSchema:

    def test_windows_type_ok(self):
        req = MatrixPrintRequest(type="windows", printer_name="Epson LX-350", content="test")
        assert req.type == "windows"
        assert req.printer_name == "Epson LX-350"

    def test_serial_type_ok(self):
        req = MatrixPrintRequest(type="serial", com_port="COM1", content="test")
        assert req.com_port == "COM1"
        assert req.baud_rate == 9600

    def test_usb_type_ok(self):
        req = MatrixPrintRequest(type="usb", vid="04b8", pid="0880", content="test")
        assert req.vid == "04b8"
        assert req.pid == "0880"

    def test_windows_missing_printer_name(self):
        with pytest.raises((ValueError, ValidationError)):
            MatrixPrintRequest(type="windows", content="test")

    def test_serial_missing_com_port(self):
        with pytest.raises((ValueError, ValidationError)):
            MatrixPrintRequest(type="serial", content="test")

    def test_usb_missing_vid_pid(self):
        with pytest.raises((ValueError, ValidationError)):
            MatrixPrintRequest(type="usb", content="test")

    def test_defaults(self):
        req = MatrixPrintRequest(type="windows", printer_name="test", content="hello")
        assert req.font == "roman"
        assert req.cpi == 10
        assert req.encoding == "cp850"
        assert req.form_feed is True
        assert req.barcodes == []

    def test_custom_font_cpi(self):
        req = MatrixPrintRequest(
            type="windows", printer_name="test", content="hello",
            font="sans_serif", cpi=12
        )
        assert req.font == "sans_serif"
        assert req.cpi == 12

    def test_barcodes_field(self):
        req = MatrixPrintRequest(
            type="windows",
            printer_name="test",
            content="factura",
            barcodes=[BarcodeItem(data="12345678", type="code39")]
        )
        assert len(req.barcodes) == 1
        assert req.barcodes[0].data == "12345678"
        assert req.barcodes[0].type == "code39"


# ── Schema: BarcodeItem ────────────────────────────────────────────────────────

class TestBarcodeItemSchema:

    def test_default_type(self):
        b = BarcodeItem(data="12345")
        assert b.type == "code39"
        assert b.width_dots == 300
        assert b.height_dots == 48

    def test_custom_barcode(self):
        b = BarcodeItem(data="123456789012", type="ean13", width_dots=400, height_dots=64)
        assert b.type == "ean13"
        assert b.width_dots == 400


# ── MatrixCommandBuilder ───────────────────────────────────────────────────────

class TestMatrixCommandBuilder:

    def test_init_command(self):
        builder = MatrixCommandBuilder()
        builder.init()
        assert ESC + b"@" in builder.build()

    def test_text_content(self):
        builder = MatrixCommandBuilder()
        builder.text("Hola mundo")
        assert b"Hola mundo" in builder.build()

    def test_text_encoding(self):
        builder = MatrixCommandBuilder(encoding="cp850")
        builder.text("Cafe")
        assert b"Cafe" in builder.build()

    def test_font_roman(self):
        builder = MatrixCommandBuilder()
        builder.font("roman")
        assert ESC + b"k\x00" in builder.build()

    def test_font_sans_serif(self):
        builder = MatrixCommandBuilder()
        builder.font("sans_serif")
        assert ESC + b"k\x01" in builder.build()

    def test_cpi_10(self):
        builder = MatrixCommandBuilder()
        builder.cpi(10)
        assert ESC + b"P" in builder.build()

    def test_cpi_12(self):
        builder = MatrixCommandBuilder()
        builder.cpi(12)
        assert ESC + b"M" in builder.build()

    def test_cpi_15(self):
        builder = MatrixCommandBuilder()
        builder.cpi(15)
        assert ESC + b"g" in builder.build()

    def test_line_spacing_sixth(self):
        builder = MatrixCommandBuilder()
        builder.line_spacing_sixth()
        assert ESC + b"2" in builder.build()

    def test_form_feed(self):
        builder = MatrixCommandBuilder()
        builder.form_feed()
        assert FF in builder.build()

    def test_newline(self):
        builder = MatrixCommandBuilder()
        builder.newline(3)
        assert LF * 3 in builder.build()

    def test_full_flow(self):
        """Secuencia completa de impresion matricial."""
        builder = MatrixCommandBuilder()
        data = (
            builder
            .init()
            .font("roman")
            .cpi(10)
            .line_spacing_sixth()
            .text("FACTURA #001\n")
            .text("Cliente: Juan Perez\n")
            .text("Total: $100.00\n")
            .newline(2)
            .form_feed()
            .build()
        )
        assert ESC + b"@" in data
        assert b"FACTURA" in data
        assert b"Juan Perez" in data
        assert FF in data
        assert len(data) > 20

    def test_build_returns_bytes(self):
        builder = MatrixCommandBuilder()
        builder.text("test")
        result = builder.build()
        assert isinstance(result, bytes)

    def test_chaining(self):
        """Los metodos retornan self para encadenamiento."""
        builder = MatrixCommandBuilder()
        result = builder.init().font("roman").cpi(10).text("test").form_feed()
        assert result is builder

    def test_barcode_fallback_without_library(self):
        """Si python-barcode no esta instalado, usa fallback de texto."""
        import sys
        import unittest.mock as mock

        builder = MatrixCommandBuilder()
        # Simular ImportError en barcode
        with mock.patch.dict(sys.modules, {"barcode": None}):
            builder.barcode("12345", barcode_type="code39")

        data = builder.build()
        # El fallback debe incluir el dato del barcode
        assert b"12345" in data

    def test_no_form_feed_option(self):
        """Verificar que form_feed es opcional."""
        builder = MatrixCommandBuilder()
        builder.init().text("sin corte")
        data = builder.build()
        assert FF not in data


# ── Print history ──────────────────────────────────────────────────────────────

class TestPrintHistory:

    def setup_method(self):
        from utils.print_history import clear
        clear()

    def test_record_and_retrieve(self):
        from utils.print_history import record, get_history
        record(printer_name="LX-350", document="factura.txt", protocol="escp", bytes_sent=312)
        jobs = get_history()
        assert len(jobs) == 1
        assert jobs[0]["printer_name"] == "LX-350"
        assert jobs[0]["protocol"] == "escp"
        assert jobs[0]["bytes_sent"] == 312

    def test_newest_first(self):
        from utils.print_history import record, get_history
        record(printer_name="P1", document="doc1", protocol="escpos", bytes_sent=100)
        record(printer_name="P1", document="doc2", protocol="escpos", bytes_sent=200)
        jobs = get_history()
        assert jobs[0]["document"] == "doc2"
        assert jobs[1]["document"] == "doc1"

    def test_filter_by_printer(self):
        from utils.print_history import record, get_history
        record(printer_name="LX-350", document="a", protocol="escp", bytes_sent=50)
        record(printer_name="TM-T88", document="b", protocol="escpos", bytes_sent=100)
        jobs = get_history(printer_name="LX-350")
        assert len(jobs) == 1
        assert jobs[0]["printer_name"] == "LX-350"

    def test_limit(self):
        from utils.print_history import record, get_history
        for i in range(10):
            record(printer_name="P1", document=f"doc{i}", protocol="escp", bytes_sent=i * 10)
        jobs = get_history(limit=3)
        assert len(jobs) == 3

    def test_record_with_job_id(self):
        from utils.print_history import record, get_history
        record(printer_name="HP", document="report.pdf", protocol="document", bytes_sent=5000, job_id=42)
        jobs = get_history()
        assert jobs[0]["job_id"] == 42

    def test_clear_history(self):
        from utils.print_history import record, get_history, clear
        record(printer_name="P1", document="doc", protocol="escp", bytes_sent=100)
        clear()
        assert get_history() == []
