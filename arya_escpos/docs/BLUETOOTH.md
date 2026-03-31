# 📘 Guía de Bluetooth para Arya ESCPOS

## 🔵 Tecnología Utilizada

**PyBluez** - Bluetooth Classic/RFCOMM (equivalente a `node-bluetooth`)

### ¿Por qué PyBluez?

✅ **Bluetooth Classic/RFCOMM** es el estándar en impresoras POS  
✅ Compatible con el **95%** de impresoras térmicas Bluetooth  
✅ Protocolo simple: RFCOMM es como "**serial inalámbrico**"  
✅ Mismo enfoque que **node-escpos** (node-bluetooth)  
✅ Probado y estable para impresoras POS  

---

## 📦 Instalación de PyBluez

### Windows

**Requisitos:**
- Microsoft C++ Build Tools (para compilar extensiones nativas)
- Bluetooth habilitado en el sistema

```powershell
# Instalar Microsoft C++ Build Tools
# Descargar: https://visualstudio.microsoft.com/visual-cpp-build-tools/

# Instalar PyBluez
pip install pybluez
```

**Alternativa (si falla la compilación):**
```powershell
# Usar wheel precompilado
pip install pybluez-win10
```

### Linux (Ubuntu/Debian)

```bash
# Instalar dependencias del sistema
sudo apt-get update
sudo apt-get install -y libbluetooth-dev

# Instalar PyBluez
pip install pybluez
```

### macOS

```bash
# PyBluez tiene problemas en macOS moderno
# Se recomienda usar Bleak (BLE) en su lugar
# O ejecutar en Linux/Windows para Bluetooth Classic
```

---

## 🔍 Uso del Adaptador Bluetooth

### 1. Escanear impresoras Bluetooth

```python
from src.adapters.bluetooth_adapter import BluetoothAdapter

# Escanear dispositivos (duración: 8 segundos)
printers = BluetoothAdapter.find_printers(duration=8)

for printer in printers:
    print(f"Impresora: {printer['name']}")
    print(f"  Dirección: {printer['address']}")
    print(f"  Canal RFCOMM: {printer['channel']}")
    print()

# Salida ejemplo:
# Impresora: POS-58
#   Dirección: 00:11:22:33:44:55
#   Canal RFCOMM: 1
```

### 2. Conectar a una impresora

```python
# Forma 1: Auto-conectar (detecta canal automáticamente)
adapter = BluetoothAdapter.get_device(
    address="00:11:22:33:44:55"
)

# Forma 2: Especificar canal manualmente
adapter = BluetoothAdapter(
    address="00:11:22:33:44:55",
    channel=1
)
adapter.open()

# Imprimir
adapter.write(b"\x1b@")  # Inicializar impresora
adapter.write(b"Hola desde Bluetooth!\n\n\n")
adapter.write(b"\x1d\x56\x00")  # Cortar papel

# Cerrar
adapter.close()
```

### 3. Usando Context Manager

```python
from src.adapters.bluetooth_adapter import BluetoothAdapter

# Auto cierra la conexión
with BluetoothAdapter.get_device("00:11:22:33:44:55") as bt:
    bt.write(b"\x1b@")
    bt.write(b"Impresion Bluetooth\n")
    bt.write(b"\x1d\x56\x00")
```

### 4. Usando el Scanner con caché

```python
from src.scanners.bluetooth_scanner import BluetoothScanner

scanner = BluetoothScanner(
    scan_duration=8,     # 8 segundos por escaneo
    cache_ttl=300        # Caché válido por 5 minutos
)

# Primera vez: escanea dispositivos
devices = scanner.scan()

# Segunda llamada: usa caché (más rápido)
devices = scanner.scan()

# Forzar nuevo escaneo
devices = scanner.scan(force=True)

# Buscar por dirección MAC
printer = scanner.find_by_address("00:11:22:33:44:55")

# Buscar por nombre
printers = scanner.find_by_name("POS", partial=True)
```

---

## 🌐 API REST - Endpoints Bluetooth

### Escanear dispositivos Bluetooth

```http
GET /devices/scan
```

**Respuesta:**
```json
{
  "devices": [
    {
      "address": "00:11:22:33:44:55",
      "name": "POS-58",
      "channel": 1,
      "type": "bluetooth"
    }
  ],
  "total": 1
}
```

---

## 🔧 Configuración (settings.yaml)

```yaml
discovery:
  bluetooth_enabled: true
  bluetooth_scan_duration: 8  # Segundos (8 es bueno balance)
```

**Duración del escaneo:**
- `5s`: Rápido pero puede perder dispositivos
- `8s`: **Recomendado** - balance velocidad/cobertura
- `15s`: Máxima cobertura pero lento

---

## 🛠️ Solución de Problemas

### Error: "PyBluez no está instalado"

```bash
pip install pybluez
```

### Error: "Bluetooth no disponible" (Linux)

```bash
# Verificar estado de Bluetooth
sudo systemctl status bluetooth

# Habilitar Bluetooth
sudo systemctl start bluetooth

# Instalar dependencias
sudo apt-get install bluez python3-bluez libbluetooth-dev
```

### Error: "Access denied" (Permisos Linux)

```bash
# Agregar usuario al grupo bluetooth
sudo usermod -a -G bluetooth $USER

# Cerrar sesión y volver a entrar
```

### No encuentra ningún dispositivo

1. **Verificar que el Bluetooth del sistema esté encendido**
2. **Emparejar el dispositivo primero** (algunas impresoras requieren pairing)
3. **Aumentar duración del escaneo** a 15 segundos
4. **Verificar que la impresora esté en modo visible/pairing**

### Canal RFCOMM incorrecto

Si la conexión falla con el canal auto-detectado:

```python
# Probar canales manualmente (usualmente 1-4 para impresoras)
for channel in range(1, 5):
    try:
        adapter = BluetoothAdapter("00:11:22:33:44:55", channel=channel)
        adapter.open()
        print(f"✓ Conectado en canal {channel}")
        adapter.close()
        break
    except:
        print(f"✗ Canal {channel} falló")
```

---

## 📊 Comparación con otras tecnologías

| Aspecto | PyBluez (Classic) | Bleak (BLE) |
|---------|------------------|-------------|
| **Protocolo** | Bluetooth Classic (RFCOMM) | Bluetooth Low Energy (GATT) |
| **Compatibilidad POS** | ✅ 95% impresoras térmicas | ⚠️ Solo impresoras modernas BLE |
| **Velocidad** | ✅ Rápido (1-3 Mbps) | ⚠️ Más lento (125 kbps) |
| **Instalación** | ⚠️ Requiere compilación | ✅ Fácil (pip install) |
| **Plataformas** | Windows, Linux | Windows, Linux, macOS |
| **Uso en POS** | ✅ Estándar de la industria | ⚠️ Impresoras nuevas |

**Arya ESCPOS usa PyBluez** porque es el estándar para impresoras POS tradicionales.

---

## 🔗 Recursos

- **PyBluez Docs:** https://pybluez.readthedocs.io/
- **Bluetooth RFCOMM:** https://en.wikipedia.org/wiki/List_of_Bluetooth_profiles#Serial_Port_Profile_(SPP)
- **node-bluetooth (equivalente Node.js):** https://github.com/song940/node-bluetooth

---

## 📝 Ejemplo Completo

```python
from src.adapters.bluetooth_adapter import BluetoothAdapter
from src.utils.logger import logger

def print_bluetooth_receipt():
    """Ejemplo de impresión de ticket por Bluetooth"""
    
    # 1. Escanear impresoras
    logger.info("Buscando impresoras Bluetooth...")
    printers = BluetoothAdapter.find_printers(duration=8)
    
    if not printers:
        logger.error("No se encontraron impresoras Bluetooth")
        return
    
    # 2. Usar primera impresora encontrada
    printer = printers[0]
    logger.info(f"Usando: {printer['name']} ({printer['address']})")
    
    # 3. Conectar e imprimir
    try:
        with BluetoothAdapter.get_device(printer['address']) as bt:
            # Comandos ESC/POS
            bt.write(b"\x1b@")              # Inicializar
            bt.write(b"\x1b\x61\x01")       # Centrar texto
            bt.write(b"TICKET DE VENTA\n")
            bt.write(b"\x1b\x61\x00")       # Alinear izquierda
            bt.write(b"------------------------\n")
            bt.write(b"Producto 1    $10.00\n")
            bt.write(b"Producto 2    $15.00\n")
            bt.write(b"------------------------\n")
            bt.write(b"TOTAL:        $25.00\n")
            bt.write(b"\n\n\n")
            bt.write(b"\x1d\x56\x00")       # Cortar papel
            
            logger.success("✓ Ticket impreso correctamente")
    
    except Exception as e:
        logger.error(f"Error al imprimir: {e}")

if __name__ == "__main__":
    print_bluetooth_receipt()
```

---

## ✅ Checklist de Implementación

- [x] BluetoothAdapter con PyBluez
- [x] Escaneo de dispositivos (find_printers)
- [x] Auto-detección de canal RFCOMM
- [x] Context manager support
- [x] BluetoothScanner con caché
- [x] Integración en API REST
- [x] Manejo de errores y eventos
- [x] Documentación completa
- [ ] Tests unitarios
- [ ] Tests de integración con impresora real
