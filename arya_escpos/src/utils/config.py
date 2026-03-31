"""
Configuration management for Arya ESCPOS
Loads settings from YAML file and environment variables
"""
from pathlib import Path
from typing import Optional, Dict, Any, List
import yaml
from pydantic import Field
from pydantic_settings import BaseSettings
from .exceptions import ConfigurationError


class ServerConfig(BaseSettings):
    """Server configuration"""
    host: str = Field(default="0.0.0.0")
    port: int = Field(default=58181)
    port_secure: int = Field(default=58182)
    reload: bool = Field(default=False)
    workers: int = Field(default=1)



class LoggingConfig(BaseSettings):
    """Logging configuration"""
    level: str = Field(default="INFO")
    format: str = Field(default="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | <level>{level: <8}</level> | <cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> - <level>{message}</level>")
    log_dir: str = Field(default="logs")
    retention_days: int = Field(default=15)


class DiscoveryConfig(BaseSettings):
    """Device discovery configuration"""
    auto_scan_interval: int = Field(default=30)
    usb_enabled: bool = Field(default=True)
    network_enabled: bool = Field(default=True)
    serial_enabled: bool = Field(default=True)
    bluetooth_enabled: bool = Field(default=False)
    bluetooth_scan_duration: int = Field(default=8)  # Bluetooth scan duration in seconds


class DeviceConfig(BaseSettings):
    """Device connection configuration"""
    auto_reconnect: bool = Field(default=True)
    connection_timeout: int = Field(default=5)
    retry_attempts: int = Field(default=3)


class PrinterConfig(BaseSettings):
    """Printer configuration"""
    default_encoding: str = Field(default="utf-8")
    paper_width: int = Field(default=80)


class NetworkScanConfig(BaseSettings):
    """Network scanning configuration"""
    enabled: bool = Field(default=True)
    subnets: List[str] = Field(default=["192.168.1.0/24"])
    ports: List[int] = Field(default=[9100, 9101])
    timeout: int = Field(default=2)


class Config(BaseSettings):
    """Main configuration class"""
    server: ServerConfig = Field(default_factory=ServerConfig)
    logging: LoggingConfig = Field(default_factory=LoggingConfig)
    discovery: DiscoveryConfig = Field(default_factory=DiscoveryConfig)
    devices: DeviceConfig = Field(default_factory=DeviceConfig)
    printers: PrinterConfig = Field(default_factory=PrinterConfig)
    network_scan: NetworkScanConfig = Field(default_factory=NetworkScanConfig)
    
    @classmethod
    def from_yaml(cls, config_path: str = "config/settings.yaml") -> "Config":
        """
        Load configuration from YAML file
        
        Args:
            config_path: Path to YAML configuration file
            
        Returns:
            Config instance
            
        Raises:
            ConfigurationError: If file not found or invalid
        """
        path = Path(config_path)
        
        if not path.exists():
            raise ConfigurationError(f"Configuration file not found: {config_path}")
        
        try:
            with open(path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f)
            
            # Convert nested dicts to config objects
            config_data = {}
            for key, value in data.items():
                if isinstance(value, dict):
                    # Map to the appropriate config class
                    config_class = {
                        "server": ServerConfig,
                        "logging": LoggingConfig,
                        "discovery": DiscoveryConfig,
                        "devices": DeviceConfig,
                        "printers": PrinterConfig,
                        "network_scan": NetworkScanConfig,
                    }.get(key)
                    
                    if config_class:
                        config_data[key] = config_class(**value)
                else:
                    config_data[key] = value
            
            return cls(**config_data)
            
        except yaml.YAMLError as e:
            raise ConfigurationError(f"Invalid YAML configuration: {e}")
        except Exception as e:
            raise ConfigurationError(f"Error loading configuration: {e}")
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert configuration to dictionary"""
        return {
            "server": self.server.model_dump(),
            "logging": self.logging.model_dump(),
            "discovery": self.discovery.model_dump(),
            "devices": self.devices.model_dump(),
            "printers": self.printers.model_dump(),
            "network_scan": self.network_scan.model_dump(),
        }


# Global config instance
_config_instance: Optional[Config] = None


def init_config(config_path: str = "config/settings.yaml") -> Config:
    """
    Initialize global configuration
    
    Args:
        config_path: Path to YAML configuration file
        
    Returns:
        Config instance
    """
    global _config_instance
    _config_instance = Config.from_yaml(config_path)
    return _config_instance


def get_config() -> Config:
    """
    Get global configuration instance
    
    Returns:
        Config instance
        
    Raises:
        RuntimeError: If config not initialized
    """
    if _config_instance is None:
        raise RuntimeError("Configuration not initialized. Call init_config() first.")
    return _config_instance
