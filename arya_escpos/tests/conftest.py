"""
Pytest Configuration and Shared Fixtures
Configuración global de tests y fixtures compartidos
"""

import pytest
import sys
from pathlib import Path
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient

# Agregar src al path
ROOT_DIR = Path(__file__).parent.parent
sys.path.insert(0, str(ROOT_DIR / "src"))


@pytest.fixture(scope="session")
def test_settings():
    """Settings de prueba simplificados"""
    return {
        "server": {"host": "127.0.0.1", "port": 58181},
        "database": {"url": "sqlite:///:memory:"},
        "cache": {"enabled": False},
        "logging": {"level": "DEBUG"},
        "discovery": {
            "usb_enabled": True,
            "network_enabled": True,
            "serial_enabled": True,
            "bluetooth_enabled": True,
            "bluetooth_scan_duration": 8
        }
    }


@pytest.fixture(scope="function")
def db_session():
    """
    Base de datos en memoria para cada test
    Se crea nueva para cada test y se destruye al terminar
    """
    from database.models import Base
    
    # Crear engine en memoria - usar modo shared cache para evitar problemas
    engine = create_engine(
        "sqlite:///:memory:",
        connect_args={"check_same_thread": False},
        poolclass=None  # Disable pooling for in-memory
    )
    
    # Crear todas las tablas
    Base.metadata.create_all(bind=engine)
    
    # Crear session
    TestingSessionLocal = sessionmaker(
        autocommit=False,
        autoflush=False,
        bind=engine
    )
    
    session = TestingSessionLocal()
    
    try:
        # Guardar el engine para que el client lo use
        session._test_engine = engine
        yield session
    finally:
        session.close()
        Base.metadata.drop_all(bind=engine)


@pytest.fixture(scope="function")
def client(db_session, test_settings):
    """
    Cliente de prueba para FastAPI
    """
    # Inicializar configuración para tests
    from utils.config import Config
    import utils.config as config_module
    
    # Crear configuración de prueba
    test_config = Config(
        server=test_settings["server"],
        database=test_settings["database"],
        cache=test_settings["cache"],
        logging=test_settings["logging"],
        discovery=test_settings["discovery"]
    )
    
    # Inyectar configuración en el módulo
    config_module._config_instance = test_config
    
    # Crear app
    from server import routes
    from fastapi import FastAPI
    
    app = FastAPI()
    app.include_router(routes.router, prefix="/api/v1")
    
    # Override database dependency
    def override_get_db():
        try:
            yield db_session
        finally:
            pass
    
    # Buscar dependency en routes si existe
    if hasattr(routes, 'get_db'):
        app.dependency_overrides[routes.get_db] = override_get_db
    
    with TestClient(app) as test_client:
        yield test_client
    
    # Limpiar configuración después del test
    config_module._config_instance = None


@pytest.fixture
def mock_usb_device():
    """Mock de dispositivo USB"""
    return {
        "vid": 0x0416,
        "pid": 0x5011,
        "manufacturer": "Test Printer Inc",
        "product": "Thermal Printer 58mm",
        "serial": "TEST123456",
        "interface": 0,
        "in_ep": 0x81,
        "out_ep": 0x02
    }


@pytest.fixture
def mock_bluetooth_device():
    """Mock de dispositivo Bluetooth"""
    return {
        "address": "00:11:22:33:44:55",
        "name": "POS-58",
        "channel": 1,
        "type": "bluetooth"
    }


@pytest.fixture
def mock_network_printer():
    """Mock de impresora de red"""
    return {
        "ip": "192.168.1.100",
        "port": 9100,
        "hostname": "printer-pos",
        "type": "network"
    }


@pytest.fixture
def sample_escpos_commands():
    """Comandos ESC/POS de ejemplo para testing"""
    return {
        "initialize": b"\x1b@",
        "cut": b"\x1d\x56\x00",
        "center": b"\x1b\x61\x01",
        "left": b"\x1b\x61\x00",
        "bold_on": b"\x1b\x45\x01",
        "bold_off": b"\x1b\x45\x00",
        "text": b"Test Print\n",
    }
