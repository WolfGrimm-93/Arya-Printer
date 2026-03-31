"""
Tests para scanners de dispositivos
"""

import pytest
from unittest.mock import Mock, patch
from scanners.bluetooth_scanner import BluetoothScanner, PYBLUEZ_AVAILABLE


def test_bluetooth_scanner_initialization():
    """Test: Inicializar scanner Bluetooth"""
    scanner = BluetoothScanner(
        scan_duration=8,
        cache_ttl=300
    )
    
    assert scanner.scan_duration == 8
    assert scanner.cache_ttl == 300
    assert scanner._cache == []
    assert scanner._last_scan is None


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
@patch('adapters.bluetooth_adapter.BluetoothAdapter.find_printers')
def test_bluetooth_scanner_scan_mock(mock_find_printers, mock_bluetooth_device):
    """Test: Escaneo de Bluetooth con mock"""
    # Mock de dispositivos encontrados
    mock_find_printers.return_value = [mock_bluetooth_device]
    
    scanner = BluetoothScanner(scan_duration=1, cache_ttl=60)
    devices = scanner.scan()
    
    assert len(devices) >= 0
    mock_find_printers.assert_called_once()


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
@patch('adapters.bluetooth_adapter.BluetoothAdapter.find_printers')
def test_bluetooth_scanner_cache(mock_find_printers, mock_bluetooth_device):
    """Test: Caché de scanner Bluetooth"""
    mock_find_printers.return_value = [mock_bluetooth_device]
    
    scanner = BluetoothScanner(scan_duration=1, cache_ttl=300)
    
    # Primera llamada: escanea
    devices1 = scanner.scan()
    assert mock_find_printers.call_count == 1
    
    # Segunda llamada: usa caché
    devices2 = scanner.scan()
    assert mock_find_printers.call_count == 1  # No llamó de nuevo
    
    # Forzar escaneo
    devices3 = scanner.scan(force=True)
    assert mock_find_printers.call_count == 2  # Sí llamó


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
@patch('adapters.bluetooth_adapter.BluetoothAdapter.find_printers')
def test_bluetooth_scanner_find_by_address(mock_find_printers, mock_bluetooth_device):
    """Test: Buscar por dirección MAC"""
    mock_find_printers.return_value = [mock_bluetooth_device]
    
    scanner = BluetoothScanner(scan_duration=1)
    scanner.scan()
    
    # Buscar existente
    found = scanner.find_by_address("00:11:22:33:44:55")
    assert found is not None
    assert found['address'] == "00:11:22:33:44:55"
    
    # Buscar inexistente
    not_found = scanner.find_by_address("FF:FF:FF:FF:FF:FF")
    assert not_found is None


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
@patch('adapters.bluetooth_adapter.BluetoothAdapter.find_printers')
def test_bluetooth_scanner_find_by_name(mock_find_printers, mock_bluetooth_device):
    """Test: Buscar por nombre"""
    mock_find_printers.return_value = [mock_bluetooth_device]
    
    scanner = BluetoothScanner(scan_duration=1)
    scanner.scan()
    
    # Buscar exacto
    found = scanner.find_by_name("POS-58", partial=False)
    assert len(found) == 1
    assert found[0]['name'] == "POS-58"
    
    # Buscar parcial
    found_partial = scanner.find_by_name("POS", partial=True)
    assert len(found_partial) >= 1


def test_bluetooth_scanner_clear_cache():
    """Test: Limpiar caché"""
    scanner = BluetoothScanner()
    scanner._cache = [{"address": "00:11:22:33:44:55"}]
    scanner._last_scan = 123456789
    
    scanner.clear_cache()
    
    assert scanner._cache == []
    assert scanner._last_scan is None


def test_bluetooth_scanner_get_cached_devices():
    """Test: Obtener dispositivos del caché"""
    scanner = BluetoothScanner()
    scanner._cache = [
        {"address": "00:11:22:33:44:55", "name": "Printer 1"},
        {"address": "AA:BB:CC:DD:EE:FF", "name": "Printer 2"}
    ]
    
    cached = scanner.get_cached_devices()
    
    assert len(cached) == 2
    assert cached[0]['name'] == "Printer 1"
    
    # Verificar que devuelve el caché (no una copia en este caso)
    cached[0]['name'] = "Modified"
    assert scanner._cache[0]['name'] == "Modified"  # El caché se modifica porque es la misma referencia
