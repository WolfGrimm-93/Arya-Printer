"""Device discovery and status routes - no database, live discovery only."""
import asyncio
from fastapi import APIRouter, HTTPException

from adapters.usb_adapter import USBAdapter
from adapters.network_adapter import NetworkAdapter
from adapters.serial_adapter import SerialAdapter
from adapters.bluetooth_adapter import BluetoothAdapter, PYBLUEZ_AVAILABLE
from server.compat import WINDOWS_PRINTER_AVAILABLE, win32print
from server.adapter_factory import create_adapter
from server.schemas import DeviceInfo, ScanResponse
from utils.logger import get_logger
from utils.config import get_config

router = APIRouter()
logger = get_logger()


@router.get("/devices/scan", response_model=ScanResponse, tags=["Devices"])
async def scan_devices():
    """Scan for available devices across all protocols (live discovery)."""
    config = get_config()
    devices: list[DeviceInfo] = []

    # Windows printers
    devices.extend(await asyncio.to_thread(_scan_windows_printers))

    # USB printers
    usb_printers = await asyncio.to_thread(USBAdapter.find_printers)
    for p in usb_printers:
        devices.append(DeviceInfo(
            id=f"usb-{p['vid']}:{p['pid']}",
            type="usb",
            name=p.get("product"),
            manufacturer=p.get("manufacturer"),
            vid=p["vid"],
            pid=p["pid"],
            serial=p.get("serial"),
        ))
    logger.info(f"Found {len(usb_printers)} USB printers")

    # Bluetooth
    if config.discovery.bluetooth_enabled and PYBLUEZ_AVAILABLE:
        try:
            duration = config.discovery.bluetooth_scan_duration
            bt_printers = await asyncio.to_thread(BluetoothAdapter.find_printers, duration)
            for p in bt_printers:
                devices.append(DeviceInfo(
                    id=f"bt-{p['address']}",
                    type="bluetooth",
                    name=p.get("name", "Unknown"),
                    address=p["address"],
                    channel=p.get("channel"),
                ))
            logger.info(f"Found {len(bt_printers)} Bluetooth printers")
        except Exception as e:
            logger.error(f"Bluetooth scan error: {e}")

    # Serial ports
    if config.discovery.serial_enabled:
        try:
            ports = await asyncio.to_thread(SerialAdapter.list_ports)
            for port_name in ports:
                devices.append(DeviceInfo(
                    id=f"serial-{port_name}",
                    type="serial",
                    name=port_name,
                    com_port=port_name,
                ))
            logger.info(f"Found {len(ports)} serial ports")
        except Exception as e:
            logger.error(f"Serial scan error: {e}")

    return ScanResponse(found=len(devices), devices=devices)


@router.get("/devices/{device_type}/{device_id}/status", tags=["Devices"])
async def get_device_status(device_type: str, device_id: str):
    """Check device status.

    - GET /devices/windows/HP_LaserJet/status
    - GET /devices/usb/0483:5743/status
    - GET /devices/network/192.168.1.100:9100/status
    """
    if device_type == "windows":
        return await asyncio.to_thread(_get_windows_printer_status, device_id)

    if device_type == "usb":
        return await _get_escpos_status(device_type, device_id, vid_pid=device_id)

    if device_type == "network":
        parts = device_id.split(":")
        host = parts[0]
        port = int(parts[1]) if len(parts) > 1 else 9100
        reachable = await asyncio.to_thread(NetworkAdapter.test_connection, host, port)
        return {
            "device_type": "network",
            "host": host,
            "port": port,
            "is_online": reachable,
            "status": "Reachable" if reachable else "Unreachable",
        }

    if device_type == "serial":
        port_name = device_id.replace("_", "/")
        available = await asyncio.to_thread(SerialAdapter.list_ports)
        is_online = port_name in available
        return {
            "device_type": "serial",
            "port": port_name,
            "is_online": is_online,
            "status": "Available" if is_online else "Not found",
        }

    raise HTTPException(status_code=400, detail=f"Unsupported device type: {device_type}")


@router.get("/devices/windows/{printer_name}/jobs/{job_id}", tags=["Devices"])
async def get_job_status(printer_name: str, job_id: int):
    """Get the status of a specific print job in a Windows printer queue.

    - GET /devices/windows/HP_LaserJet/jobs/123
    """
    if not WINDOWS_PRINTER_AVAILABLE:
        raise HTTPException(status_code=500, detail="Windows printer support not available")

    printer_name = printer_name.replace("_", " ")
    return await asyncio.to_thread(_get_windows_job_status, printer_name, job_id)


# --------------- Private helpers ---------------


def _scan_windows_printers() -> list[DeviceInfo]:
    if not WINDOWS_PRINTER_AVAILABLE:
        return []
    try:
        printer_enum = win32print.EnumPrinters(
            win32print.PRINTER_ENUM_LOCAL | win32print.PRINTER_ENUM_CONNECTIONS
        )
        devices = []
        for flags, description, name, comment in printer_enum:
            conn_type = "local" if flags & win32print.PRINTER_ENUM_LOCAL else "network"
            devices.append(DeviceInfo(
                id=f"win-{name.replace(' ', '_')}",
                type="windows",
                name=name,
                manufacturer=description,
                description=f"{conn_type} - {comment}" if comment else conn_type,
            ))
        logger.info(f"Found {len(devices)} Windows printers")
        return devices
    except Exception as e:
        logger.error(f"Windows printer scan error: {e}")
        return []


def _get_windows_printer_status(printer_name: str) -> dict:
    """Get Windows printer status via Spooler API."""
    if not WINDOWS_PRINTER_AVAILABLE:
        raise HTTPException(status_code=500, detail="Windows printer support not available")

    printer_name = printer_name.replace("_", " ")

    hPrinter = win32print.OpenPrinter(printer_name)
    try:
        printer_info = win32print.GetPrinter(hPrinter, 2)
        status_code = printer_info.get("Status", 0)
        errors, warnings, details = _decode_win_status(status_code)
        is_online = len(errors) == 0

        if errors:
            status_text = f"Error: {', '.join(errors)}"
        elif warnings:
            status_text = f"Warning: {', '.join(warnings)}"
        elif details:
            status_text = ", ".join(details)
        else:
            status_text = "Ready"

        try:
            jobs = win32print.EnumJobs(hPrinter, 0, -1, 1)
            jobs_count = len(jobs)
        except Exception:
            jobs_count = 0

        return {
            "device_type": "windows",
            "printer_name": printer_name,
            "is_online": is_online,
            "status": status_text,
            "status_code": status_code,
            "errors": errors,
            "warnings": warnings,
            "details": details,
            "jobs_in_queue": jobs_count,
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error checking Windows printer status: {e}")
        raise HTTPException(status_code=500, detail="Failed to get printer status")
    finally:
        win32print.ClosePrinter(hPrinter)


def _decode_win_status(status_code: int) -> tuple:
    """Decode Windows printer status bits into errors, warnings, details."""
    errors, warnings, details = [], [], []

    error_flags = {
        win32print.PRINTER_STATUS_OFFLINE: "Offline",
        win32print.PRINTER_STATUS_PAPER_JAM: "Paper jam",
        win32print.PRINTER_STATUS_PAPER_OUT: "Out of paper",
        win32print.PRINTER_STATUS_DOOR_OPEN: "Door open",
        win32print.PRINTER_STATUS_ERROR: "Printer error",
        win32print.PRINTER_STATUS_NOT_AVAILABLE: "Not available",
        win32print.PRINTER_STATUS_OUT_OF_MEMORY: "Out of memory",
        win32print.PRINTER_STATUS_NO_TONER: "No toner",
    }
    warning_flags = {
        win32print.PRINTER_STATUS_TONER_LOW: "Toner low",
        win32print.PRINTER_STATUS_PAGE_PUNT: "Page punt",
        win32print.PRINTER_STATUS_USER_INTERVENTION: "User intervention required",
        win32print.PRINTER_STATUS_PAPER_PROBLEM: "Paper problem",
        win32print.PRINTER_STATUS_PAUSED: "Paused",
        win32print.PRINTER_STATUS_PENDING_DELETION: "Pending deletion",
    }
    detail_flags = {
        win32print.PRINTER_STATUS_BUSY: "Busy",
        win32print.PRINTER_STATUS_PRINTING: "Printing",
        win32print.PRINTER_STATUS_WARMING_UP: "Warming up",
        win32print.PRINTER_STATUS_INITIALIZING: "Initializing",
        win32print.PRINTER_STATUS_WAITING: "Waiting",
        win32print.PRINTER_STATUS_PROCESSING: "Processing",
    }

    for flag, label in error_flags.items():
        if status_code & flag:
            errors.append(label)
    for flag, label in warning_flags.items():
        if status_code & flag:
            warnings.append(label)
    for flag, label in detail_flags.items():
        if status_code & flag:
            details.append(label)

    return errors, warnings, details


def _get_windows_job_status(printer_name: str, job_id: int) -> dict:
    """Get Windows print job status from the spooler."""
    if not WINDOWS_PRINTER_AVAILABLE:
        raise HTTPException(status_code=500, detail="Windows printer support not available")

    hPrinter = win32print.OpenPrinter(printer_name)
    try:
        job_info = win32print.GetJob(hPrinter, job_id, 1)

        # Decode status
        status_code = job_info.get("Status", 0)
        status_map = {
            0: "queued",
            1: "paused",
            2: "error",
            3: "deleting",
            4: "spooling",
            5: "printing",
            6: "printed",
        }
        status_str = status_map.get(status_code, f"unknown ({status_code})")

        return {
            "job_id": job_info.get("JobId"),
            "printer_name": printer_name,
            "document_name": job_info.get("pDocument"),
            "status_code": status_code,
            "status": status_str,
            "total_pages": job_info.get("TotalPages", 0),
            "pages_printed": job_info.get("PagesPrinted", 0),
            "submitted_time": str(job_info.get("SubmittedTime", "N/A")),
            "position_in_queue": job_info.get("Position", 0),
            "size_in_bytes": job_info.get("Size", 0),
        }
    except Exception as e:
        logger.error(f"Error getting job status: {e}")
        raise HTTPException(status_code=404, detail=f"Job {job_id} not found or error: {e}")
    finally:
        win32print.ClosePrinter(hPrinter)


async def _get_escpos_status(device_type: str, device_id: str, **kwargs) -> dict:
    """Query ESC/POS printer status via DLE EOT."""
    try:
        if device_type == "usb":
            vid_pid = kwargs["vid_pid"]
            parts = vid_pid.split(":")
            vid = int(parts[0], 16)
            pid = int(parts[1], 16)
            adapter = await asyncio.to_thread(create_adapter, "usb", vid=vid, pid=pid)
        else:
            return {
                "device_type": device_type,
                "is_online": False,
                "status": "Status check not supported for this type",
            }

        try:
            status_cmd = bytes([0x10, 0x04, 0x01])
            await asyncio.to_thread(adapter.write, status_cmd)
            await asyncio.sleep(0.1)
            response = await asyncio.to_thread(adapter.read, 1)

            if not response:
                raise RuntimeError("No response from printer")

            status_byte = response[0]
            errors, warnings, details = [], [], []

            if status_byte & (1 << 3):
                errors.append("Offline")
            else:
                details.append("Online")
            if status_byte & (1 << 5):
                warnings.append("Waiting for online recovery")
            if status_byte & (1 << 6):
                details.append("Paper feed button pressed")

            is_online = not (status_byte & (1 << 3))
            if errors:
                status_text = f"Error: {', '.join(errors)}"
            elif warnings:
                status_text = f"Warning: {', '.join(warnings)}"
            else:
                status_text = "Ready"

            return {
                "device_type": device_type,
                "is_online": is_online,
                "status": status_text,
                "status_byte": f"0x{status_byte:02X}",
                "errors": errors,
                "warnings": warnings,
                "details": details,
            }
        finally:
            await asyncio.to_thread(adapter.close)

    except Exception as e:
        logger.error(f"Error checking ESC/POS printer status: {e}")
        return {
            "device_type": device_type,
            "is_online": False,
            "status": "Cannot communicate",
            "errors": ["Communication error"],
            "warnings": [],
            "details": [],
        }
