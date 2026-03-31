"""
Device Manager - Gestión centralizada de dispositivos
Inspirado en QZ-Tray's NativePrinterMap
"""
from typing import Dict, List, Optional
from datetime import datetime
from sqlalchemy.orm import Session

from database.models import Device
from database.database import get_database
from utils.logger import get_logger
from utils.exceptions import DeviceNotFoundError, DeviceError
from core.base_adapter import BaseAdapter


class DeviceManager:
    """
    Gestor centralizado de dispositivos
    Mantiene cache de dispositivos detectados y sincroniza con BD
    """
    
    def __init__(self):
        """Initialize device manager"""
        self.logger = get_logger()
        self._devices: Dict[str, Dict] = {}  # In-memory cache
        self._adapters: Dict[str, BaseAdapter] = {}  # Active adapter instances
    
    def register_device(
        self,
        device_id: str,
        device_type: str,
        **metadata
    ) -> Device:
        """
        Register a new device or update existing
        
        Args:
            device_id: Unique device identifier
            device_type: Device type (usb, network, serial, bluetooth)
            **metadata: Additional device metadata
            
        Returns:
            Device model instance
        """
        db = get_database()
        
        with db.get_session() as session:
            # Check if device exists
            device = session.query(Device).filter(Device.id == device_id).first()
            
            if device:
                # Update existing device
                device.last_seen = datetime.utcnow()
                device.is_active = True
                for key, value in metadata.items():
                    if hasattr(device, key):
                        setattr(device, key, value)
                self.logger.debug(f"Updated device: {device_id}")
            else:
                # Create new device
                device = Device(
                    id=device_id,
                    type=device_type,
                    first_seen=datetime.utcnow(),
                    last_seen=datetime.utcnow(),
                    is_active=True,
                    **metadata
                )
                session.add(device)
                self.logger.info(f"Registered new device: {device_id}")
            
            session.commit()
            session.refresh(device)
            
            # Update in-memory cache
            self._devices[device_id] = {
                "id": device.id,
                "type": device.type,
                "vid": device.vid,
                "pid": device.pid,
                "serial_number": device.serial_number,
                "manufacturer": device.manufacturer,
                "product": device.product,
                "alias": device.alias,
                "last_port": device.last_port,
                "is_active": device.is_active,
                "last_seen": device.last_seen,
            }
            
            return device
    
    def unregister_device(self, device_id: str) -> bool:
        """
        Mark device as inactive
        
        Args:
            device_id: Device identifier
            
        Returns:
            True if successful
        """
        db = get_database()
        
        with db.get_session() as session:
            device = session.query(Device).filter(Device.id == device_id).first()
            
            if device:
                device.is_active = False
                device.last_seen = datetime.utcnow()
                session.commit()
                
                # Remove from cache
                if device_id in self._devices:
                    del self._devices[device_id]
                
                # Close adapter if exists
                if device_id in self._adapters:
                    self._adapters[device_id].close()
                    del self._adapters[device_id]
                
                self.logger.info(f"Unregistered device: {device_id}")
                return True
        
        return False
    
    def get_device(self, device_id: str) -> Optional[Dict]:
        """
        Get device information
        
        Args:
            device_id: Device identifier
            
        Returns:
            Device info dict or None
        """
        # Check cache first
        if device_id in self._devices:
            return self._devices[device_id]
        
        # Query database
        db = get_database()
        with db.get_session() as session:
            device = session.query(Device).filter(Device.id == device_id).first()
            if device:
                device_dict = {
                    "id": device.id,
                    "type": device.type,
                    "vid": device.vid,
                    "pid": device.pid,
                    "serial_number": device.serial_number,
                    "manufacturer": device.manufacturer,
                    "product": device.product,
                    "alias": device.alias,
                    "last_port": device.last_port,
                    "is_active": device.is_active,
                    "last_seen": device.last_seen,
                }
                
                # Update cache
                if device.is_active:
                    self._devices[device_id] = device_dict
                
                return device_dict
        
        return None
    
    def get_all_devices(self, active_only: bool = True) -> List[Dict]:
        """
        Get all devices
        
        Args:
            active_only: Return only active devices
            
        Returns:
            List of device info dicts
        """
        db = get_database()
        with db.get_session() as session:
            query = session.query(Device)
            if active_only:
                query = query.filter(Device.is_active == True)
            
            devices = query.all()
            
            return [
                {
                    "id": d.id,
                    "type": d.type,
                    "vid": d.vid,
                    "pid": d.pid,
                    "serial_number": d.serial_number,
                    "manufacturer": d.manufacturer,
                    "product": d.product,
                    "alias": d.alias,
                    "last_port": d.last_port,
                    "is_active": d.is_active,
                    "last_seen": d.last_seen,
                }
                for d in devices
            ]
    
    def register_adapter(self, device_id: str, adapter: BaseAdapter):
        """
        Register an active adapter instance
        
        Args:
            device_id: Device identifier
            adapter: Adapter instance
        """
        self._adapters[device_id] = adapter
        self.logger.debug(f"Registered adapter for device: {device_id}")
    
    def get_adapter(self, device_id: str) -> Optional[BaseAdapter]:
        """
        Get active adapter for device
        
        Args:
            device_id: Device identifier
            
        Returns:
            Adapter instance or None
        """
        return self._adapters.get(device_id)
    
    def clear_cache(self):
        """Clear in-memory cache"""
        self._devices.clear()
        self.logger.info("Device cache cleared")
    
    def sync_from_database(self):
        """Synchronize cache from database"""
        devices = self.get_all_devices(active_only=True)
        self._devices = {d["id"]: d for d in devices}
        self.logger.info(f"Synchronized {len(devices)} devices from database")


# Global device manager instance
_device_manager: Optional[DeviceManager] = None


def get_device_manager() -> DeviceManager:
    """Get global device manager instance"""
    global _device_manager
    if _device_manager is None:
        _device_manager = DeviceManager()
    return _device_manager
