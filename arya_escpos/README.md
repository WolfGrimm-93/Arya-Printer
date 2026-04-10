# Arya ESCPOS Service

Servicio REST API local para deteccion y gestion de impresoras ESC/POS y documentos. Se ejecuta en cada PC cliente como servicio de Windows.

## Arquitectura

```
Frontend (Browser)
    |
    v
http://localhost:58181/api/v1/...
    |
    v
Arya ESCPOS Service (FastAPI + Uvicorn)
    |
    +-- Windows Spooler API (win32print)
    +-- USB (libusb / pyusb)
    +-- Network (TCP socket :9100)
    +-- Serial (pyserial COM port)
    +-- Bluetooth (PyBluez RFCOMM)
```

- **Sin base de datos** - descubrimiento en vivo y datos de conexion provistos por el cliente
- **asyncio.to_thread()** - todas las operaciones de I/O bloqueante se ejecutan en hilos
- **Adapter Factory** - creacion centralizada de adaptadores en `adapter_factory.py`
- **1 servicio por PC** - cada PC cliente corre su propia instancia en localhost:58181

---

## Estructura del Proyecto

```
arya_escpos/
├── src/
│   ├── main.py                    # Entry point
│   ├── core/
│   │   ├── base_adapter.py        # Clase abstracta BaseAdapter
│   │   └── command_builder.py     # Constructor de comandos ESC/POS
│   ├── adapters/
│   │   ├── usb_adapter.py         # USB via libusb
│   │   ├── network_adapter.py     # TCP socket
│   │   ├── serial_adapter.py      # COM port
│   │   └── bluetooth_adapter.py   # RFCOMM
│   ├── server/
│   │   ├── api_server.py          # FastAPI factory + CORS
│   │   ├── routes.py              # Router principal
│   │   ├── device_routes.py       # Scan y status de dispositivos
│   │   ├── print_routes.py        # Impresion (tickets, reportes, documentos)
│   │   ├── config_routes.py       # Configuracion
│   │   ├── schemas.py             # Pydantic schemas
│   │   ├── adapter_factory.py     # Creacion centralizada de adaptadores
│   │   └── compat.py              # Imports condicionales win32
│   └── utils/
│       ├── config.py              # Carga YAML + modelo Config
│       ├── logger.py              # Loguru con rotacion diaria
│       ├── exceptions.py          # Excepciones custom
│       ├── pdf_printer.py         # Impresion nativa PDF
│       └── document_converter.py  # Conversion de documentos a PDF
├── config/
│   └── settings.yaml              # Configuracion principal
├── libs/
│   └── libusb-1.0.dll             # Driver USB para Windows
├── tests/                         # Tests con pytest
├── installer.iss                  # Script Inno Setup
├── build_executable.py            # Script PyInstaller
└── requirements.txt
```

---

## Instalacion (Desarrollo)

```bash
cd arya_escpos
python -m venv venv
.\venv\Scripts\activate        # Windows
pip install -r requirements.txt
```

### Configuracion de libusb (solo Windows con USB)

Si vas a usar impresoras USB en Windows, necesitas libusb:

1. La DLL ya esta incluida en `libs/libusb-1.0.dll`
2. Para acceder a impresoras USB sin el Spooler de Windows:
   - Descargar [Zadig](https://zadig.akeo.ie/)
   - Ejecutar como Administrador
   - Options > List All Devices
   - Seleccionar la impresora e instalar driver WinUSB

Ver [libs/README.md](libs/README.md) para mas detalles.

### Ejecutar

```bash
python src/main.py
```

El servicio arranca en `http://localhost:58181`. Documentacion interactiva en `/docs`.

---

## API Endpoints

Base URL: `http://localhost:58181/api/v1`

### Health

| Metodo | Ruta       | Descripcion             |
|--------|------------|-------------------------|
| GET    | `/`        | Info del servicio        |
| GET    | `/health`  | Health check             |

### Dispositivos

| Metodo | Ruta                                           | Descripcion                      |
|--------|------------------------------------------------|----------------------------------|
| GET    | `/api/v1/devices/scan`                         | Escanear todos los dispositivos  |
| GET    | `/api/v1/devices/{type}/{id}/status`           | Estado de un dispositivo         |
| GET    | `/api/v1/devices/windows/{printer}/jobs/{id}`  | Estado de un trabajo de impresion|

### Impresion

| Metodo | Ruta                    | Descripcion                              | Impresoras compatibles                  |
|--------|-------------------------|------------------------------------------|-----------------------------------------|
| POST   | `/api/v1/print`         | Ticket ESC/POS (termicas)                | Epson TM, CUSTOM, Star, Bixolon, Citizen|
| POST   | `/api/v1/print/matrix`  | Texto plano ESC/P (matriciales)          | Epson LX-350, FX-890, Oki Microline     |
| POST   | `/api/v1/print/report`  | Reporte texto plano via Windows          | Cualquier impresora con driver Windows  |
| POST   | `/api/v1/print/document`| Documento PDF/DOCX/TXT via Windows       | Cualquier impresora con driver Windows  |
| GET    | `/api/v1/print/history` | Historial de trabajos enviados           | -                                       |

### Configuracion

| Metodo | Ruta              | Descripcion              |
|--------|--------------------|--------------------------|
| GET    | `/api/v1/config`   | Ver configuracion actual |

---

## Escaneo de Dispositivos

**GET** `/api/v1/devices/scan`

Escanea en vivo por todos los protocolos habilitados.

**Respuesta:**
```json
{
  "found": 3,
  "devices": [
    {
      "id": "win-HP_LaserJet",
      "type": "windows",
      "name": "HP LaserJet",
      "manufacturer": "HP",
      "description": "local"
    },
    {
      "id": "usb-04b8:0202",
      "type": "usb",
      "name": "TM-T20",
      "manufacturer": "EPSON",
      "vid": "04b8",
      "pid": "0202"
    },
    {
      "id": "serial-COM3",
      "type": "serial",
      "name": "COM3",
      "com_port": "COM3"
    }
  ]
}
```

---

## Estado de Dispositivo

**GET** `/api/v1/devices/{type}/{id}/status`

Tipos soportados:

| Tipo      | Ejemplo de ruta                              |
|-----------|----------------------------------------------|
| windows   | `/devices/windows/HP_LaserJet/status`        |
| usb       | `/devices/usb/04b8:0202/status`              |
| network   | `/devices/network/192.168.1.100:9100/status` |
| serial    | `/devices/serial/COM3/status`                |

**Respuesta (Windows):**
```json
{
  "device_type": "windows",
  "printer_name": "HP LaserJet",
  "is_online": true,
  "status": "Ready",
  "status_code": 0,
  "errors": [],
  "warnings": [],
  "details": [],
  "jobs_in_queue": 0
}
```

---

## Impresion

### Ticket ESC/POS — Termicas

**POST** `/api/v1/print`

**Impresoras compatibles:** Epson TM-T20/T88/T70, CUSTOM P3L/Q3X, Star TSP100/TSP650, Bixolon SRP-350, Citizen CT-S310II. Cualquier termica ESC/POS de 58mm u 80mm.

El cliente indica el tipo de dispositivo y los datos de conexion. No hay device_id ni base de datos.

Campos comunes:

| Campo        | Tipo   | Requerido | Descripcion                              |
|--------------|--------|-----------|------------------------------------------|
| `type`       | string | Si        | Tipo de conexion (windows, usb, network, serial, bluetooth) |
| `content`    | string | Si        | Texto a imprimir                         |
| `header_image` | string | No       | Logo/imagen en base64 (se imprime antes del contenido) |
| `image_width` | int   | No        | Ancho maximo en dots (default 576 para 80mm) |

**Ejemplo con logo:**
```json
{
  "type": "windows",
  "printer_name": "POS-80",
  "header_image": "iVBORw0KGgo...",
  "image_width": 576,
  "content": "FACTURA #001\nTotal: $100"
}
```

Campos requeridos segun tipo:

| type      | Campos requeridos           |
|-----------|-----------------------------|
| windows   | `printer_name`              |
| usb       | `vid`, `pid`                |
| network   | `ip` (port default 9100)    |
| serial    | `com_port`                  |
| bluetooth | `address`                   |

**Respuesta:**
```json
{
  "success": true,
  "message": "Print job sent (windows)",
  "bytes_sent": 245
}
```

---

### Texto Plano — Matriciales (ESC/P)

**POST** `/api/v1/print/matrix`

**Impresoras compatibles:** Epson LX-350, LX-300+II, LX-810, FX-890II, DFX-9000. Oki Microline 320/390. Cualquier matricial de 9 o 24 pines con protocolo ESC/P.

Envia texto plano con inicializacion ESC/P. No usa comandos ESC/POS (sin corte, sin graficos raster).

| Campo        | Tipo    | Requerido | Default  | Descripcion                              |
|--------------|---------|-----------|----------|------------------------------------------|
| `type`       | string  | Si        | -        | `windows`, `usb`, o `serial`             |
| `content`    | string  | Si        | -        | Texto a imprimir                         |
| `encoding`   | string  | No        | `cp850`  | Codificacion del texto                   |
| `form_feed`  | bool    | No        | `true`   | Avanzar pagina al terminar (FF)          |
| `printer_name` | string | Si*      | -        | Nombre Windows (para type=windows)       |
| `vid`        | string  | Si*       | -        | VID hex (para type=usb)                  |
| `pid`        | string  | Si*       | -        | PID hex (para type=usb)                  |
| `com_port`   | string  | Si*       | -        | Puerto COM (para type=serial)            |
| `baud_rate`  | int     | No        | `9600`   | Baudrate serial (para type=serial)       |

**Ejemplo via Windows driver (recomendado):**
```json
{
  "type": "windows",
  "printer_name": "Epson LX-350",
  "content": "FACTURA #001\nCliente: Juan Perez\nTotal: $100.00"
}
```

**Ejemplo via serial RS-232:**
```json
{
  "type": "serial",
  "com_port": "COM1",
  "baud_rate": 9600,
  "content": "FACTURA #001\nTotal: $100.00",
  "form_feed": true
}
```

**Respuesta:**
```json
{
  "success": true,
  "message": "Matrix print job sent (windows)",
  "bytes_sent": 48
}
```

---

### Reporte

**POST** `/api/v1/print/report`

**Impresoras compatibles:** Cualquier impresora con driver de Windows (laser, tinta, matriciales).

Imprime texto formateado en una impresora Windows (documentos, no ESC/POS).

```json
{
  "printer_name": "HP LaserJet",
  "title": "Reporte de Ventas",
  "content": "Contenido del reporte..."
}
```

### Documento

**POST** `/api/v1/print/document`

**Impresoras compatibles:** Cualquier impresora con driver de Windows que soporte PDF.

Sube un archivo (PDF, DOCX, TXT, etc.) y lo imprime en una impresora Windows.

```
POST /api/v1/print/document?printer_name=HP_LaserJet&orientation=portrait&copies=1&color=color&duplex=simplex
Content-Type: multipart/form-data

file: [archivo]
```

Parametros opcionales: `orientation` (portrait/landscape), `copies`, `color` (color/grayscale), `duplex` (simplex/duplex).

El servicio convierte automaticamente DOCX y otros formatos a PDF antes de imprimir. Intenta multiples metodos: nativo (PyMuPDF), PDFtoPrinter, SumatraPDF.

**Respuesta:**
```json
{
  "success": true,
  "message": "Document 'factura.pdf' sent to HP LaserJet",
  "file_size_bytes": 125432,
  "file_size": "122.49 KB",
  "job_id": 123,
  "track_url": "/api/v1/devices/windows/HP_LaserJet/jobs/123"
}
```

### Historial de Impresion

**GET** `/api/v1/print/history`

Retorna los ultimos trabajos enviados al servicio (max 500, en memoria, se reinicia con el servicio). Util para verificar trabajos de impresoras rapidas (matriciales, termicas) que completan antes de aparecer en la cola Windows.

```bash
# Ultimos 50 trabajos
curl http://localhost:58181/api/v1/print/history

# Filtrar por impresora
curl "http://localhost:58181/api/v1/print/history?printer_name=Epson+LX-350&limit=20"
```

**Respuesta:**
```json
{
  "total": 2,
  "jobs": [
    {
      "id": 1744300000123,
      "timestamp": "2026-04-10T10:45:22",
      "printer_name": "Epson LX-350",
      "document": "matrix-ticket",
      "protocol": "escp",
      "bytes_sent": 312,
      "job_id": null,
      "status": "sent"
    },
    {
      "id": 1744299900456,
      "timestamp": "2026-04-10T10:43:10",
      "printer_name": "HP LaserJet",
      "document": "factura.pdf",
      "protocol": "document",
      "bytes_sent": 125432,
      "job_id": 123,
      "status": "sent"
    }
  ]
}
```

Protocolos posibles: `escpos` | `escp` | `document` | `report`

---

### Rastreo de Trabajos de Impresion

**GET** `/api/v1/devices/windows/{printer_name}/jobs/{job_id}`

Obtiene el estado actual de un trabajo en la cola de impresion.

```bash
curl http://localhost:58181/api/v1/devices/windows/HP_LaserJet/jobs/123
```

**Respuesta:**
```json
{
  "job_id": 123,
  "printer_name": "HP LaserJet",
  "document_name": "factura.pdf",
  "status": "printing",
  "status_code": 5,
  "total_pages": 10,
  "pages_printed": 3,
  "position_in_queue": 1,
  "submitted_time": "2026-03-31 10:30:45",
  "size_in_bytes": 125432
}
```

Estados posibles:
- `queued` (0) - Trabajo en cola esperando
- `paused` (1) - Pausado
- `error` (2) - Error en la impresion
- `deleting` (3) - Se esta eliminando
- `spooling` (4) - Siendo spooled
- `printing` (5) - Imprimiendo actualmente
- `printed` (6) - Ya se imprimi

---

## Configuracion

Archivo: `config/settings.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 58181
  reload: false
  workers: 1

logging:
  level: "INFO"
  log_dir: "logs"
  retention_days: 15

discovery:
  auto_scan_interval: 30
  usb_enabled: true
  network_enabled: true
  serial_enabled: true
  bluetooth_enabled: true
  bluetooth_scan_duration: 8

devices:
  auto_reconnect: true
  connection_timeout: 5
  retry_attempts: 3

printers:
  default_encoding: "utf-8"
  paper_width: 80

network_scan:
  enabled: true
  subnets:
    - "192.168.1.0/24"
  ports:
    - 9100
  timeout: 2
```

---

## Logs

- Rotacion diaria: `logs/YYYY-MM-DD/arya_escpos.log`
- Limpieza automatica de logs mayores a 15 dias
- Compresion automatica de logs rotados (.zip)

---

## Tipos de Impresora Soportados

| Endpoint            | Protocolo  | Modelos compatibles                              | Conexion              |
|---------------------|------------|--------------------------------------------------|-----------------------|
| `/print`            | ESC/POS    | Epson TM, CUSTOM P3L, Star TSP, Bixolon, Citizen | Windows/USB/Red/Serial/BT |
| `/print/matrix`     | ESC/P      | Epson LX-350/FX-890, Oki Microline               | Windows/USB/Serial    |
| `/print/report`     | Win32 RAW  | Cualquier impresora con driver Windows           | Windows               |
| `/print/document`   | Win32 PDF  | Cualquier impresora con driver Windows           | Windows               |

### Conexiones disponibles por protocolo

| Tipo       | Protocolo        | Dependencia           | Uso tipico               |
|------------|------------------|-----------------------|--------------------------|
| Windows    | Win32 Spooler    | pywin32               | Impresoras instaladas    |
| USB        | libusb           | pyusb + libusb DLL    | POS termicas directas    |
| Network    | TCP socket :9100 | stdlib                | POS en red               |
| Serial     | COM port         | pyserial              | POS/Matriciales RS-232   |
| Bluetooth  | RFCOMM           | PyBluez (Python <3.12)| POS portatiles           |

---

## Build y Deployment

### Compilar ejecutable

```bash
python build_executable.py
# Resultado: dist/AryaESCPOS/AryaESCPOS.exe
```

Requiere PyInstaller (`pip install pyinstaller`).

### Crear instalador

1. Instalar [Inno Setup 6](https://jrsoftware.org/isdl.php)
2. Abrir `installer.iss` en Inno Setup Compiler
3. Build > Compile (F9)
4. Resultado: `installer_output/AryaESCPOS_Setup_v1.0.0.exe`

### Que hace el instalador

- Instala en `C:\Program Files\AryaESCPOS`
- Crea Tarea Programada de Windows (inicio automatico con SYSTEM)
- Opcionalmente abre puerto 58181 en firewall
- Opcionalmente escanea impresoras
- Desinstalacion limpia desde Panel de Control

### Instalacion silenciosa

```powershell
.\AryaESCPOS_Setup_v1.0.0.exe /VERYSILENT
```

### Actualizar version existente

El instalador detecta la version previa, detiene el servicio, actualiza archivos y reinicia. La configuracion se mantiene.

1. Cambiar version en `installer.iss`: `#define MyAppVersion "1.1.0"`
2. Recompilar: `python build_executable.py`
3. Recompilar instalador (F9 en Inno Setup)
4. Ejecutar nuevo instalador sobre la instalacion existente

---

## Comandos Utiles en Produccion

```powershell
# Verificar servicio
Get-ScheduledTask -TaskName "AryaESCPOS"

# Iniciar/detener
schtasks /Run /TN "AryaESCPOS"
taskkill /F /IM AryaESCPOS.exe

# Probar API
Invoke-RestMethod http://localhost:58181/api/v1/devices/scan

# Ver logs
Get-Content "C:\Program Files\AryaESCPOS\logs\*\arya_escpos.log" -Tail 50

# Desinstalar
"C:\Program Files\AryaESCPOS\unins000.exe"
```

---

## Testing

```bash
.\venv\Scripts\activate

# Ejecutar todos los tests
pytest -v

# Con cobertura
pytest --cov=src --cov-report=html

# Solo un componente
pytest tests/test_config.py -v
pytest tests/test_api.py -v
pytest -k "bluetooth" -v

# Paralelo (requiere pytest-xdist)
pytest -n auto
```

| Archivo            | Que prueba                        |
|--------------------|-----------------------------------|
| test_config.py     | Carga y validacion de config      |
| test_adapters.py   | Adaptadores con mocks             |
| test_api.py        | Endpoints FastAPI                  |
| test_commands.py   | Constructor de comandos ESC/POS    |
| test_scanners.py   | Scanner Bluetooth con cache        |

---

## Bluetooth

PyBluez no es compatible con Python 3.12+. Alternativas:

- **bleak** - BLE moderno, multiplataforma (`pip install bleak`)
- **pybluez2** - Fork con soporte parcial

Si PyBluez no esta instalado, los endpoints de Bluetooth se deshabilitan automaticamente sin afectar el resto del servicio.

---

## Troubleshooting

| Problema                          | Solucion                                          |
|-----------------------------------|---------------------------------------------------|
| Puerto 58181 en uso                | `netstat -ano \| findstr :58181` y matar proceso   |
| USB no detecta impresoras         | Verificar libusb DLL en `libs/` e instalar Zadig  |
| Servicio no inicia al reiniciar   | `Get-ScheduledTask -TaskName "AryaESCPOS"`        |
| Error "No module named 'src'"     | Ejecutar desde raiz: `cd arya_escpos && python src/main.py` |
| Firewall bloquea conexion         | Ejecutar `cleanup_firewall.ps1` como Admin (limpia reglas duplicadas) |
| Reglas duplicadas en firewall     | PowerShell como Admin: `.\cleanup_firewall.ps1` |
| PDF no imprime                    | Instalar PDFtoPrinter o SumatraPDF como fallback  |

---

## Stack Tecnologico

- **FastAPI** + **Uvicorn** - Framework web async
- **Pydantic** - Validacion de datos y schemas
- **Loguru** - Logging con rotacion diaria
- **PyUSB** - Comunicacion USB (requiere libusb)
- **PySerial** - Comunicacion serial
- **PyBluez** - Bluetooth Classic/RFCOMM
- **Zeroconf** - Discovery de red
- **PyWin32** - Integracion con impresoras Windows
- **PyMuPDF** - Renderizado nativo de PDF
- **PyInstaller** - Compilacion a ejecutable
- **Inno Setup** - Generacion de instalador

---

## Registro de Cambios

### v1.3.0
- **Nuevo endpoint /print/matrix**: soporte para impresoras matriciales ESC/P (Epson LX-350, FX-890, Oki Microline)
- **Comentarios de compatibilidad**: cada endpoint documenta que modelos de impresoras son compatibles
- Conexiones soportadas en matriciales: Windows driver, USB directo, Serial RS-232

### v1.2.0
- Puerto cambiado de 8181 a 58181 (rango privado IANA, sin conflictos)
- Eliminados emojis de main.py (fix UnicodeEncodeError en consola cp1252)
- /print ya no tiene template hardcodeado, envia el content del cliente
- /print/report usa send_raw_to_windows en vez de ShellExecute
- Validacion cruzada de campos movida a Pydantic model_validator
- vid/pid consistente como str hex en schemas y scan
- DEVMODE settings (orientation, copies, color, duplex) aplicados correctamente
- pdf_printer.py: imports protegidos, logger, DEVMODE via hDC
- document_converter.py: NamedTemporaryFile, logger, Excel cleanup
- Eliminados asyncio.sleep() innecesarios
- **CORS**: allow_origins=["*"] para soportar cualquier frontend local
- **ESC/POS Images**: nuevo campo `header_image` (base64) en /print para logos/cabeceras
- **Print Job Tracking**: POST /print/document devuelve job_id + track_url
- **GET /devices/windows/{printer}/jobs/{job_id}**: rastrear estado del trabajo en cola
- **Smart Firewall Setup**: instalador verifica si regla existe antes de crear (no duplica)

### v1.1.0
- Eliminada dependencia de SQLAlchemy/SQLite
- Arquitectura client-driven (sin device_id, sin DB)
- asyncio.to_thread() en todas las operaciones bloqueantes
- Centralizado adapter_factory.py y compat.py
- Eliminados endpoints redundantes (/printers, /devices/{id}, DELETE)
- Sistema de logs con rotacion diaria y auto-limpieza

### v1.0.0
- Release inicial
- Soporte USB/Serial/Network/Bluetooth/Windows
- API REST + WebSocket
- Instalador Windows con Task Scheduler
- ESC/POS command builder
