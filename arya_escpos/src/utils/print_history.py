"""In-memory print job history. Thread-safe, max 500 entries, resets on service restart.

Usage:
    from utils.print_history import record, get_history

    # After sending a job to the printer:
    record(printer_name="LX-350", document="factura.txt", protocol="escp", bytes_sent=312)

    # Query history:
    jobs = get_history(limit=20, printer_name="LX-350")
"""
import time
import threading
from collections import deque
from datetime import datetime
from typing import Optional

_lock = threading.Lock()
_history: deque = deque(maxlen=500)


def record(
    printer_name: str,
    document: str,
    protocol: str,
    bytes_sent: int,
    job_id: Optional[int] = None,
) -> dict:
    """Record a print job after successfully sending to printer.

    Args:
        printer_name: Nombre de la impresora destino.
        document: Nombre o descripcion del documento/contenido.
        protocol: "escpos" | "escp" | "document" | "report"
        bytes_sent: Bytes enviados a la impresora.
        job_id: ID del trabajo en la cola Windows (si aplica).

    Returns:
        El registro guardado.
    """
    entry = {
        "id": int(time.time() * 1000),
        "timestamp": datetime.now().isoformat(timespec="seconds"),
        "printer_name": printer_name,
        "document": document,
        "protocol": protocol,
        "bytes_sent": bytes_sent,
        "job_id": job_id,
        "status": "sent",
    }
    with _lock:
        _history.appendleft(entry)
    return entry


def get_history(limit: int = 50, printer_name: Optional[str] = None) -> list:
    """Get recent print history.

    Args:
        limit: Max number of entries to return (1-500).
        printer_name: Optional filter by printer name (case-insensitive).

    Returns:
        List of job dicts, newest first.
    """
    with _lock:
        items = list(_history)

    if printer_name:
        items = [j for j in items if j["printer_name"].lower() == printer_name.lower()]

    limit = max(1, min(limit, 500))
    return items[:limit]


def clear() -> None:
    """Clear all history entries."""
    with _lock:
        _history.clear()
