# 📘 Documentación API - Arya ESCPOS

Sistema de gestión e impresión para impresoras ESC/POS, térmicas y de documentos Windows.

**Base URL:** `http://localhost:8181/api/v1`  
**Swagger UI:** `http://localhost:8181/docs`  
**Versión:** 1.0.0

---

## 📑 Tabla de Contenidos

1. [Gestión de Dispositivos](#gestión-de-dispositivos)
   - Listar Dispositivos
   - Escanear Dispositivos
   - Obtener Dispositivo Específico
   - Eliminar Dispositivo
   - Obtener Estado de Dispositivo (Status) ⭐ NUEVO
2. [Impresión](#impresión)
   - Imprimir Ticket ESC/POS
   - Imprimir Reporte Formateado
   - Imprimir Documento con Configuración Avanzada
3. [Configuración](#configuración)
4. [Modelos de Datos](#modelos-de-datos)
5. [Códigos de Error](#códigos-de-error)

---

## 🖨️ Gestión de Dispositivos

### 1. Listar Dispositivos

**Endpoint:** `GET /api/v1/devices`

**Descripción:** Obtiene la lista completa de todos los dispositivos de impresión registrados y activos en el sistema. Incluye impresoras Windows, USB, Bluetooth y de red.

**Parámetros:** Ninguno

**Respuesta exitosa (200):**
```json
[
  {
    "id": "win-Canon_GX6000_series",
    "type": "windows",
    "manufacturer": "Canon",
    "product": "Canon GX6000 series",
    "alias": null,
    "is_active": true
  },
  {
    "id": "usb-0x04b8:0x0e15",
    "type": "usb",
    "manufacturer": "Epson",
    "product": "TM-T20III",
    "alias": "Caja Principal",
    "is_active": true
  }
]
```

**Casos de uso:**
- Ver qué impresoras están disponibles antes de imprimir
- Listar dispositivos para mostrar en una UI de selección
- Verificar si un dispositivo específico está registrado

---

### 2. Escanear Dispositivos

**Endpoint:** `GET /api/v1/devices/scan`

**Descripción:** Busca y registra automáticamente nuevas impresoras conectadas al sistema. Escanea impresoras Windows (locales y de red), dispositivos USB, y opcionalmente Bluetooth. Los dispositivos encontrados se registran automáticamente en la base de datos.

**Parámetros:** Ninguno

**Respuesta exitosa (200):**
```json
{
  "found": 3,
  "devices": [
    {
      "name": "Canon GX6000 series",
      "description": "Canon Inkjet Printer",
      "type": "windows",
      "connection_type": "local"
    },
    {
      "vid": "0x04b8",
      "pid": "0x0e15",
      "manufacturer": "Epson",
      "product": "TM-T20III",
      "serial": "ABC123456"
    },
    {
      "address": "00:11:22:33:44:55",
      "name": "Bluetooth Printer",
      "type": "bluetooth"
    }
  ]
}
```

**Comportamiento:**
- Detecta impresoras Windows instaladas (locales y compartidas en red)
- Escanea dispositivos USB conectados con interfaz de impresora
- Busca impresoras Bluetooth (si está habilitado en config)
- Registra automáticamente dispositivos nuevos
- No duplica dispositivos ya existentes

**Casos de uso:**
- Primera configuración del sistema
- Detectar impresoras recién conectadas
- Actualizar lista después de instalar una nueva impresora Windows
- Troubleshooting de conectividad

---

### 3. Obtener Dispositivo Específico

**Endpoint:** `GET /api/v1/devices/{device_id}`

**Descripción:** Recupera la información detallada de un dispositivo específico mediante su ID único.

**Parámetros:**
- **device_id** (path, requerido): ID único del dispositivo
  - Ejemplos: `win-Canon_GX6000_series`, `usb-0x04b8:0x0e15`, `bt-00:11:22:33:44:55`

**Respuesta exitosa (200):**
```json
{
  "id": "win-Canon_GX6000_series",
  "type": "windows",
  "manufacturer": "Canon",
  "product": "Canon GX6000 series",
  "alias": null,
  "is_active": true
}
```

**Respuesta error (404):**
```json
{
  "detail": "Device not found"
}
```

**Casos de uso:**
- Verificar que un dispositivo existe antes de imprimir
- Obtener detalles de un dispositivo para mostrar en UI
- Validar el tipo de dispositivo antes de enviar comandos específicos

---

### 4. Eliminar Dispositivo

**Endpoint:** `DELETE /api/v1/devices/{device_id}`

**Descripción:** Desactiva lógicamente un dispositivo del sistema. El dispositivo no se elimina físicamente de la base de datos, solo se marca como inactivo (`is_active = false`).

**Parámetros:**
- **device_id** (path, requerido): ID del dispositivo a eliminar

**Respuesta exitosa (200):**
```json
{
  "message": "Device deleted successfully"
}
```

**Respuesta error (404):**
```json
{
  "detail": "Device not found"
}
```

**Casos de uso:**
- Remover impresoras desconectadas permanentemente
- Limpiar la lista de dispositivos
- Ocultar impresoras temporalmente sin perder configuración

---

### 5. Obtener Estado de Dispositivo

**Endpoint:** `GET /api/v1/devices/{device_id}/status`

**Descripción:** Consulta el estado actual de una impresora sin necesidad de imprimir. Detecta si la impresora está online/offline, tiene errores (papel atascado, sin papel, tapa abierta), o warnings (toner bajo). 

Para impresoras Windows usa el spooler de Windows, para impresoras ESC/POS envía comandos DLE EOT.

**Parámetros:**
- **device_id** (path, requerido): ID del dispositivo a verificar

**Respuesta exitosa (200) - Windows Printer:**
```json
{
  "device_id": "win-Canon_GX6000_series",
  "device_type": "windows",
  "printer_name": "Canon GX6000 series",
  "is_online": true,
  "status": "Ready",
  "status_code": 0,
  "errors": [],
  "warnings": [],
  "details": [],
  "jobs_in_queue": 0
}
```

**Respuesta con errores - Windows:**
```json
{
  "device_id": "win-Canon_GX6000_series",
  "device_type": "windows",
  "printer_name": "Canon GX6000 series",
  "is_online": false,
  "status": "Error: Offline, Out of paper",
  "status_code": 128,
  "errors": ["Offline", "Out of paper"],
  "warnings": [],
  "details": [],
  "jobs_in_queue": 2
}
```

**Respuesta exitosa (200) - ESC/POS Printer:**
```json
{
  "device_id": "usb-0x04b8:0x0e15",
  "device_type": "usb",
  "is_online": true,
  "status": "Ready",
  "status_byte": "0x12",
  "errors": [],
  "warnings": [],
  "details": ["Online", "Drawer kick-out pin HIGH"]
}
```

**Respuesta con error de comunicación - ESC/POS:**
```json
{
  "device_id": "usb-0x04b8:0x0e15",
  "device_type": "usb",
  "is_online": false,
  "status": "Cannot communicate: Failed to open USB device",
  "errors": ["Communication error"],
  "warnings": [],
  "details": ["Failed to open USB device"]
}
```

**Errores detectados (Windows):**
- `Offline` - Impresora apagada o desconectada
- `Paper jam` - Atasco de papel
- `Out of paper` - Sin papel
- `Door open` - Tapa/puerta abierta
- `Printer error` - Error general
- `Not available` - No disponible
- `Out of memory` - Sin memoria
- `No toner` - Sin tóner/tinta

**Warnings detectados (Windows):**
- `Toner low` - Tóner/tinta baja
- `User intervention required` - Requiere intervención
- `Paper problem` - Problema con papel
- `Paused` - Pausada manualmente

**Errores detectados (ESC/POS):**
- `Offline` - Impresora offline (bit 3 = 1)

**Warnings detectados (ESC/POS):**
- `Waiting for online recovery` - Esperando recuperación (bit 5)

**Casos de uso:**
- Verificar disponibilidad antes de imprimir
- Monitoreo de impresoras en tiempo real
- Alertas de errores (papel, tóner, atasco)
- Dashboard de estado de impresoras
- Validación en formularios de impresión

**Notas técnicas:**

**Windows:** Usa `win32print.GetPrinter()` con level 2 para obtener PRINTER_INFO_2 que incluye el campo Status (bitmask con flags como PRINTER_STATUS_OFFLINE, PRINTER_STATUS_PAPER_JAM, etc.)

**ESC/POS:** Envía comando `DLE EOT 1` (bytes: 0x10 0x04 0x01) y lee 1 byte de respuesta. Los bits del byte indican:
- Bit 3: Online (0) / Offline (1)
- Bit 5: Waiting for online recovery
- Bit 6: Paper feed button pressed
- Bit 2: Drawer kick-out connector pin 3 HIGH

---

### 6. Listar Impresoras (Alias)

**Endpoint:** `GET /api/v1/printers`

**Descripción:** Alias de `/api/v1/devices`. Retorna la misma información pero con un nombre semánticamente más específico para APIs orientadas a impresión.

**Parámetros:** Ninguno

**Respuesta:** Idéntica a `GET /devices`

**Casos de uso:**
- Usar un nombre de endpoint más descriptivo en el código cliente
- Compatibilidad con integraciones que esperan `/printers`

---

## 🖨️ Impresión

### 6. Imprimir Ticket ESC/POS

**Endpoint:** `POST /api/v1/print`

**Descripción:** Imprime un ticket de texto plano a impresoras térmicas o matriciales usando comandos ESC/POS. El texto se formatea automáticamente con:
- Título centrado y en doble tamaño
- Contenido alineado a la izquierda
- Separadores decorativos
- Corte de papel automático (si la impresora lo soporta)

**Endpoint:** `POST /api/v1/print`

**Descripción:** Imprime un ticket de texto plano a impresoras térmicas o matriciales usando comandos ESC/POS. El texto se formatea automáticamente con:
- Título centrado y en doble tamaño
- Contenido alineado a la izquierda
- Separadores decorativos
- Corte de papel automático (si la impresora lo soporta)

**Encoding:** CP850 (compatible con caracteres en español)

**Request Body (JSON):**
```json
{
  "device_id": "usb-0x04b8:0x0e15",
  "content": "Producto 1 ............ $10.00\nProducto 2 ............ $15.00\n\nSUBTOTAL .............. $25.00\nIVA 16% ............... $4.00\nTOTAL ................. $29.00"
}
```

**Parámetros:**
- **device_id** (string, requerido): ID del dispositivo (Windows, USB, Bluetooth o Serial)
- **content** (string, requerido): Texto del ticket (soporta `\n` para saltos de línea)

**Respuesta exitosa (200):**
```json
{
  "success": true,
  "message": "Print job sent to TM-T20III",
  "bytes_sent": 342
}
```

**Respuestas error:**
- **404:** Device not found
- **500:** Failed to open device / Windows printer support not available

**Comandos ESC/POS generados:**
- `ESC @` - Reset impresora
- `ESC a 1` - Centrar texto (título)
- `GS ! 11` - Doble ancho y alto (título)
- `GS ! 00` - Tamaño normal
- `ESC a 0` - Alinear izquierda (contenido)
- `GS V 00` - Cortar papel

**Dispositivos compatibles:**
- ✅ Windows (RAW printing)
- ✅ USB (write directo)
- ✅ Network (socket TCP)
- ✅ Serial/COM port
- ✅ Bluetooth (si PyBluez disponible)

**Casos de uso:**
- Tickets de venta en POS
- Comandas de cocina en restaurantes
- Etiquetas de productos
- Comprobantes de pago

---

### 9. Imprimir Reporte Formateado

**Endpoint:** `POST /api/v1/print/report`

**Descripción:** Imprime un reporte de texto formateado en impresoras de documentos Windows (A4, Letter). Crea un archivo temporal `.txt` con formato profesional y lo envía a la impresora usando `ShellExecute` de Windows.

**Solo Windows Printers:** Este endpoint requiere dispositivos tipo `windows`.

**Request Body (JSON):**
```json
{
  "device_id": "win-Canon_GX6000_series",
  "title": "REPORTE DE VENTAS DIARIAS",
  "content": "Fecha: 26/01/2026\nCajero: Juan Pérez\n\n================================\n\nVentas del día:\n- Producto A: 10 unidades - $500.00\n- Producto B: 5 unidades - $250.00\n- Producto C: 8 unidades - $400.00\n\n================================\n\nTOTAL DEL DÍA: $1,150.00\nForma de pago: 60% Efectivo, 40% Tarjeta"
}
```

**Parámetros:**
- **device_id** (string, requerido): ID de una impresora Windows
- **title** (string, requerido): Título del reporte (se centra automáticamente en 80 caracteres)
- **content** (string, requerido): Contenido del reporte (soporta `\n` para saltos de línea)

**Formato generado:**
```
                           REPORTE DE VENTAS DIARIAS                           
================================================================================

Fecha: 26/01/2026
Cajero: Juan Pérez
...

--------------------------------------------------------------------------------
Impreso: Canon GX6000 series
```

**Respuesta exitosa (200):**
```json
{
  "success": true,
  "message": "Report sent to Canon GX6000 series (document printer)"
}
```

**Respuestas error:**
- **404:** Device not found
- **400:** Reports only supported for Windows printers
- **500:** Windows printer support not available

**Casos de uso:**
- Reportes de fin de turno
- Resúmenes de ventas diarias/semanales
- Reportes de inventario
- Documentos administrativos simples
- Impresión rápida sin necesidad de generar PDF

---

### 10. Imprimir Documento con Configuración Avanzada ⭐

**Endpoint:** `POST /api/v1/print/document`

**Descripción:** Imprime documentos (PDF, DOCX, TXT, etc.) en impresoras Windows con control total sobre configuración de impresión: orientación, copias, color y dúplex. Utiliza DEVMODE de Windows para configurar la impresora antes de enviar el documento.

**Solo Windows Printers:** Este endpoint requiere dispositivos tipo `windows`.

**Tipos de archivo soportados:**
- ✅ PDF (recomendado)
- ✅ DOCX, DOC
- ✅ TXT
- ✅ XLS, XLSX
- ✅ Cualquier formato asociado con una aplicación en Windows

**Request (multipart/form-data):**

**Query Parameters:**
| Parámetro | Tipo | Valores | Default | Descripción |
|-----------|------|---------|---------|-------------|
| `device_id` | string | - | **requerido** | ID de la impresora Windows |
| `orientation` | string | `portrait`, `landscape` | `portrait` | Orientación de la página |
| `copies` | int | `1-99` | `1` | Número de copias |
| `color` | string | `color`, `grayscale` | `color` | Modo de color |
| `duplex` | string | `simplex`, `duplex` | `simplex` | Impresión simple/doble cara |

**Form Data:**
- **file** (file, requerido): Archivo del documento a imprimir

**Ejemplos de Request:**

**cURL:**
```bash
# Imprimir PDF a color, doble cara, 2 copias
curl -X POST \
  "http://localhost:8181/api/v1/print/document?device_id=win-Canon_GX6000_series&orientation=portrait&copies=2&color=color&duplex=duplex" \
  -F "file=@factura.pdf"

# Imprimir DOCX en escala de grises, horizontal
curl -X POST \
  "http://localhost:8181/api/v1/print/document?device_id=win-HP_LaserJet&orientation=landscape&color=grayscale" \
  -F "file=@reporte.docx"
```

**JavaScript (Fetch API):**
```javascript
const formData = new FormData();
formData.append('file', fileInput.files[0]);

const params = new URLSearchParams({
  device_id: 'win-Canon_GX6000_series',
  orientation: 'portrait',
  copies: 1,
  color: 'color',
  duplex: 'duplex'
});

fetch(`http://localhost:8181/api/v1/print/document?${params}`, {
  method: 'POST',
  body: formData
})
.then(res => res.json())
.then(data => console.log(data))
.catch(err => console.error(err));
```

**Python (requests):**
```python
import requests

url = "http://localhost:8181/api/v1/print/document"
params = {
    "device_id": "win-Canon_GX6000_series",
    "orientation": "portrait",
    "copies": 1,
    "color": "color",
    "duplex": "duplex"
}

with open("documento.pdf", "rb") as f:
    files = {"file": f}
    response = requests.post(url, params=params, files=files)
    print(response.json())
```

**Respuesta exitosa (200):**
```json
{
  "success": true,
  "message": "Document 'factura.pdf' sent to Canon GX6000 series",
  "file_size_bytes": 245760,
  "file_size": "240.00 KB"
}
```

**Respuestas error:**
- **404:** Device not found
- **400:** Documents only supported for Windows printers
- **500:** Windows printer support not available

**Configuración DEVMODE (Windows):**

El endpoint configura automáticamente el DEVMODE de Windows con:

```python
# Orientación
DMORIENT_PORTRAIT = 1
DMORIENT_LANDSCAPE = 2

# Color
DMCOLOR_MONOCHROME = 1  # Escala de grises
DMCOLOR_COLOR = 2        # Color

# Dúplex
DMDUP_SIMPLEX = 1        # Una cara
DMDUP_VERTICAL = 3       # Doble cara (volteo vertical/borde largo)
```

**Permisos de Windows:**
- Intenta primero con `PRINTER_ACCESS_ADMINISTER` (requiere ejecutar como admin)
- Si falla, usa `PRINTER_ACCESS_USE` (no requiere admin pero puede no aplicar todos los ajustes)
- **Recomendado:** Ejecutar servicio como Administrador en producción

**Casos de uso:**
- Imprimir facturas PDF con configuración específica
- Documentos oficiales doble cara para ahorrar papel
- Reportes Excel/Word desde sistemas externos
- Integración con sistemas de gestión documental
- Automatización de impresión de contratos/formularios

**Notas importantes:**
- ✅ **Duplex:** Usa `duplex=duplex` para imprimir 2 páginas en 1 hoja (frente y dorso)
- ⚙️ PDFs: Se imprimen directamente usando `win32print.StartDocPrinter`
- ⏱️ Tiempo de espera: 2 segundos para permitir que el trabajo se envíe al spooler
- 🗑️ Archivos temporales se eliminan automáticamente después de imprimir
- 📄 Dúplex vertical (DMDUP_VERTICAL = 3) voltea por el borde largo (estándar para documentos)

---

### 6. Listar Impresoras (Alias)

**Endpoint:** `GET /api/v1/printers`

**Descripción:** Alias de `/api/v1/devices`. Retorna la misma información pero con un nombre semánticamente más específico para APIs orientadas a impresión.

**Parámetros:** Ninguno

**Respuesta:** Idéntica a `GET /devices`

**Casos de uso:**
- Usar un nombre de endpoint más descriptivo en el código cliente
- Compatibilidad con integraciones que esperan `/printers`

---

### 7. Obtener Configuración del Sistema

**Endpoint:** `GET /api/v1/config`

**Descripción:** Obtiene la configuración actual del servicio Arya ESCPOS, incluyendo configuración de servidor, descubrimiento de dispositivos y otras opciones del sistema.

**Parámetros:** Ninguno

**Respuesta exitosa (200):**
```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8181,
    "reload": true,
    "log_level": "info"
  },
  "discovery": {
    "bluetooth_enabled": true,
    "bluetooth_scan_duration": 8,
    "usb_scan_enabled": true
  },
  "printing": {
    "default_encoding": "cp850",
    "timeout_seconds": 30
  },
  "database": {
    "path": "arya_escpos.db",
    "echo_sql": false
  }
}
```

**Casos de uso:**
- Verificar configuración activa del servicio
- Troubleshooting de problemas de conectividad
- Validar que Bluetooth está habilitado antes de escanear
- Documentación automática de la instalación

---

## 🖨️ Impresión

### 8. Imprimir Ticket ESC/POS

### DeviceResponse

Representa un dispositivo de impresión registrado en el sistema.

```typescript
{
  id: string;              // ID único: "win-<nombre>", "usb-<vid>:<pid>", "bt-<mac>"
  type: string;            // "windows" | "usb" | "bluetooth" | "network" | "serial"
  manufacturer?: string;   // Fabricante del dispositivo (opcional)
  product?: string;        // Nombre/modelo del producto (opcional)
  alias?: string;          // Nombre personalizado (opcional)
  is_active: boolean;      // true si el dispositivo está activo
}
```

**Tipos de dispositivos:**
- `windows`: Impresoras Windows instaladas (locales o red)
- `usb`: Dispositivos USB conectados directamente
- `bluetooth`: Impresoras Bluetooth emparejadas
- `network`: Impresoras de red (TCP/IP)
- `serial`: Impresoras conectadas por puerto serial/COM

### PrintRequest

```typescript
{
  device_id: string;  // ID del dispositivo
  content: string;    // Contenido de texto a imprimir
}
```

### ReportPrintRequest

```typescript
{
  device_id: string;  // ID de impresora Windows
  title: string;      // Título del reporte
  content: string;    // Contenido del reporte
}
```

### ScanResponse

```typescript
{
  found: number;           // Cantidad de dispositivos encontrados
  devices: Array<{         // Lista de dispositivos detectados
    name?: string;
    description?: string;
    type: string;
    connection_type?: string;
    vid?: string;          // USB Vendor ID
    pid?: string;          // USB Product ID
    serial?: string;       // Número de serie
    address?: string;      // Dirección MAC (Bluetooth)
  }>;
}
```

---

## ⚠️ Códigos de Error

### Errores HTTP

| Código | Descripción | Posibles causas |
|--------|-------------|-----------------|
| **400** | Bad Request | Tipo de dispositivo no soportado, parámetros inválidos |
| **404** | Not Found | Dispositivo no existe en base de datos |
| **500** | Internal Server Error | Fallo al abrir dispositivo, Windows printer no disponible, error de hardware |

### Mensajes de Error Comunes

**Device not found**
- El `device_id` no existe en la base de datos
- Solución: Ejecutar `/devices/scan` primero

**Windows printer support not available**
- pywin32 no está instalado
- Sistema operativo no es Windows
- Solución: Instalar pywin32 (`pip install pywin32`)

**Documents only supported for Windows printers**
- Intentando imprimir documento/reporte en dispositivo USB/Bluetooth
- Solución: Usar `/print` para tickets o seleccionar impresora Windows

**Reports only supported for Windows printers**
- Igual que arriba, específico para `/print/report`

**Failed to open USB device**
- Puerto USB desconectado
- Permisos insuficientes (Linux requiere udev rules)
- Dispositivo ocupado por otra aplicación

**Failed to connect to {ip}:{port}**
- Impresora de red apagada o desconectada
- IP/puerto incorrectos
- Firewall bloqueando conexión

**Access is denied (SetPrinter)**
- Servicio no ejecutándose como Administrador
- Permisos limitados en impresora compartida
- **Nota:** Este error se puede ignorar si DEVMODE se configura correctamente en memoria

---

## 🔧 Configuración Recomendada

### Para Producción

1. **Ejecutar como Administrador:**
   ```powershell
   # Ejecutar PowerShell como Administrador
   cd "C:\ruta\arya_escpos"
   python src/main.py
   ```

2. **Configurar como Servicio de Windows:**
   - Usar NSSM (Non-Sucking Service Manager)
   - Configurar para ejecutar con privilegios elevados
   - Auto-reinicio en caso de fallo

3. **Firewall:**
   - Permitir puerto 8181 (o el configurado)
   - Si impresoras de red, permitir puertos 9100, 9101, etc.

4. **Antivirus:**
   - Agregar excepción para el ejecutable Python
   - Excluir carpeta del proyecto de escaneo en tiempo real

### Para Desarrollo

```bash
# Instalar dependencias
pip install -r requirements.txt

# Ejecutar en modo desarrollo
python src/main.py

# Acceder a Swagger UI
http://localhost:8181/docs
```

---

## 📝 Ejemplos de Integración

### React/Next.js

```typescript
// services/printer.service.ts
import axios from 'axios';

const API_BASE = 'http://localhost:8181/api/v1';

export const printerService = {
  async scanDevices() {
    const { data } = await axios.get(`${API_BASE}/devices/scan`);
    return data;
  },

  async listDevices() {
    const { data } = await axios.get(`${API_BASE}/devices`);
    return data;
  },

  async printTicket(deviceId: string, content: string) {
    const { data } = await axios.post(`${API_BASE}/print`, {
      device_id: deviceId,
      content: content
    });
    return data;
  },

  async printDocument(
    deviceId: string,
    file: File,
    options: {
      orientation?: 'portrait' | 'landscape';
      copies?: number;
      color?: 'color' | 'grayscale';
      duplex?: 'simplex' | 'duplex';
    } = {}
  ) {
    const formData = new FormData();
    formData.append('file', file);

    const params = new URLSearchParams({
      device_id: deviceId,
      orientation: options.orientation || 'portrait',
      copies: String(options.copies || 1),
      color: options.color || 'color',
      duplex: options.duplex || 'simplex'
    });

    const { data } = await axios.post(
      `${API_BASE}/print/document?${params}`,
      formData,
      {
        headers: { 'Content-Type': 'multipart/form-data' }
      }
    );
    return data;
  }
};
```

### Python

```python
import requests

class AryaESCPOS:
    def __init__(self, base_url="http://localhost:8181/api/v1"):
        self.base_url = base_url
    
    def scan_devices(self):
        """Escanear dispositivos"""
        response = requests.get(f"{self.base_url}/devices/scan")
        return response.json()
    
    def list_devices(self):
        """Listar dispositivos"""
        response = requests.get(f"{self.base_url}/devices")
        return response.json()
    
    def print_ticket(self, device_id: str, content: str):
        """Imprimir ticket ESC/POS"""
        response = requests.post(
            f"{self.base_url}/print",
            json={"device_id": device_id, "content": content}
        )
        return response.json()
    
    def print_document(
        self,
        device_id: str,
        file_path: str,
        orientation: str = "portrait",
        copies: int = 1,
        color: str = "color",
        duplex: str = "simplex"
    ):
        """Imprimir documento con configuración"""
        params = {
            "device_id": device_id,
            "orientation": orientation,
            "copies": copies,
            "color": color,
            "duplex": duplex
        }
        
        with open(file_path, "rb") as f:
            files = {"file": f}
            response = requests.post(
                f"{self.base_url}/print/document",
                params=params,
                files=files
            )
        
        return response.json()

# Uso
printer = AryaESCPOS()

# Escanear impresoras
result = printer.scan_devices()
print(f"Encontradas: {result['found']} impresoras")

# Imprimir PDF doble cara
result = printer.print_document(
    device_id="win-Canon_GX6000_series",
    file_path="factura.pdf",
    duplex="duplex",
    color="color"
)
print(result['message'])
```

---

## 🆘 Troubleshooting

### Problema: No se encuentran impresoras Windows

**Solución:**
1. Verificar que pywin32 esté instalado: `pip install pywin32`
2. Verificar que las impresoras estén instaladas en Windows
3. Ejecutar `/devices/scan`

### Problema: Error "Access is denied" al imprimir

**Solución:**
1. Ejecutar servicio como Administrador
2. Verificar permisos de la impresora en Windows
3. El error se puede ignorar si la impresión funciona correctamente

### Problema: Dúplex no funciona

**Solución:**
1. Verificar que la impresora soporte dúplex (doble cara)
2. Ejecutar servicio como Administrador para permisos completos
3. Usar `duplex=duplex` en el request
4. Verificar logs: debe mostrar "Duplex=3 (DMDUP_VERTICAL)"

### Problema: PDF imprime en blanco y negro en lugar de color

**Solución:**
1. Verificar que `color=color` en el request
2. Verificar que el PDF contenga contenido a color
3. Verificar configuración predeterminada de impresora en Windows
4. Ejecutar servicio como Administrador

### Problema: Impresora USB no detectada

**Solución:**
1. Verificar que el cable USB esté conectado
2. Verificar en Administrador de dispositivos (Windows)
3. Probar otro puerto USB
4. Instalar drivers del fabricante

---

## 📞 Soporte

**Repositorio:** [GitHub - Arya ESCPOS](https://github.com/tu-repo/arya-escpos)  
**Documentación adicional:** `/docs`  
**Swagger UI:** `http://localhost:8181/docs`  
**Versión:** 1.0.0  
**Licencia:** MIT

---

*Última actualización: 26 de Enero, 2026*
