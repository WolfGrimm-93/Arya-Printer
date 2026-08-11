"""
Document Converter Utility
Converts various document formats to PDF for printing.
"""
import os
import tempfile
from pathlib import Path
from typing import Optional

from utils.logger import get_logger

logger = get_logger()


def convert_to_pdf(file_path: str) -> Optional[str]:
    """Convert a document to PDF format.

    Returns:
        Path to the converted PDF file, or None if conversion failed.
    """
    ext = Path(file_path).suffix.lower()

    if ext == ".pdf":
        return file_path
    if ext in [".doc", ".docx"]:
        return _convert_word_to_pdf(file_path)
    if ext in [".jpg", ".jpeg", ".png", ".bmp", ".gif", ".tiff"]:
        return _convert_image_to_pdf(file_path)
    if ext in [".txt", ".log", ".csv"]:
        return _convert_text_to_pdf(file_path)
    if ext in [".xls", ".xlsx"]:
        return _convert_excel_to_pdf(file_path)

    return None


def get_document_type(file_path: str) -> str:
    """Get document type from file extension.

    Returns:
        Type: 'pdf', 'word', 'excel', 'image', 'text', 'unknown'
    """
    ext = Path(file_path).suffix.lower()

    if ext == ".pdf":
        return "pdf"
    if ext in [".doc", ".docx"]:
        return "word"
    if ext in [".xls", ".xlsx"]:
        return "excel"
    if ext in [".jpg", ".jpeg", ".png", ".bmp", ".gif", ".tiff"]:
        return "image"
    if ext in [".txt", ".log", ".csv"]:
        return "text"
    return "unknown"


def _convert_word_to_pdf(word_path: str) -> Optional[str]:
    """Convert Word document to PDF."""
    try:
        from docx2pdf import convert

        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
            pdf_path = f.name

        convert(word_path, pdf_path)

        if os.path.exists(pdf_path):
            return pdf_path

    except Exception as e:
        logger.error(f"Error converting Word to PDF: {e}")

    return None


def _convert_image_to_pdf(image_path: str) -> Optional[str]:
    """Convert image to PDF."""
    try:
        from PIL import Image

        img = Image.open(image_path)
        if img.mode != "RGB":
            img = img.convert("RGB")

        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
            pdf_path = f.name

        img.save(pdf_path, "PDF", resolution=300.0)
        return pdf_path

    except Exception as e:
        logger.error(f"Error converting image to PDF: {e}")

    return None


def _convert_text_to_pdf(text_path: str) -> Optional[str]:
    """Convert text file to PDF.

    Usa fpdf2 (MIT) en vez de PyMuPDF/fitz (AGPL-3.0/comercial) -- ver
    pdf_printer.py para el razonamiento completo del reemplazo. A diferencia
    del fitz.insert_textbox() original (que truncaba en silencio el texto
    que no entraba en una sola pagina), set_auto_page_break() agrega paginas
    nuevas automaticamente -- corrige esa limitacion conocida en vez de
    replicarla.
    """
    try:
        from fpdf import FPDF

        with open(text_path, "r", encoding="utf-8", errors="ignore") as f:
            text = f.read()

        pdf = FPDF(format=(595, 842), unit="pt")  # A4 en puntos, igual que antes
        pdf.set_margins(left=50, top=50, right=50)
        pdf.set_auto_page_break(auto=True, margin=50)
        pdf.add_page()
        pdf.set_font("Courier", size=10)
        pdf.multi_cell(0, 12, text)

        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
            pdf_path = f.name

        pdf.output(pdf_path)
        return pdf_path

    except Exception as e:
        logger.error(f"Error converting text to PDF: {e}")

    return None


def _convert_excel_to_pdf(excel_path: str) -> Optional[str]:
    """Convert Excel to PDF using COM automation."""
    excel = None
    try:
        import win32com.client
        import pythoncom

        pythoncom.CoInitialize()

        excel = win32com.client.Dispatch("Excel.Application")
        excel.Visible = False
        excel.DisplayAlerts = False

        wb = excel.Workbooks.Open(os.path.abspath(excel_path))

        with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
            pdf_path = f.name

        wb.ExportAsFixedFormat(0, pdf_path)  # 0 = xlTypePDF
        wb.Close(False)
        excel.Quit()
        excel = None
        pythoncom.CoUninitialize()

        if os.path.exists(pdf_path):
            return pdf_path

    except Exception as e:
        logger.error(f"Error converting Excel to PDF: {e}")
        if excel is not None:
            try:
                excel.Quit()
            except Exception:
                pass
        try:
            import pythoncom
            pythoncom.CoUninitialize()
        except Exception:
            pass

    return None
