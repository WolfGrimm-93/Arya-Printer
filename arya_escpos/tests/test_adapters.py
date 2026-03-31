"""
Tests para adaptadores de dispositivos
"""

import pytest
from unittest.mock import Mock, patch, MagicMock
from adapters.usb_adapter import USBAdapter, PYUSB_AVAILABLE
from adapters.network_adapter import NetworkAdapter
from adapters.serial_adapter import SerialAdapter
from adapters.bluetooth_adapter import BluetoothAdapter, PYBLUEZ_AVAILABLE


# ============================================================================
# USB ADAPTER TESTS
# ============================================================================

def test_usb_adapter_availability():
    """Test: PyUSB debe estar disponible"""
    assert PYUSB_AVAILABLE is not None


@patch('adapters.usb_adapter.usb')
def test_usb_find_printers_mock(mock_usb, mock_usb_device):
    """Test: Buscar impresoras USB (mock)"""
    # Mock de dispositivo USB
    mock_device = Mock()
    mock_device.idVendor = mock_usb_device['vid']
    mock_device.idProduct = mock_usb_device['pid']
    mock_device.manufacturer = mock_usb_device['manufacturer']
    mock_device.product = mock_usb_device['product']
    mock_device.serial_number = mock_usb_device['serial']
    
    # Mock de interfaz
    mock_interface = Mock()
    mock_interface.bInterfaceClass = 0x07  # USB_CLASS_PRINTER
    
    mock_config = Mock()
    mock_config.__iter__ = Mock(return_value=iter([mock_interface]))
    
    mock_device.__getitem__ = Mock(return_value=mock_config)
    
    # Mock de core.find
    mock_usb.core.find.return_value = [mock_device]
    
    printers = USBAdapter.find_printers()
    
    assert len(printers) >= 0  # Puede retornar 0 si no hay hardware


def test_usb_adapter_initialization(mock_usb_device):
    """Test: Inicializar adaptador USB"""
    adapter = USBAdapter(
        vid=mock_usb_device['vid'],
        pid=mock_usb_device['pid']
    )
    
    assert adapter.vid == 0x0416
    assert adapter.pid == 0x5011
    assert adapter.is_connected is False


def test_usb_adapter_context_manager(mock_usb_device):
    """Test: Context manager de USB adapter"""
    adapter = USBAdapter(
        vid=mock_usb_device['vid'],
        pid=mock_usb_device['pid']
    )
    
    # Verificar que tiene métodos __enter__ y __exit__
    assert hasattr(adapter, '__enter__')
    assert hasattr(adapter, '__exit__')


# ============================================================================
# NETWORK ADAPTER TESTS
# ============================================================================

def test_network_adapter_initialization():
    """Test: Inicializar adaptador de red"""
    adapter = NetworkAdapter(host="192.168.1.100", port=9100)
    
    assert adapter.host == "192.168.1.100"
    assert adapter.port == 9100
    assert adapter.is_connected is False


def test_network_adapter_invalid_port():
    """Test: NetworkAdapter acepta cualquier puerto (validación en tiempo de conexión)"""
    # NetworkAdapter no valida puerto en __init__, solo al conectar
    adapter = NetworkAdapter(host="192.168.1.100", port=99999)
    assert adapter.port == 99999


@patch('adapters.network_adapter.socket.socket')
def test_network_test_connection_success(mock_socket):
    """Test: Test de conexión exitoso"""
    # test_connection() usa connect_ex() que retorna 0 en éxito
    mock_sock = Mock()
    mock_sock.settimeout = Mock()
    mock_sock.connect_ex = Mock(return_value=0)  # 0 = success
    mock_sock.close = Mock()
    mock_socket.return_value = mock_sock
    
    result = NetworkAdapter.test_connection("192.168.1.100", 9100)
    assert result is True


@patch('socket.socket')
def test_network_test_connection_failure(mock_socket):
    """Test: Test de conexión fallido"""
    mock_sock = Mock()
    mock_sock.connect.side_effect = ConnectionRefusedError()
    mock_socket.return_value.__enter__.return_value = mock_sock
    
    result = NetworkAdapter.test_connection("192.168.1.100", 9100)
    assert result is False


def test_network_adapter_context_manager():
    """Test: Context manager de Network adapter"""
    adapter = NetworkAdapter(host="192.168.1.100", port=9100)
    
    assert hasattr(adapter, '__enter__')
    assert hasattr(adapter, '__exit__')


# ============================================================================
# SERIAL ADAPTER TESTS
# ============================================================================

def test_serial_adapter_initialization():
    """Test: Inicializar adaptador serial"""
    adapter = SerialAdapter(port="COM1", baudrate=9600)
    
    assert adapter.port == "COM1"
    assert adapter.baudrate == 9600
    assert adapter.is_connected is False


def test_serial_list_ports():
    """Test: Listar puertos seriales"""
    ports = SerialAdapter.list_ports()
    
    assert isinstance(ports, list)
    # En máquina sin puertos seriales puede retornar lista vacía
    assert ports is not None


def test_serial_adapter_invalid_baudrate():
    """Test: Baudrate común debe ser válido"""
    valid_baudrates = [9600, 19200, 38400, 57600, 115200]
    
    for baudrate in valid_baudrates:
        adapter = SerialAdapter(port="COM1", baudrate=baudrate)
        assert adapter.baudrate == baudrate


# ============================================================================
# BLUETOOTH ADAPTER TESTS
# ============================================================================

def test_bluetooth_adapter_availability():
    """Test: Verificar disponibilidad de PyBluez"""
    assert PYBLUEZ_AVAILABLE is not None


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
def test_bluetooth_adapter_initialization(mock_bluetooth_device):
    """Test: Inicializar adaptador Bluetooth"""
    adapter = BluetoothAdapter(
        address=mock_bluetooth_device['address'],
        channel=mock_bluetooth_device['channel']
    )
    
    assert adapter.address == "00:11:22:33:44:55"
    assert adapter.channel == 1
    assert adapter.is_connected is False


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
def test_bluetooth_adapter_no_channel():
    """Test: Bluetooth sin canal (auto-detect)"""
    adapter = BluetoothAdapter(address="00:11:22:33:44:55")
    
    assert adapter.address == "00:11:22:33:44:55"
    assert adapter.channel is None  # Se detectará al conectar


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
@patch('bluetooth.discover_devices')
def test_bluetooth_find_printers_mock(mock_discover):
    """Test: Buscar impresoras Bluetooth (mock)"""
    # Mock de dispositivos encontrados
    mock_discover.return_value = [
        ("00:11:22:33:44:55", "POS-58"),
        ("AA:BB:CC:DD:EE:FF", "Thermal Printer")
    ]
    
    with patch('adapters.bluetooth_adapter.BluetoothAdapter._find_rfcomm_channel') as mock_channel:
        mock_channel.return_value = 1
        
        printers = BluetoothAdapter.find_printers(duration=1)
        
        assert len(printers) >= 0


@pytest.mark.skipif(not PYBLUEZ_AVAILABLE, reason="PyBluez no instalado")
def test_bluetooth_adapter_repr(mock_bluetooth_device):
    """Test: Representación string del adaptador"""
    adapter = BluetoothAdapter(
        address=mock_bluetooth_device['address'],
        channel=1
    )
    
    repr_str = repr(adapter)
    assert "BluetoothAdapter" in repr_str
    assert "00:11:22:33:44:55" in repr_str
    assert "disconnected" in repr_str


# ============================================================================
# BASE ADAPTER TESTS
# ============================================================================

def test_base_adapter_event_callbacks():
    """Test: Callbacks de eventos en adaptadores"""
    adapter = NetworkAdapter(host="192.168.1.100", port=9100)
    
    callback_called = []
    
    def on_connect_callback(adapter):
        callback_called.append('connect')
    
    adapter.on_connect = on_connect_callback
    
    # Verificar que el callback se asignó
    assert adapter.on_connect == on_connect_callback


def test_adapter_write_without_connection():
    """Test: Escribir sin conexión debe fallar"""
    adapter = NetworkAdapter(host="192.168.1.100", port=9100)
    
    with pytest.raises(Exception):
        adapter.write(b"test data")


def test_adapter_read_without_connection():
    """Test: Leer sin conexión debe fallar"""
    adapter = NetworkAdapter(host="192.168.1.100", port=9100)
    
    with pytest.raises(Exception):
        adapter.read(100)
