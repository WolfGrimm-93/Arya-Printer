# 🚀 Guía de Inicio Rápido - Arya ESCPOS

## 📦 Instalación

### 1. Requisitos Previos

**Windows:**
```powershell
# 1. Descargar e instalar Python 3.11+
# https://www.python.org/downloads/

# 2. Para soporte USB - Instalar libusb (ELIGE UNA):

# FORMA A - Usando Zadig (Recomendado para USB)
# 1. Descargar Zadig: https://zadig.akeo.ie/
# 2. Conectar tu impresora USB
# 3. Ejecutar Zadig como Administrador
# 4. Options > List All Devices
# 5. Seleccionar tu impresora
# 6. Instalar driver WinUSB o libusb-win32
# ✅ Más fácil, interfaz gráfica

# FORMA B - Instalación Manual de libusb
# 1. Descargar: https://github.com/libusb/libusb/releases
# 2. Extraer libusb-1.0.dll (versión 64-bit)
# 3. Copiar a: C:\Windows\System32 (64-bit) o C:\Windows\SysWOW64 (32-bit)
# ✅ Más control sobre la instalación

# 3. Para Network/Serial - NO requiere libusb
# Si NO usas USB, desactiva en config/settings.yaml:
#   discovery:
#     usb_enabled: false
#     network_enabled: true  # Impresoras de red
#     serial_enabled: true   # Impresoras COM/Serial
# ✅ Funciona sin instalaciones adicionales
```

**Linux:**
```bash
sudo apt-get update
sudo apt-get install python3 python3-pip python3-venv
sudo apt-get install libusb-1.0-0-dev
```

### 2. Configurar Entorno Virtual

```powershell
# Windows
cd arya_escpos
python -m venv venv
.\venv\Scripts\activate

# Linux/Mac
cd arya_escpos
python3 -m venv venv
source venv/bin/activate
```

### 3. Instalar Dependencias

```powershell
pip install -r requirements.txt
```

## ▶️ Ejecutar el Servicio

```powershell
# Asegúrate de estar en la carpeta arya_escpos con el venv activado
python src/main.py
```

Verás:
```
✅ Configuration loaded
🖨️  Arya ESCPOS Service Starting
✅ Database initialized: sqlite:///./arya_escpos.db
✅ FastAPI application created
🚀 Starting server on 0.0.0.0:8181
📚 API Documentation: http://localhost:8181/docs
🔌 WebSocket: ws://localhost:8181/ws
```

## 🧪 Probar el API

### 1. Abrir Documentación Interactiva

Visita: http://localhost:8181/docs

### 2. Escanear Dispositivos

```powershell
# PowerShell
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices/scan" -Method Get

# O usar curl
curl http://localhost:8181/api/v1/devices/scan
```

### 3. Listar Dispositivos

```powershell
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices" -Method Get
```

### 4. Obtener Configuración

```powershell
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/config" -Method Get
```

## 🔧 Configuración

Editar `config/settings.yaml`:

```yaml
server:
  port: 8181  # Cambiar puerto si es necesario
  
discovery:
  auto_scan_interval: 30  # Escaneo automático cada 30 segundos
  usb_enabled: true
  network_enabled: true
```

## 📝 Ejemplos de Uso

### Python Client

```python
import requests

# Base URL
API_URL = "http://localhost:8181/api/v1"

# Escanear dispositivos
response = requests.get(f"{API_URL}/devices/scan")
print(response.json())

# Listar dispositivos
response = requests.get(f"{API_URL}/devices")
devices = response.json()
for device in devices:
    print(f"Device: {device['id']} - {device['product']}")
```

### JavaScript/Browser

```javascript
// Fetch devices
fetch('http://localhost:8181/api/v1/devices')
  .then(res => res.json())
  .then(data => console.log(data));

// WebSocket connection
const ws = new WebSocket('ws://localhost:8181/ws/devices');
ws.onmessage = (event) => {
  console.log('Event:', JSON.parse(event.data));
};
```

### PowerShell Script

```powershell
# Función para escanear e imprimir
function Scan-Printers {
    $result = Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices/scan"
    Write-Host "Encontradas $($result.found) impresoras:"
    $result.devices | Format-Table -Property vid, pid, manufacturer, product
}

# Ejecutar
Scan-Printers
```

## 🐛 Solución de Problemas

### Error: "No USB printers found"

**Windows:**
1. Asegurar que libusb-1.0.dll está instalado
2. Ejecutar como Administrador
3. Verificar que la impresora está conectada y encendida

**Linux:**
```bash
# Agregar usuario al grupo de USB
sudo usermod -a -G dialout $USER
# Cerrar sesión y volver a entrar
```

### Error: "Module 'usb' not found"

```powershell
pip install pyusb
```

### Error: "Port already in use"

Cambiar puerto en `config/settings.yaml`:
```yaml
server:
  port: 8182  # Usar otro puerto
```

## 📊 Ver Logs

Los logs se guardan en `logs/arya_escpos.log`

```powershell
# Ver logs en tiempo real (PowerShell)
Get-Content logs/arya_escpos.log -Wait

# O con tail en Linux
tail -f logs/arya_escpos.log
```

## 🛑 Detener el Servicio

Presionar `Ctrl + C` en la terminal donde está corriendo.

## 🎯 Próximos Pasos

1. ✅ Escanear e identificar tus dispositivos
2. ✅ Configurar impresora por defecto
3. ✅ Integrar con tu aplicación
4. ✅ Implementar funciones de impresión ESC/POS

## 📚 Documentación Completa

- API Docs: http://localhost:8181/docs
- ReDoc: http://localhost:8181/redoc
- README: Ver README.md en la raíz del proyecto

## 🆘 Soporte

Para reportar issues o solicitar features, abre un issue en el repositorio.
