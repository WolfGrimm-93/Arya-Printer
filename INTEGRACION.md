# Guia de Integracion: Arya ESCPOS Service

Este documento describe como integrar una aplicacion cliente con el servicio Arya ESCPOS para imprimir tickets, reportes y documentos.

## Requisitos

- Arya ESCPOS Service instalado y corriendo en la PC del cliente
- El servicio escucha en `http://localhost:58181`
- No requiere autenticacion

## Base URL

```
http://localhost:58181/api/v1
```

---

## 1. Verificar que el servicio esta activo

Antes de intentar imprimir, verificar que el servicio responde.

### Endpoint

```
GET /health
```

### Ejemplo (JavaScript/Fetch)

```javascript
async function isAryaPrinterAvailable() {
  try {
    const response = await fetch('http://localhost:58181/health', {
      signal: AbortSignal.timeout(3000)
    });
    return response.ok;
  } catch {
    return false;
  }
}
```

### Respuesta esperada

```json
{
  "status": "ok",
  "service": "Arya ESCPOS Service"
}
```

---

## 2. Escanear impresoras disponibles

Descubre todas las impresoras conectadas a la PC del cliente.

### Endpoint

```
GET /api/v1/devices/scan
```

### Para que sirve

- Mostrar al usuario una lista de impresoras para que elija
- Detectar automaticamente impresoras ESC/POS (tickets) e impresoras normales (documentos)
- No requiere parametros

### Ejemplo (JavaScript/Fetch)

```javascript
async function getAvailablePrinters() {
  const response = await fetch('http://localhost:58181/api/v1/devices/scan');
  const data = await response.json();
  return data.devices;
}

// Uso
const printers = await getAvailablePrinters();
// Filtrar solo impresoras Windows (las mas comunes)
const windowsPrinters = printers.filter(p => p.type === 'windows');
```

### Respuesta esperada

```json
{
  "found": 2,
  "devices": [
    {
      "id": "win-CUSTOM_P3L",
      "type": "windows",
      "name": "CUSTOM P3L",
      "manufacturer": "CUSTOM P3L,CUSTOM P3L,",
      "description": "network",
      "vid": null,
      "pid": null,
      "serial": null,
      "ip": null,
      "port": null,
      "address": null,
      "channel": null,
      "com_port": null
    },
    {
      "id": "win-Canon_GX6000_series",
      "type": "windows",
      "name": "Canon GX6000 series",
      "manufacturer": "Canon GX6000 series,Canon GX6000 series,",
      "description": "network",
      "vid": null,
      "pid": null,
      "serial": null,
      "ip": null,
      "port": null,
      "address": null,
      "channel": null,
      "com_port": null
    }
  ]
}
```

### Como implementarlo en tu aplicacion

1. Al abrir la seccion de impresion, llamar a este endpoint
2. Guardar el `name` de la impresora seleccionada por el usuario en la configuracion local
3. Usar ese `name` como `printer_name` en las llamadas de impresion

---

## 3. Imprimir ticket ESC/POS (impresoras termicas)

Envia texto plano a una impresora termica. El servicio lo envuelve en comandos ESC/POS (init + encode + cut).

**Impresoras compatibles:** Epson TM-T20, TM-T88, TM-T70, CUSTOM P3L/Q3X, Star TSP100/TSP650, Bixolon SRP-350, Citizen CT-S310II. Cualquier termica ESC/POS de 58mm u 80mm.

### Endpoint

```
POST /api/v1/print
Content-Type: application/json
```

### Para que sirve

- Imprimir tickets de venta
- Imprimir comandas de cocina
- Imprimir recibos
- Cualquier impresion en impresora termica de tickets

### Body (JSON)

| Campo          | Tipo   | Requerido | Descripcion                          |
|----------------|--------|-----------|--------------------------------------|
| `type`         | string | Si        | Tipo de conexion (ver tabla abajo)   |
| `content`      | string | Si        | Texto a imprimir (con \n para saltos)|
| `header_image` | string | No        | Logo en base64 (se imprime antes del contenido) |
| `image_width`  | int    | No        | Ancho maximo en dots (default 576 para 80mm) |
| `printer_name` | string | Si*       | Nombre de la impresora Windows       |
| `vid`          | string | Si*       | Vendor ID hex (solo para USB)        |
| `pid`          | string | Si*       | Product ID hex (solo para USB)       |
| `ip`           | string | Si*       | IP de la impresora (solo network)    |
| `port`         | int    | No        | Puerto TCP (default 9100)            |
| `com_port`     | string | Si*       | Puerto COM (solo serial)             |
| `address`      | string | Si*       | MAC address (solo bluetooth)         |

*Requerido segun el `type` elegido.

### Campos requeridos segun tipo

| type        | Campos requeridos         |
|-------------|---------------------------|
| `windows`   | `printer_name`            |
| `usb`       | `vid`, `pid`              |
| `network`   | `ip` (port default 9100)  |
| `serial`    | `com_port`                |
| `bluetooth` | `address`                 |

### Ejemplo - Impresora Windows (caso mas comun)

```javascript
async function printTicket(printerName, content) {
  const response = await fetch('http://localhost:58181/api/v1/print', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      type: 'windows',
      printer_name: printerName,
      content: content
    })
  });
  return await response.json();
}

// Uso - Ticket de venta
const ticketContent = [
  '        MI NEGOCIO',
  '        ==========',
  '',
  'Fecha: 2026-03-13 10:30',
  'Cajero: Maria',
  '--------------------------------',
  'Producto         Cant   Precio',
  '--------------------------------',
  'Cafe Americano     2    $5.00',
  'Pan Integral       1    $3.50',
  '--------------------------------',
  'TOTAL:                   $13.50',
  '================================',
  '',
  '   Gracias por su compra!',
  ''
].join('\n');

const result = await printTicket('CUSTOM P3L', ticketContent);
// { success: true, message: "Print job sent (windows)", bytes_sent: 245 }
```

### Ejemplo - Impresora USB directa

```javascript
const result = await fetch('http://localhost:58181/api/v1/print', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    type: 'usb',
    vid: '04b8',
    pid: '0202',
    content: 'Ticket USB directo\n'
  })
});
```

### Ejemplo - Impresora de red

```javascript
const result = await fetch('http://localhost:58181/api/v1/print', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    type: 'network',
    ip: '192.168.1.100',
    port: 9100,
    content: 'Ticket por red\n'
  })
});
```

### Respuesta exitosa

```json
{
  "success": true,
  "message": "Print job sent (windows)",
  "bytes_sent": 245
}
```

### Respuesta de error (campos faltantes)

```json
{
  "detail": [
    {
      "type": "value_error",
      "msg": "Value error, printer_name is required for type='windows'"
    }
  ]
}
```

---

---

## 4. Imprimir en impresora matricial (ESC/P)

Envia texto plano a una impresora matricial de impacto usando el protocolo ESC/P.

**Impresoras compatibles:** Epson LX-350, LX-300+II, LX-810, FX-890II, DFX-9000. Oki Microline 320/390. Cualquier matricial de 9 o 24 pines con protocolo ESC/P.

### Endpoint

```
POST /api/v1/print/matrix
Content-Type: application/json
```

### Para que sirve

- Imprimir formularios continuos (facturas, comprobantes)
- Imprimir en papel multipartes (original + copias)
- Sistemas de facturacion con impresoras antiguas industriales
- Bancos y entidades con equipos matriciales

### Body (JSON)

| Campo          | Tipo   | Requerido | Default  | Descripcion                                |
|----------------|--------|-----------|----------|--------------------------------------------|
| `type`         | string | Si        | -        | `windows`, `usb`, o `serial`               |
| `content`      | string | Si        | -        | Texto a imprimir                           |
| `encoding`     | string | No        | `cp850`  | Codificacion del texto                     |
| `form_feed`    | bool   | No        | `true`   | Avanzar pagina al terminar                 |
| `printer_name` | string | Si*       | -        | Nombre Windows (si type=windows)           |
| `vid`          | string | Si*       | -        | Vendor ID hex (si type=usb)                |
| `pid`          | string | Si*       | -        | Product ID hex (si type=usb)               |
| `com_port`     | string | Si*       | -        | Puerto COM (si type=serial)                |
| `baud_rate`    | int    | No        | `9600`   | Baudrate (si type=serial)                  |

### Ejemplo - Via Windows driver (recomendado)

```javascript
async function printMatrixTicket(printerName, content) {
  const response = await fetch('http://localhost:58181/api/v1/print/matrix', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      type: 'windows',
      printer_name: printerName,
      content: content
    })
  });
  return await response.json();
}

// Uso - Factura en matricial
const factura = [
  'EMPRESA S.A.',
  'RUC: 20123456789',
  '================================',
  'FACTURA #001-0001234',
  'Fecha: 2026-04-10',
  '--------------------------------',
  'Producto         Cant   Precio',
  '--------------------------------',
  'Articulo A          2   $50.00',
  'Articulo B          1   $30.00',
  '--------------------------------',
  'TOTAL:                  $130.00',
  '================================',
].join('\n');

const result = await printMatrixTicket('Epson LX-350', factura);
// { success: true, message: "Matrix print job sent (windows)", bytes_sent: 312 }
```

### Ejemplo - Via serial RS-232

```javascript
const result = await fetch('http://localhost:58181/api/v1/print/matrix', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    type: 'serial',
    com_port: 'COM1',
    baud_rate: 9600,
    content: 'FACTURA #001\nTotal: $100.00',
    form_feed: true
  })
});
```

### Respuesta exitosa

```json
{
  "success": true,
  "message": "Matrix print job sent (windows)",
  "bytes_sent": 312
}
```

---

## 5. Imprimir reporte de texto

Envia texto formateado (UTF-8) directo al spooler de Windows. No usa comandos ESC/POS. Ideal para reportes en impresoras normales.

**Impresoras compatibles:** Cualquier impresora con driver de Windows (laser, tinta, matriciales).

### Endpoint

```
POST /api/v1/print/report
Content-Type: application/json
```

### Para que sirve

- Imprimir reportes de ventas
- Imprimir listas de inventario
- Cualquier texto plano en impresora normal (no termica)

### Body (JSON)

| Campo          | Tipo   | Requerido | Descripcion                   |
|----------------|--------|-----------|-------------------------------|
| `printer_name` | string | Si        | Nombre de la impresora        |
| `title`        | string | Si        | Titulo del reporte (centrado) |
| `content`      | string | Si        | Contenido del reporte         |

### Ejemplo (JavaScript/Fetch)

```javascript
async function printReport(printerName, title, content) {
  const response = await fetch('http://localhost:58181/api/v1/print/report', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      printer_name: printerName,
      title: title,
      content: content
    })
  });
  return await response.json();
}

// Uso
const result = await printReport(
  'Canon GX6000 series',
  'REPORTE DE VENTAS - MARZO 2026',
  'Ventas totales: 450\nMonto total: $12,500.00\nTicket promedio: $27.78'
);
// { success: true, message: "Report sent to Canon GX6000 series" }
```

### Respuesta exitosa

```json
{
  "success": true,
  "message": "Report sent to Canon GX6000 series"
}
```

---

## 5. Imprimir documento (PDF, Word, Excel, imagen, texto)

Sube un archivo y lo imprime en una impresora Windows. Convierte automaticamente a PDF si es necesario.

### Endpoint

```
POST /api/v1/print/document
Content-Type: multipart/form-data
```

### Para que sirve

- Imprimir facturas en PDF
- Imprimir documentos Word/Excel
- Imprimir imagenes (JPG, PNG, etc.)
- Cualquier archivo que necesite imprimirse "tal cual"

### Parametros (query string)

| Parametro      | Tipo   | Requerido | Default    | Descripcion                   |
|----------------|--------|-----------|------------|-------------------------------|
| `printer_name` | string | Si        | -          | Nombre de la impresora        |
| `orientation`  | string | No        | `portrait` | `portrait` o `landscape`      |
| `copies`       | int    | No        | `1`        | Numero de copias              |
| `color`        | string | No        | `color`    | `color` o `grayscale`         |
| `duplex`       | string | No        | `simplex`  | `simplex` o `duplex`          |

### Body (multipart/form-data)

| Campo  | Tipo | Requerido | Descripcion        |
|--------|------|-----------|--------------------|
| `file` | File | Si        | Archivo a imprimir |

### Formatos soportados

| Extension                        | Tipo    | Conversion         |
|----------------------------------|---------|---------------------|
| `.pdf`                           | PDF     | Directo (sin conversion) |
| `.doc`, `.docx`                  | Word    | docx2pdf            |
| `.xls`, `.xlsx`                  | Excel   | COM automation      |
| `.jpg`, `.jpeg`, `.png`, `.bmp`, `.gif`, `.tiff` | Imagen | Pillow |
| `.txt`, `.log`, `.csv`           | Texto   | PyMuPDF             |

### Ejemplo (JavaScript/Fetch)

```javascript
async function printDocument(printerName, file, options = {}) {
  const formData = new FormData();
  formData.append('file', file);

  const params = new URLSearchParams({
    printer_name: printerName,
    orientation: options.orientation || 'portrait',
    copies: options.copies || 1,
    color: options.color || 'color',
    duplex: options.duplex || 'simplex'
  });

  const response = await fetch(
    `http://localhost:58181/api/v1/print/document?${params}`,
    { method: 'POST', body: formData }
  );
  return await response.json();
}

// Uso con input file de HTML
const fileInput = document.getElementById('fileInput');
const file = fileInput.files[0];

const result = await printDocument('Canon GX6000 series', file, {
  orientation: 'portrait',
  copies: 2,
  color: 'color',
  duplex: 'simplex'
});
// {
//   success: true,
//   message: "Document 'factura.pdf' sent to Canon GX6000 series",
//   file_size_bytes: 125432,
//   file_size: "122.49 KB",
//   job_id: 123,
//   track_url: "/api/v1/devices/windows/Canon_GX6000_series/jobs/123"
// }
```

### Ejemplo con Blob (generar PDF desde frontend)

```javascript
// Si generas un PDF en el frontend (por ejemplo con jsPDF)
const pdfBlob = doc.output('blob');
const file = new File([pdfBlob], 'factura.pdf', { type: 'application/pdf' });

const result = await printDocument('Canon GX6000 series', file);
```

### Respuesta exitosa

```json
{
  "success": true,
  "message": "Document 'factura.pdf' sent to Canon GX6000 series",
  "file_size_bytes": 125432,
  "file_size": "122.49 KB",
  "job_id": 123,
  "track_url": "/api/v1/devices/windows/Canon_GX6000_series/jobs/123"
}
```

---

## 6. Rastrear un trabajo de impresion

Obtiene el estado actual de un trabajo en la cola.

### Endpoint

```
GET /api/v1/devices/windows/{printer_name}/jobs/{job_id}
```

### Para que sirve

- Verificar el progreso de una impresion
- Saber si ya se imprimi, se esta imprimiendo, o esta en cola
- Mostrar numero de paginas impresas

### Ejemplo (JavaScript/Fetch)

```javascript
async function getJobStatus(printerName, jobId) {
  const encodedName = printerName.replace(/ /g, '_');
  const response = await fetch(
    `http://localhost:58181/api/v1/devices/windows/${encodedName}/jobs/${jobId}`
  );
  return await response.json();
}

// Uso - Usar el job_id obtenido al imprimir
const jobStatus = await getJobStatus('Canon GX6000 series', 123);
console.log(jobStatus.status); // "printing", "printed", "queued", etc
```

### Respuesta exitosa

```json
{
  "job_id": 123,
  "printer_name": "Canon GX6000 series",
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

### Estados posibles

- `queued` (0) - Trabajo en cola esperando
- `paused` (1) - Pausado
- `error` (2) - Error en la impresion
- `deleting` (3) - Se esta eliminando
- `spooling` (4) - Siendo enviado al spooler
- `printing` (5) - Imprimiendo actualmente
- `printed` (6) - Ya se imprimi correctamente

---

## 8. Consultar estado de una impresora

Verifica si una impresora esta en linea y su estado actual.

### Endpoint

```
GET /api/v1/devices/{type}/{id}/status
```

### Para que sirve

- Verificar que la impresora esta lista antes de imprimir
- Mostrar estado al usuario (online/offline, errores, trabajos en cola)

### Ejemplo (JavaScript/Fetch)

```javascript
async function getPrinterStatus(printerName) {
  const encodedName = printerName.replace(/ /g, '_');
  const response = await fetch(
    `http://localhost:58181/api/v1/devices/windows/${encodedName}/status`
  );
  return await response.json();
}

// Uso
const status = await getPrinterStatus('CUSTOM P3L');
// {
//   device_type: "windows",
//   printer_name: "CUSTOM P3L",
//   is_online: true,
//   status: "Ready",
//   status_code: 0,
//   errors: [],
//   warnings: [],
//   details: [],
//   jobs_in_queue: 0
// }
```

### Importante

- Para impresoras Windows: reemplazar espacios por `_` en el nombre
- El campo `is_online` es `true` si no hay errores criticos
- `errors` contiene problemas que impiden imprimir (offline, sin papel, atasco)
- `warnings` contiene problemas menores (toner bajo, intervencion del usuario)

---

## 9. Consultar configuracion del servicio

### Endpoint

```
GET /api/v1/config
```

### Para que sirve

- Health check avanzado (verificar que el servicio esta configurado correctamente)
- Obtener el puerto y host configurados
- Ver que protocolos de scan estan habilitados

### Ejemplo

```javascript
const config = await fetch('http://localhost:58181/api/v1/config').then(r => r.json());
// Verificar que el servicio esta corriendo en el puerto esperado
console.log(config.server.port); // 58181
```

---

## Implementacion recomendada

### 1. Servicio de impresion (service/printer.js o similar)

```javascript
const ARYA_PRINTER_URL = 'http://localhost:58181';

class AryaPrinterService {

  // Verificar disponibilidad
  async isAvailable() {
    try {
      const res = await fetch(`${ARYA_PRINTER_URL}/health`, {
        signal: AbortSignal.timeout(3000)
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  // Obtener impresoras
  async getPrinters() {
    const res = await fetch(`${ARYA_PRINTER_URL}/api/v1/devices/scan`);
    const data = await res.json();
    return data.devices;
  }

  // Imprimir ticket ESC/POS (termicas: Epson TM, CUSTOM, Star, Bixolon)
  async printTicket(printerName, content) {
    const res = await fetch(`${ARYA_PRINTER_URL}/api/v1/print`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'windows',
        printer_name: printerName,
        content: content
      })
    });
    if (!res.ok) throw new Error(`Print failed: ${res.status}`);
    return await res.json();
  }

  // Imprimir en matricial ESC/P (Epson LX-350, FX-890, Oki Microline)
  async printMatrix(printerName, content, options = {}) {
    const res = await fetch(`${ARYA_PRINTER_URL}/api/v1/print/matrix`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'windows',
        printer_name: printerName,
        content: content,
        encoding: options.encoding || 'cp850',
        form_feed: options.form_feed !== undefined ? options.form_feed : true
      })
    });
    if (!res.ok) throw new Error(`Matrix print failed: ${res.status}`);
    return await res.json();
  }

  // Imprimir reporte
  async printReport(printerName, title, content) {
    const res = await fetch(`${ARYA_PRINTER_URL}/api/v1/print/report`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        printer_name: printerName,
        title: title,
        content: content
      })
    });
    if (!res.ok) throw new Error(`Report print failed: ${res.status}`);
    return await res.json();
  }

  // Imprimir documento/archivo
  async printDocument(printerName, file, options = {}) {
    const formData = new FormData();
    formData.append('file', file);
    const params = new URLSearchParams({
      printer_name: printerName,
      orientation: options.orientation || 'portrait',
      copies: options.copies || 1,
      color: options.color || 'color',
      duplex: options.duplex || 'simplex'
    });
    const res = await fetch(
      `${ARYA_PRINTER_URL}/api/v1/print/document?${params}`,
      { method: 'POST', body: formData }
    );
    if (!res.ok) throw new Error(`Document print failed: ${res.status}`);
    return await res.json();
  }

  // Estado de impresora
  async getStatus(printerName) {
    const encoded = printerName.replace(/ /g, '_');
    const res = await fetch(
      `${ARYA_PRINTER_URL}/api/v1/devices/windows/${encoded}/status`
    );
    return await res.json();
  }
}

export const aryaPrinter = new AryaPrinterService();
```

### 2. Uso desde componentes

```javascript
import { aryaPrinter } from './services/printer';

// Verificar servicio al cargar la app
const printerAvailable = await aryaPrinter.isAvailable();
if (!printerAvailable) {
  showWarning('Servicio de impresion no detectado. Instale Arya ESCPOS Service.');
}

// Selector de impresora
const printers = await aryaPrinter.getPrinters();
const windowsPrinters = printers.filter(p => p.type === 'windows');
// Mostrar dropdown con windowsPrinters.map(p => p.name)

// Imprimir ticket de venta
await aryaPrinter.printTicket(selectedPrinter, ticketText);

// Imprimir factura PDF
await aryaPrinter.printDocument(selectedPrinter, pdfFile, { copies: 2 });
```

### 3. Manejo de errores

```javascript
try {
  await aryaPrinter.printTicket('CUSTOM P3L', ticketContent);
  showSuccess('Ticket impreso correctamente');
} catch (error) {
  if (error.message.includes('Failed to fetch')) {
    showError('No se puede conectar con el servicio de impresion');
  } else {
    showError(`Error al imprimir: ${error.message}`);
  }
}
```

---

## Resumen de endpoints

| Metodo | Endpoint                                          | Uso                        | Impresoras                          | Content-Type         |
|--------|---------------------------------------------------|----------------------------|-------------------------------------|----------------------|
| GET    | `/health`                                         | Health check               | -                                   | -                    |
| GET    | `/api/v1/devices/scan`                            | Listar impresoras          | -                                   | -                    |
| GET    | `/api/v1/devices/{type}/{id}/status`              | Estado de impresora        | -                                   | -                    |
| GET    | `/api/v1/devices/windows/{printer}/jobs/{job_id}` | Estado de un trabajo       | -                                   | -                    |
| GET    | `/api/v1/config`                                  | Configuracion del servicio | -                                   | -                    |
| POST   | `/api/v1/print`                                   | Ticket ESC/POS             | Epson TM, CUSTOM, Star, Bixolon     | application/json     |
| POST   | `/api/v1/print/matrix`                            | Texto plano ESC/P          | Epson LX-350/FX-890, Oki Microline  | application/json     |
| POST   | `/api/v1/print/report`                            | Reporte de texto           | Cualquier impresora Windows         | application/json     |
| POST   | `/api/v1/print/document`                          | Archivo (PDF, DOCX, etc.)  | Cualquier impresora Windows         | multipart/form-data  |

## Notas importantes

- El servicio corre en `localhost` (misma PC del cliente). No es accesible desde otras PCs por defecto.
- Puerto **58181** (rango privado IANA, sin conflictos con servicios conocidos).
- No requiere autenticacion.
- CORS esta habilitado para todos los origenes.
- Las impresoras Windows solo necesitan `printer_name`. Los campos como `vid`, `pid`, `ip` son para conexiones directas (USB/red/serial/bluetooth) sin pasar por el spooler de Windows.
- El scan puede tardar unos segundos si hay muchos protocolos habilitados (especialmente Bluetooth).
