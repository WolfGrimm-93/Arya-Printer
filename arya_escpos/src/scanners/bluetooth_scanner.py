"""
Bluetooth Device Scanner
Escaneo periódico de dispositivos Bluetooth Classic

⚠️ ESTADO: PENDIENTE DE IMPLEMENTACIÓN
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PROBLEMA: PyBluez no es compatible con Python 3.12+ en Windows

ALTERNATIVA CON BLEAK (BLE Scanner):
    
    from bleak import BleakScanner
    
    async def scan_ble_devices():
        devices = await BleakScanner.discover(timeout=5.0)
        return [{
            'address': d.address,
            'name': d.name or 'Unknown',
            'rssi': d.rssi
        } for d in devices]

Ver: https://bleak.readthedocs.io/
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
"""

import time
from typing import List, Dict, Any, Optional
from adapters.bluetooth_adapter import BluetoothAdapter, PYBLUEZ_AVAILABLE
from utils.logger import logger


class BluetoothScanner:
    """
    Escáner de dispositivos Bluetooth
    
    Proporciona escaneo periódico y caché de dispositivos descubiertos
    Similar al DeviceINQ de node-bluetooth
    """
    
    def __init__(self, scan_duration: int = 8, cache_ttl: int = 300):
        """
        Args:
            scan_duration: Duración de cada escaneo en segundos
            cache_ttl: Tiempo de vida del caché en segundos
        """
        self.scan_duration = scan_duration
        self.cache_ttl = cache_ttl
        self._cache: List[Dict[str, Any]] = []
        self._last_scan: Optional[float] = None
    
    def scan(self, force: bool = False) -> List[Dict[str, Any]]:
        """
        Escanea dispositivos Bluetooth
        
        Args:
            force: Si es True, fuerza nuevo escaneo ignorando caché
            
        Returns:
            Lista de dispositivos encontrados
        """
        if not PYBLUEZ_AVAILABLE:
            logger.warning("PyBluez no disponible, retornando caché vacío")
            return []
        
        # Verificar caché
        current_time = time.time()
        if not force and self._cache and self._last_scan:
            age = current_time - self._last_scan
            if age < self.cache_ttl:
                logger.debug(f"Usando caché Bluetooth ({age:.0f}s de edad)")
                return self._cache
        
        # Escanear dispositivos
        logger.info("Iniciando escaneo Bluetooth...")
        self._cache = BluetoothAdapter.find_printers(duration=self.scan_duration)
        self._last_scan = current_time
        
        return self._cache
    
    def find_by_address(self, address: str) -> Optional[Dict[str, Any]]:
        """
        Busca dispositivo por dirección MAC
        
        Args:
            address: Dirección MAC (ej: "00:11:22:33:44:55")
            
        Returns:
            Info del dispositivo o None
        """
        devices = self.scan()
        
        for device in devices:
            if device['address'].upper() == address.upper():
                return device
        
        return None
    
    def find_by_name(self, name: str, partial: bool = True) -> List[Dict[str, Any]]:
        """
        Busca dispositivos por nombre
        
        Args:
            name: Nombre a buscar
            partial: Si es True, permite coincidencias parciales
            
        Returns:
            Lista de dispositivos que coinciden
        """
        devices = self.scan()
        results = []
        
        search_name = name.lower()
        
        for device in devices:
            device_name = device.get('name', '').lower()
            
            if partial:
                if search_name in device_name:
                    results.append(device)
            else:
                if search_name == device_name:
                    results.append(device)
        
        return results
    
    def clear_cache(self) -> None:
        """Limpia el caché de dispositivos"""
        self._cache.clear()
        self._last_scan = None
        logger.debug("Caché Bluetooth limpiado")
    
    def get_cached_devices(self) -> List[Dict[str, Any]]:
        """
        Retorna dispositivos del caché sin escanear
        
        Returns:
            Lista de dispositivos en caché
        """
        return self._cache.copy()
