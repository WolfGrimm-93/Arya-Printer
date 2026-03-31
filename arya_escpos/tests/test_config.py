"""
Tests para el sistema de configuración
"""

import pytest
from pathlib import Path


def test_settings_creation():
    """Test: Crear config con valores por defecto"""
    from utils.config import Config
    
    config = Config()
    
    assert config.server.host == "0.0.0.0"
    assert config.server.port == 58181
    assert config.database.url.startswith("sqlite:///")


def test_config_from_yaml():
    """Test: Cargar config desde YAML"""
    from utils.config import Config
    
    config_path = Path(__file__).parent.parent / "config" / "settings.yaml"
    
    if config_path.exists():
        config = Config.from_yaml(str(config_path))
        assert config is not None
        assert config.server.port > 0
        assert config.discovery.usb_enabled is not None
    else:
        pytest.skip("settings.yaml no existe")


def test_discovery_settings(test_settings):
    """Test: Configuración de discovery"""
    assert test_settings["discovery"]["usb_enabled"] is True
    assert test_settings["discovery"]["network_enabled"] is True
    assert test_settings["discovery"]["serial_enabled"] is True
    assert test_settings["discovery"]["bluetooth_enabled"] is True


def test_config_file_exists():
    """Test: settings.yaml existe"""
    config_path = Path(__file__).parent.parent / "config" / "settings.yaml"
    assert config_path.exists(), "settings.yaml debe existir"


def test_bluetooth_scan_duration(test_settings):
    """Test: Configuración de escaneo Bluetooth"""
    duration = test_settings["discovery"]["bluetooth_scan_duration"]
    assert isinstance(duration, int)
    assert duration > 0
    assert duration <= 30
