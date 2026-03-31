"""Printing routes - no database, client provides connection details."""
import asyncio
import os
import subprocess
import tempfile
from fastapi import APIRouter, HTTPException, UploadFile, File

from server.compat import WINDOWS_PRINTER_AVAILABLE, win32print, win32api, win32con, pywintypes
from server.adapter_factory import create_adapter, send_raw_to_windows
from core.command_builder import CommandBuilder
from utils.logger import get_logger
from utils.document_converter import convert_to_pdf, get_document_type
from utils.pdf_printer import print_pdf_native, get_job_id_from_queue
from server.schemas import PrintRequest, ReportPrintRequest

router = APIRouter()
logger = get_logger()


@router.post("/print", tags=["Printing"])
async def print_ticket(request: PrintRequest):
    """Print ESC/POS text to a device. Client provides connection details.

    The content is sent as plain text wrapped in basic ESC/POS commands
    (init + text encoding + cut). The client controls the content.
    """
    builder = CommandBuilder(encoding="cp850")
    builder.init()
    if request.header_image:
        builder.align("center")
        builder.image(request.header_image, max_width=request.image_width)
        builder.align("left")
        builder.newline()
    builder.text(request.content)
    builder.text("\n\n\n")
    builder.cut()
    data = builder.build()

    await asyncio.to_thread(_send_to_device, request, data)

    return {
        "success": True,
        "message": f"Print job sent ({request.type})",
        "bytes_sent": len(data),
    }


@router.post("/print/report", tags=["Printing"])
async def print_report(request: ReportPrintRequest):
    """Print a formatted report to a Windows printer via RAW spooler."""
    if not WINDOWS_PRINTER_AVAILABLE:
        raise HTTPException(status_code=500, detail="Windows printer support not available")

    lines = [
        request.title.center(80),
        "=" * 80,
        "",
        request.content,
        "",
        "-" * 80,
    ]
    text_bytes = "\n".join(lines).encode("utf-8")

    await asyncio.to_thread(
        send_raw_to_windows, request.printer_name, text_bytes, "Arya Report"
    )

    return {"success": True, "message": f"Report sent to {request.printer_name}"}


@router.post("/print/document", tags=["Printing"])
async def print_document(
    printer_name: str,
    file: UploadFile = File(...),
    orientation: str = "portrait",
    copies: int = 1,
    color: str = "color",
    duplex: str = "simplex",
):
    """Print a document file (PDF, DOCX, TXT, etc.) to a Windows printer."""
    if not WINDOWS_PRINTER_AVAILABLE:
        raise HTTPException(status_code=500, detail="Windows printer support not available")

    suffix = os.path.splitext(file.filename)[1] or ".pdf"

    with tempfile.NamedTemporaryFile(mode="wb", suffix=suffix, delete=False) as f:
        content = await file.read()
        f.write(content)
        temp_path = f.name

    pdf_path = temp_path
    converted = False

    try:
        doc_type = get_document_type(temp_path)
        logger.info(f"Document type detected: {doc_type} ({suffix})")

        if doc_type != "pdf":
            logger.info(f"Converting {doc_type} to PDF...")
            converted_pdf = await asyncio.to_thread(convert_to_pdf, temp_path)
            if converted_pdf:
                pdf_path = converted_pdf
                converted = True
            else:
                logger.warning(f"Failed to convert {doc_type} to PDF")

        if not pdf_path.lower().endswith(".pdf"):
            raise HTTPException(status_code=400, detail=f"Unsupported document type: {doc_type}")

        job_id = await asyncio.to_thread(
            _print_pdf_to_windows, pdf_path, printer_name, orientation, copies, color, duplex, file.filename
        )

    finally:
        try:
            os.unlink(temp_path)
        except OSError:
            pass
        if converted and pdf_path != temp_path:
            try:
                os.unlink(pdf_path)
            except OSError:
                pass

    file_size_bytes = len(content)
    if file_size_bytes < 1024:
        file_size_str = f"{file_size_bytes} bytes"
    elif file_size_bytes < 1024 * 1024:
        file_size_str = f"{file_size_bytes / 1024:.2f} KB"
    else:
        file_size_str = f"{file_size_bytes / (1024 * 1024):.2f} MB"

    return {
        "success": True,
        "message": f"Document '{file.filename}' sent to {printer_name}",
        "file_size_bytes": file_size_bytes,
        "file_size": file_size_str,
        "job_id": job_id,
        "track_url": f"/api/v1/devices/windows/{printer_name.replace(' ', '_')}/jobs/{job_id}" if job_id else None,
    }


# --------------- Private helpers ---------------


def _send_to_device(request: PrintRequest, data: bytes):
    """Send raw bytes to the appropriate device. Runs in a thread.

    Validation is already done by Pydantic model_validator in PrintRequest.
    """
    if request.type == "windows":
        send_raw_to_windows(request.printer_name, data)

    elif request.type == "usb":
        vid = int(request.vid, 16)
        pid = int(request.pid, 16)
        adapter = create_adapter("usb", vid=vid, pid=pid)
        try:
            adapter.write(data)
            logger.info(f"Printed to USB device: {request.vid}:{request.pid}")
        finally:
            adapter.close()

    elif request.type == "network":
        adapter = create_adapter("network", host=request.ip, port=request.port)
        try:
            adapter.write(data)
            logger.info(f"Printed to network device: {request.ip}:{request.port}")
        finally:
            adapter.close()

    elif request.type == "serial":
        adapter = create_adapter("serial", port=request.com_port)
        try:
            adapter.write(data)
            logger.info(f"Printed to serial device: {request.com_port}")
        finally:
            adapter.close()

    elif request.type == "bluetooth":
        adapter = create_adapter("bluetooth", address=request.address)
        try:
            adapter.write(data)
            logger.info(f"Printed to Bluetooth device: {request.address}")
        finally:
            adapter.close()


def _print_pdf_to_windows(pdf_path: str, printer_name: str, orientation: str, copies: int, color: str, duplex: str, doc_name: str = "") -> int | None:
    """Print a PDF file to a Windows printer with print settings."""
    abs_pdf_path = os.path.abspath(pdf_path)

    # Build print options to pass to the native printer
    print_options = {
        "orientation": orientation,
        "copies": max(1, copies),
        "color": color,
        "duplex": duplex,
    }

    # Method 1: Native Python (PyMuPDF + win32ui) - supports DEVMODE
    try:
        if print_pdf_native(abs_pdf_path, printer_name, **print_options):
            logger.info(f"PDF printed via native method to {printer_name}")
            job_id = get_job_id_from_queue(printer_name, doc_name)
            logger.info(f"Captured job ID: {job_id}")
            return job_id
    except Exception as e:
        logger.warning(f"Native PDF printing failed: {e}")

    # Method 2: PDFtoPrinter
    pdftoprinter_paths = [
        r"C:\Program Files\PDFtoPrinter\PDFtoPrinter.exe",
        r"C:\Program Files (x86)\PDFtoPrinter\PDFtoPrinter.exe",
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "libs", "PDFtoPrinter.exe"),
    ]
    for exe in pdftoprinter_paths:
        if os.path.exists(exe):
            try:
                cmd = [exe, abs_pdf_path, printer_name]
                if copies > 1:
                    cmd.extend(["/copies", str(copies)])
                subprocess.run(cmd, check=True, capture_output=True, timeout=30)
                logger.info(f"PDF printed via PDFtoPrinter to {printer_name}")
                job_id = get_job_id_from_queue(printer_name, doc_name)
                logger.info(f"Captured job ID: {job_id}")
                return job_id
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
                logger.warning(f"PDFtoPrinter failed: {e}")

    # Method 3: SumatraPDF
    sumatra_paths = [
        r"C:\Program Files\SumatraPDF\SumatraPDF.exe",
        r"C:\Program Files (x86)\SumatraPDF\SumatraPDF.exe",
    ]
    for exe in sumatra_paths:
        if os.path.exists(exe):
            try:
                cmd = [exe, "-print-to", printer_name, "-silent"]
                if orientation.lower() == "landscape":
                    cmd.extend(["-print-settings", "landscape"])
                cmd.append(abs_pdf_path)
                subprocess.run(cmd, check=True, capture_output=True, timeout=30)
                logger.info(f"PDF printed via SumatraPDF to {printer_name}")
                job_id = get_job_id_from_queue(printer_name, doc_name)
                logger.info(f"Captured job ID: {job_id}")
                return job_id
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
                logger.warning(f"SumatraPDF failed: {e}")

    raise RuntimeError("All PDF printing methods failed")
