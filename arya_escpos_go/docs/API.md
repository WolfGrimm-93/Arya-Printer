# API — Arya Printer Service

Base URL: `http://127.0.0.1:58181` (o `https://` si HTTPS local esta configurado — ver README, seccion "HTTPS en localhost"). Host/puerto vienen de `configs/settings.yaml` (`server.host`/`server.port`).

Contrato fuente: `internal/contract/dto.go`, `internal/contract/services.go` y `internal/apiserver/*.go`. Este documento describe el comportamiento real de esos archivos, no un diseño aspiracional.

## Autenticacion

Todas las rutas `/api/v1/*` (los 9 endpoints de abajo) requieren el header:

```
X-API-Key: <api key de secrets/apikey.key>
```

Sin el header, o con una key invalida: `401 Unauthorized`, cuerpo `{"detail": "missing or invalid API key"}`. Se puede desactivar con `security.auth_enabled: false` en `settings.yaml` (solo para desarrollo).

`GET /` y `GET /health` son las UNICAS rutas exentas — no autenticadas, para chequeos de liveness/monitoreo — y no forman parte del contrato de 9 endpoints versionado bajo `/api/v1`.

## CORS

Cualquier origen recibe los headers `Access-Control-Allow-*` (se hace eco del `Origin` recibido, nunca un `*` literal) — no hay whitelist de dominios que mantener. El control de acceso real es el header `X-API-Key`: una página que no conoce la key de esa instalación no puede autenticar sus requests, tenga o no CORS de por medio. Sin `Origin` header (curl, server-to-server) no aplica. `Access-Control-Allow-Private-Network` nunca se emite. Ver `internal/middleware/cors.go` para el razonamiento completo.

## Formato de error

Cualquier error de un handler se devuelve como:

```json
{"detail": "mensaje legible"}
```

con el status HTTP correspondiente:

| Status | Cuando |
|---|---|
| 400 Bad Request | Input del cliente invalido (JSON malformado, campo faltante, hex invalido, tipo no soportado). |
| 401 Unauthorized | Falta o es invalida `X-API-Key`. |
| 403 Forbidden | Bloqueado por una politica de seguridad (`internal/netguard` sobre `type=network`). |
| 404 Not Found | Dispositivo/trabajo/impresora referenciado no existe. |
| 413 Payload Too Large | Upload o payload decodificado excede el limite configurado (`security.max_upload_bytes`/`max_image_bytes`). |
| 502 Bad Gateway | Fallo hablando con hardware o un proceso externo (USB/serial/TCP, SumatraPDF/PDFtoPrinter, LibreOffice). |
| 503 Service Unavailable | Falta un prerequisito del entorno (LibreOffice no instalado, Sumatra/PDFtoPrinter no encontrado, subsistema de impresion de Windows no disponible). |
| 500 Internal Server Error | Cualquier otro error — nunca expone detalles internos (paths, stack, mensajes de driver) en el cuerpo de la respuesta. |

---

## Health (sin autenticacion, fuera de los 9 endpoints)

| Metodo | Ruta | Descripcion |
|---|---|---|
| GET | `/` | Info del servicio (`service`, `version`, `status`, `docs`). |
| GET | `/health` | Health check (`{"status": "healthy"}`). |

---

## Los 9 endpoints de `/api/v1`

| # | Metodo | Ruta |
|---|---|---|
| 1 | GET | `/api/v1/devices/scan` |
| 2 | GET | `/api/v1/devices/{type}/{id}/status` |
| 3 | GET | `/api/v1/devices/windows/{printer}/jobs/{job_id}` |
| 4 | POST | `/api/v1/print` |
| 5 | POST | `/api/v1/print/matrix` |
| 6 | POST | `/api/v1/print/report` |
| 7 | POST | `/api/v1/print/document` |
| 8 | GET | `/api/v1/print/history` |
| 9 | GET | `/api/v1/config` |

### 1. Escanear dispositivos

**GET** `/api/v1/devices/scan`

Escanea en vivo los transportes habilitados (`discovery.usb_enabled`/`network_enabled`/`serial_enabled` en la config), mas las impresoras Windows instaladas. Sin bluetooth (fuera de alcance de esta reescritura).

**Respuesta 200:**
```json
{
  "found": 3,
  "devices": [
    { "id": "win-HP_LaserJet", "type": "windows", "name": "HP LaserJet", "manufacturer": "HP", "description": "local" },
    { "id": "usb-04b8:0202", "type": "usb", "name": "TM-T20", "manufacturer": "EPSON", "vid": "04b8", "pid": "0202" },
    { "id": "serial-COM3", "type": "serial", "name": "COM3", "com_port": "COM3" }
  ]
}
```

### 2. Estado de un dispositivo

**GET** `/api/v1/devices/{type}/{id}/status`

`type` es uno de `windows`, `usb`, `network`, `serial`.

> **`type=network`**: a diferencia del servicio Python (donde este endpoint era un oraculo SSRF — cualquier `host:port` se intentaba conectar sin validacion), acá `internal/netguard` valida el destino contra el allowlist de `network_scan` (`subnets`/`ports` en `settings.yaml`) **antes** de intentar la conexion. Un destino fuera del allowlist responde `403 Forbidden`, no un intento de conexion silencioso.

Ejemplos de ruta: `/devices/windows/HP_LaserJet/status`, `/devices/usb/04b8:0202/status`, `/devices/network/192.168.1.100:9100/status`, `/devices/serial/COM3/status`.

**Respuesta 200 (windows):**
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

Campos no aplicables al `device_type` consultado se omiten del JSON (`omitempty`) en vez de aparecer en `null`/`0`.

### 3. Estado de un trabajo de impresion

**GET** `/api/v1/devices/windows/{printer}/jobs/{job_id}`

`{printer}` usa guion bajo en vez de espacio en la URL (`HP_LaserJet`); el servicio lo traduce de vuelta a espacio antes de consultar el spooler. `{job_id}` debe ser un entero — de lo contrario `400 Bad Request` (`invalid_job_id`).

**Respuesta 200:**
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

Estados posibles (`status_code`): `0` queued, `1` paused, `2` error, `3` deleting, `4` spooling, `5` printing, `6` printed.

### 4. Imprimir ticket ESC/POS (termicas)

**POST** `/api/v1/print`

Impresoras compatibles: Epson TM-T20/T88/T70, CUSTOM P3L/Q3X, Star TSP100/TSP650, Bixolon SRP-350, Citizen CT-S310II — cualquier termica ESC/POS de 58 mm u 80 mm.

| Campo | Tipo | Requerido | Notas |
|---|---|---|---|
| `type` | string | Si | `windows`, `usb`, `network`, `serial`. `bluetooth` se acepta por compatibilidad de esquema pero responde `400 unsupported_type` — no soportado en esta version. |
| `content` | string | Si | Texto a imprimir. |
| `header_image` | string | No | Imagen/logo en base64, se imprime antes del contenido. Sujeto a `security.max_image_bytes` una vez decodificada. |
| `image_width` | int | No | Ancho maximo en dots, default `576` (80 mm). |
| `printer_name` | string | Si*, para `type=windows` | |
| `vid`, `pid` | string (hex) | Si*, para `type=usb` | Formato hex (ej. `"04b8"`); invalido -> `400 invalid_hex`. A diferencia del servicio Python (bug conocido: vid/pid nunca llegaban al adaptador USB, siempre auto-detectaba), acá sí se propagan y se usan para seleccionar el dispositivo. |
| `ip` | string | Si*, para `type=network` | `port` default `9100`. Validado contra `network_scan` antes de conectar. |
| `com_port` | string | Si*, para `type=serial` | |

**Respuesta 200:**
```json
{ "success": true, "message": "Print job sent (windows)", "bytes_sent": 245 }
```

Campo faltante segun `type` -> `400 missing_field` (ej. `type=usb` sin `vid`/`pid`).

### 5. Imprimir texto plano matricial (ESC/P)

**POST** `/api/v1/print/matrix`

Impresoras compatibles: Epson LX-350/LX-300+II/LX-810/FX-890II/DFX-9000, Oki Microline 320/390 — cualquier matricial de 9/24 pines con ESC/P. Sin red, sin bluetooth (`type`: solo `windows`, `usb`, `serial`).

| Campo | Tipo | Default | Notas |
|---|---|---|---|
| `type` | string | — | `windows`, `usb`, `serial`. Requerido. |
| `content` | string | — | Requerido. |
| `encoding` | string | `cp850` | |
| `form_feed` | bool | `true` | Avanza pagina (FF) al terminar. |
| `font` | string | `roman` | `roman` o `sans_serif`; otro valor -> `400 invalid_field`. |
| `cpi` | int | `10` | `10`, `12` o `15`; otro valor -> `400 invalid_field`. |
| `barcodes` | array | `[]` | Ver abajo. |
| `printer_name` | string | — | Requerido si `type=windows`. |
| `vid`, `pid` | string (hex) | — | Requeridos si `type=usb`. |
| `com_port` | string | — | Requerido si `type=serial`. |
| `baud_rate` | int | `9600` | A diferencia del servicio Python (bug conocido: `baud_rate` nunca llegaba al puerto serial, siempre corria al default del constructor), acá sí se aplica. |

Cada elemento de `barcodes`:

| Campo | Tipo | Default |
|---|---|---|
| `data` | string | requerido |
| `type` | string | `code39` (`code39`, `code128`, `ean13`, `ean8`, `upca`, `upc`) |
| `width_dots` | int | `300` |
| `height_dots` | int | `48` |

Un tipo de codigo de barras invalido/no soportado **no** rechaza el request: cae a una etiqueta de texto plano `[TIPO: data]`, comportamiento heredado a proposito de `matrix_command_builder.py`.

**Respuesta 200:**
```json
{ "success": true, "message": "Matrix print job sent (windows)", "bytes_sent": 48 }
```

### 6. Imprimir reporte formateado

**POST** `/api/v1/print/report`

Envia texto plano RAW al spooler de Windows (no ESC/POS), con titulo centrado y separadores — cualquier impresora con driver de Windows.

```json
{ "printer_name": "HP LaserJet", "title": "Reporte de Ventas", "content": "Contenido del reporte..." }
```

**Respuesta 200:**
```json
{ "success": true, "message": "Report sent to HP LaserJet" }
```

### 7. Imprimir documento

**POST** `/api/v1/print/document`

`multipart/form-data` con un campo `file`; parametros por query string.

```
POST /api/v1/print/document?printer_name=HP_LaserJet&orientation=portrait&copies=1&color=color&duplex=simplex
Content-Type: multipart/form-data

file: [archivo]
```

| Query param | Default | Valores |
|---|---|---|
| `printer_name` | — | Requerido -> `400 missing_field` si falta. |
| `orientation` | `portrait` | `portrait`, `landscape` |
| `copies` | `1` | entero, se clampea a >= 1 |
| `color` | `color` | `color`, `grayscale` |
| `duplex` | `simplex` | `simplex`, `duplex` |

Convierte automaticamente a PDF (DOCX, XLSX, etc.) antes de imprimir. El archivo subido se guarda en un temporal que el servicio siempre borra (exito o error); si la conversion genera un PDF intermedio, ese archivo lo administra y limpia el propio conversor. Sin `file` -> `400 missing_file`. Excede `security.max_upload_bytes` -> `413`.

**Respuesta 200:**
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

`job_id`/`track_url` solo estan presentes si el trabajo se pudo asociar a una entrada de la cola de Windows.

### 8. Historial de impresion

**GET** `/api/v1/print/history`

En memoria, maximo 500 entradas, mas reciente primero, se reinicia con el servicio (sin persistencia).

| Query param | Default | Notas |
|---|---|---|
| `limit` | `50` | Se clampea a `[1, 500]`. |
| `printer_name` | (ninguno) | Filtro exacto, **case-insensitive** — no es un `contains`. |

```bash
curl -H "X-API-Key: $KEY" "http://127.0.0.1:58181/api/v1/print/history?printer_name=Epson+LX-350&limit=20"
```

**Respuesta 200:**
```json
{
  "total": 1,
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
    }
  ]
}
```

`protocol` es uno de `escpos`, `escp`, `document`, `report`.

### 9. Configuracion activa

**GET** `/api/v1/config`

Snapshot de la configuracion en efecto, en `snake_case`, con **`security.api_key_path` siempre omitido** (es una ruta de archivo local, no la key en si, pero igual no se expone por HTTP) — a diferencia del servicio Python, que devolvia el dict de config completo tal cual.

```json
{
  "server": { "host": "127.0.0.1", "port": 58181 },
  "logging": { "level": "INFO", "log_dir": "logs", "retention_days": 15 },
  "discovery": { "auto_scan_interval": 30, "usb_enabled": true, "network_enabled": true, "serial_enabled": true },
  "devices": { "auto_reconnect": true, "connection_timeout": 5, "retry_attempts": 3 },
  "printers": { "paper_width": 80 },
  "network_scan": { "enabled": true, "subnets": ["192.168.1.0/24"], "ports": [9100, 9101], "timeout": 2 },
  "security": { "auth_enabled": true, "max_upload_bytes": 52428800, "max_image_bytes": 5242880 }
}
```
