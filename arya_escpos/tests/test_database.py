"""
Tests para modelos de base de datos
"""

import pytest
from datetime import datetime
from database.models import Device, Configuration, NetworkDevice, PrintHistory


def test_create_device(db_session):
    """Test: Crear dispositivo en BD"""
    device = Device(
        id="usb-test-001",
        type="usb",
        vid=0x0416,
        pid=0x5011,
        manufacturer="Test Printer",
        product="Thermal 58mm",
        is_active=True
    )
    
    db_session.add(device)
    db_session.commit()
    
    # Verificar que se creó
    saved = db_session.query(Device).filter_by(id="usb-test-001").first()
    assert saved is not None
    assert saved.type == "usb"
    assert saved.manufacturer == "Test Printer"
    assert saved.is_active is True


def test_device_timestamps(db_session):
    """Test: Timestamps automáticos"""
    device = Device(
        id="test-timestamps",
        type="usb",
        is_active=True
    )
    
    db_session.add(device)
    db_session.commit()
    
    saved = db_session.query(Device).filter_by(id="test-timestamps").first()
    assert saved.first_seen is not None
    assert saved.last_seen is not None
    assert isinstance(saved.first_seen, datetime)


def test_device_unique_id(db_session):
    """Test: ID de dispositivo debe ser único"""
    device1 = Device(id="duplicate-id", type="usb", is_active=True)
    device2 = Device(id="duplicate-id", type="usb", is_active=True)
    
    db_session.add(device1)
    db_session.commit()
    
    db_session.add(device2)
    
    with pytest.raises(Exception):  # IntegrityError
        db_session.commit()


def test_create_bluetooth_device(db_session, mock_bluetooth_device):
    """Test: Crear dispositivo Bluetooth"""
    device = Device(
        id=f"bt-{mock_bluetooth_device['address']}",
        type="bluetooth",
        serial_number=mock_bluetooth_device['address'],
        product=mock_bluetooth_device['name'],
        is_active=True
    )
    
    db_session.add(device)
    db_session.commit()
    
    saved = db_session.query(Device).filter_by(type="bluetooth").first()
    assert saved is not None
    assert saved.serial_number == "00:11:22:33:44:55"
    assert saved.product == "POS-58"


def test_create_network_device(db_session):
    """Test: Crear dispositivo de red"""
    network = NetworkDevice(
        id="net-192.168.1.100:9100",
        ip="192.168.1.100",
        port=9100,
        name="printer-01",
        is_reachable=True
    )
    
    db_session.add(network)
    db_session.commit()
    
    saved = db_session.query(NetworkDevice).filter_by(ip="192.168.1.100").first()
    assert saved is not None
    assert saved.port == 9100
    assert saved.is_reachable is True


def test_create_configuration(db_session):
    """Test: Guardar configuración"""
    config = Configuration(
        key="default_printer",
        value="usb-test-001",
        description="Default printer ID"
    )
    
    db_session.add(config)
    db_session.commit()
    
    saved = db_session.query(Configuration).filter_by(key="default_printer").first()
    assert saved is not None
    assert saved.value == "usb-test-001"


def test_create_print_history(db_session):
    """Test: Guardar historial de impresión"""
    device = Device(id="printer-001", type="usb", is_active=True)
    db_session.add(device)
    db_session.commit()
    
    history = PrintHistory(
        device_id="printer-001",
        document_type="receipt",
        status="success",
        error_message=None
    )
    
    db_session.add(history)
    db_session.commit()
    
    saved = db_session.query(PrintHistory).filter_by(device_id="printer-001").first()
    assert saved is not None
    assert saved.status == "success"
    assert saved.document_type == "receipt"


def test_query_active_devices(db_session):
    """Test: Consultar solo dispositivos activos"""
    # Crear dispositivos
    device1 = Device(id="active-1", type="usb", is_active=True)
    device2 = Device(id="active-2", type="usb", is_active=True)
    device3 = Device(id="inactive-1", type="usb", is_active=False)
    
    db_session.add_all([device1, device2, device3])
    db_session.commit()
    
    # Consultar solo activos
    active = db_session.query(Device).filter_by(is_active=True).all()
    assert len(active) == 2
    
    all_devices = db_session.query(Device).all()
    assert len(all_devices) == 3


def test_device_alias(db_session):
    """Test: Alias de dispositivo"""
    device = Device(
        id="aliased-device",
        type="usb",
        product="Generic Printer",
        alias="Impresora Principal",
        is_active=True
    )
    
    db_session.add(device)
    db_session.commit()
    
    saved = db_session.query(Device).filter_by(id="aliased-device").first()
    assert saved.alias == "Impresora Principal"
