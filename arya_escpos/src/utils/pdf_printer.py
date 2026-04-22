"""
PDF Printer Utility
Uses PyMuPDF to render PDF pages and win32print to print them.
"""
import io
from pathlib import Path

from utils.logger import get_logger

logger = get_logger()

try:
    import fitz  # PyMuPDF
    import win32ui
    import win32con
    import win32print
    from PIL import Image, ImageWin
    PDF_NATIVE_AVAILABLE = True
except ImportError:
    PDF_NATIVE_AVAILABLE = False


def get_job_id_from_queue(printer_name: str, doc_name: str, timeout_sec: int = 5) -> int | None:
    """Get the Job ID of a recently submitted document by searching the queue.

    Looks for a document with matching name in the printer queue.
    Returns the Job ID or None if not found.
    """
    import time

    printer_handle = None
    try:
        printer_handle = win32print.OpenPrinter(printer_name)
        start_time = time.time()
        last_error = None

        while time.time() - start_time < timeout_sec:
            try:
                jobs = win32print.EnumJobs(printer_handle, 0, -1, 1)

                if not jobs:
                    logger.debug(f"No jobs in queue yet for {printer_name}")
                    time.sleep(0.2)
                    continue

                # Look for job with matching document name
                for job in jobs:
                    job_doc = job.get("pDocument", "").lower()
                    doc_match = doc_name.lower()
                    doc_basename = Path(doc_name).name.lower()

                    logger.debug(f"Queue job: {job_doc} vs search: {doc_match}")

                    # Match by exact filename, basename, or stem
                    if (doc_match == job_doc or
                        doc_basename == job_doc or
                        doc_match in job_doc or
                        doc_basename in job_doc or
                        Path(doc_match).stem.lower() in job_doc):
                        job_id = job.get("JobId")
                        logger.info(f"Captured job ID {job_id}: {job_doc}")
                        return job_id

            except Exception as e:
                last_error = e
                logger.debug(f"EnumJobs error (attempt): {e}")
                time.sleep(0.2)
                continue

            time.sleep(0.2)

        if last_error:
            logger.warning(f"Could not retrieve job ID after {timeout_sec}s: {last_error}")
        else:
            logger.warning(f"Job ID not found in queue after {timeout_sec}s for {doc_name}")
        return None

    except Exception as e:
        logger.error(f"Failed to open printer {printer_name}: {e}")
        return None
    finally:
        if printer_handle:
            try:
                win32print.ClosePrinter(printer_handle)
            except Exception:
                pass


def print_pdf_native(
    pdf_path: str,
    printer_name: str,
    orientation: str = "portrait",
    copies: int = 1,
    color: str = "color",
    duplex: str = "simplex",
    **kwargs,
) -> bool:
    """Print a PDF file using native Windows printing through win32ui.

    Args:
        pdf_path: Path to the PDF file.
        printer_name: Name of the printer.
        orientation: "portrait" or "landscape".
        copies: Number of copies.
        color: "color" or "grayscale".
        duplex: "simplex" or "duplex".

    Returns:
        True if successful.
    """
    if not PDF_NATIVE_AVAILABLE:
        raise RuntimeError("PyMuPDF, win32ui or Pillow not available")

    doc = None
    hDC = None
    try:
        doc = fitz.open(pdf_path)

        hDC = win32ui.CreateDC()
        hDC.CreatePrinterDC(printer_name)

        # Apply DEVMODE settings
        try:
            devmode = hDC.GetDevMode()
            devmode.Fields |= win32con.DM_ORIENTATION
            devmode.Orientation = (
                win32con.DMORIENT_LANDSCAPE if orientation.lower() == "landscape"
                else win32con.DMORIENT_PORTRAIT
            )
            devmode.Fields |= win32con.DM_COPIES
            devmode.Copies = max(1, copies)
            devmode.Fields |= win32con.DM_DUPLEX
            devmode.Duplex = 3 if duplex.lower() == "duplex" else 1
            devmode.Fields |= win32con.DM_COLOR
            devmode.Color = (
                win32con.DMCOLOR_MONOCHROME if color.lower() == "grayscale"
                else win32con.DMCOLOR_COLOR
            )
            hDC.ResetDC(devmode)
        except Exception as e:
            logger.warning(f"Could not apply DEVMODE settings: {e}")

        hDC.StartDoc(Path(pdf_path).name)

        for page_num in range(len(doc)):
            page = doc[page_num]

            # Render page to image at 150 DPI
            mat = fitz.Matrix(150 / 72, 150 / 72)
            pix = page.get_pixmap(matrix=mat, alpha=False)

            img_data = pix.tobytes("ppm")
            img = Image.open(io.BytesIO(img_data))

            hDC.StartPage()

            printer_width = hDC.GetDeviceCaps(win32con.HORZRES)
            printer_height = hDC.GetDeviceCaps(win32con.VERTRES)

            img_width, img_height = img.size
            scale = min(printer_width / img_width, printer_height / img_height)

            scaled_width = int(img_width * scale)
            scaled_height = int(img_height * scale)
            x = (printer_width - scaled_width) // 2
            y = (printer_height - scaled_height) // 2

            dib = ImageWin.Dib(img)
            dib.draw(hDC.GetHandleOutput(), (x, y, x + scaled_width, y + scaled_height))

            hDC.EndPage()

        hDC.EndDoc()
        return True

    except Exception as e:
        logger.error(f"Native PDF printing failed: {e}")
        try:
            if hDC:
                hDC.AbortDoc()
        except Exception:
            pass
        return False

    finally:
        try:
            if hDC:
                hDC.DeleteDC()
        except Exception:
            pass
        try:
            if doc:
                doc.close()
        except Exception:
            pass
