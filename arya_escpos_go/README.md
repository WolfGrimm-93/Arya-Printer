# Arya Printer Service (Go)

Reescritura en Go del "Arya ESCPOS Service" (Python/FastAPI). Servicio REST local para deteccion e impresion en impresoras ESC/POS (termicas), ESC/P (matriciales) y documentos via el driver de Windows. Corre como servicio de Windows en cada PC cliente para que la app web **Arya Market** pueda imprimir sin acceso directo al hardware desde el navegador.

Esta reescritura corrige los hallazgos de la auditoria de seguridad del servicio Python: sin autenticacion, CORS abierto a cualquier origen, corria como Tarea Programada con usuario **SYSTEM**, carpetas con permisos `everyone-full`, regla de firewall sin restriccion de alcance, y una SSRF en el endpoint de estado de dispositivos de red. Cada seccion de abajo indica que cambio y por que.

## Arquitectura

```
Arya Market (navegador)
    |
    v
http(s)://127.0.0.1:58181/api/v1/...   (X-API-Key requerido)
    |
    v
Arya Printer Service (Go, net/http)
    |
    +-- Windows Spooler (golang.org/x/sys/windows + winspool.drv)
    +-- USB (libusb via libs/libusb-1.0.dll)
    +-- Network (TCP socket :9100, allowlist via internal/netguard)
    +-- Serial (go.bug.st/serial, puerto COM)
```

- **Sin base de datos** — descubrimiento en vivo, datos de conexion provistos por el cliente en cada request.
- **Sin bluetooth** — el soporte Bluetooth del servicio Python (PyBluez, incompatible con Python 3.12+) no se porto; el campo `type: "bluetooth"` se acepta por compatibilidad de esquema pero responde `400`.
- **1 servicio por PC** — cada PC cliente corre su propia instancia en `127.0.0.1:58181` (nunca `0.0.0.0` por defecto — ver seccion de seguridad).

## Requisitos

**Para compilar:**
- Go 1.26 o superior.

**En la PC donde corre el servicio (runtime):**
- Windows (usa `winspool.drv` y el Service Control Manager).
- [LibreOffice](https://www.libreoffice.org/) — conversion de DOCX/XLSX/etc. a PDF para `POST /api/v1/print/document`.
- [SumatraPDF](https://www.sumatrapdfreader.org/) o [PDFtoPrinter](https://github.com/artiebits/pdftoprinter) — impresion de PDF sin dialogo, tambien para `/print/document`. Sin ninguno de los dos ese endpoint responde `503`; el resto del servicio (tickets ESC/POS, ESC/P, reportes) no depende de ellos.

El instalador (`installer/installer.iss`) verifica la presencia de estos dos ultimos en rutas conocidas y avisa si faltan, sin bloquear la instalacion — ver [Build y distribucion](#build-y-distribucion).

## Estructura del proyecto

```
arya_escpos_go/
├── cmd/aryaprinter/            # entry point, flags CLI, Windows Service handler
├── internal/
│   ├── contract/                # DTOs + interfaces compartidas (congelado desde Fase 0)
│   ├── config/                  # carga de configs/settings.yaml
│   ├── auth/                    # API key local (generacion, validacion constant-time)
│   ├── middleware/               # auth, CORS, recovery, limite de body, logging
│   ├── apiserver/                # los 9 endpoints /api/v1/*, mas / y /health
│   ├── hwadapter/                # adaptadores USB/red/serial (implementa contract.DeviceAdapter)
│   ├── winspool/                 # Windows Spooler API
│   ├── escpos/ escp/             # constructores de comandos ESC/POS y ESC/P
│   ├── printsvc/                 # orquesta adaptador + comandos por tipo de impresion
│   ├── document/                 # conversion + impresion de documentos (LibreOffice/Sumatra/PDFtoPrinter)
│   ├── history/                  # historial de impresion en memoria (max 500)
│   ├── netguard/                 # allowlist de red para el tipo "network" (fix SSRF)
│   └── ssl/                      # HTTPS local via mkcert (igual mecanismo que el servicio Python)
├── installer/
│   ├── installer.iss             # instalador Inno Setup
│   ├── svcinstall/                # instalacion/desinstalacion del servicio de Windows
│   ├── setup_firewall.ps1 / remove_firewall.ps1
│   └── set_secure_acl.ps1        # ACL restrictiva de logs/configs/ssl/secrets
├── scripts/build.ps1             # gate de build+vet+test antes de commitear
├── test/e2e/                     # tests end-to-end contra el stack HTTP completo
├── configs/settings.yaml
├── libs/libusb-1.0.dll
└── mkcert.exe
```

## Compilar

```powershell
go build ./cmd/aryaprinter
```

Para el build que consume el instalador (ver `installer/installer.iss`, que espera el binario en `dist\aryaprinter.exe`):

```powershell
go build -o dist\aryaprinter.exe .\cmd\aryaprinter
```

Antes de cualquier commit, correr el gate de verificacion (build + vet + test, falla si algo no sale limpio):

```powershell
.\scripts\build.ps1
```

## Correr en desarrollo

```powershell
go run .\cmd\aryaprinter
```

Arranca en `http://127.0.0.1:58181` (host/puerto en `configs/settings.yaml`, seccion `server`). La primera vez genera una API key nueva en `secrets/apikey.key` (ver [Autenticacion](#autenticacion-y-api-key)).

## Flags CLI

Todos se leen en `cmd/aryaprinter/main.go`; `-config` acepta una ruta alternativa a `settings.yaml` (default `configs/settings.yaml`).

| Flag | Que hace |
|------|----------|
| `--setup-ssl` | Genera/renueva el certificado HTTPS local via mkcert (CA + `ssl/server.crt` + `ssl/server.key`). Ver [HTTPS en localhost](#https-en-localhost). |
| `--remove-ssl` | Elimina la CA de mkcert de los almacenes de confianza y borra los certificados generados. |
| `--show-api-key` | Imprime la API key actual (la genera si todavia no existe) y termina. No queda registrada en los logs. |
| `--install-service` | Registra este binario como servicio de Windows (`installer/svcinstall`, cuenta virtual de bajo privilegio) y lo arranca. Requiere ejecutarse como Administrador. |
| `--uninstall-service` | Detiene y elimina el registro del servicio de Windows. Requiere Administrador. |

Sin ningun flag, el proceso corre el servidor HTTP interactivamente (o, si el Service Control Manager lo lanzo, como servicio) hasta `Ctrl+C`/`SIGTERM` o el comando de parada del SCM.

## Seguridad — que cambio respecto al servicio Python

| Hallazgo de la auditoria (Python) | Correccion en esta reescritura |
|---|---|
| Corria como Tarea Programada con usuario **SYSTEM** | Servicio de Windows real bajo una **cuenta virtual** (`NT SERVICE\AryaESCPOSService`) sin privilegios de SYSTEM — ver `installer/svcinstall`. |
| `logs/`, `config/`, `ssl/` con permisos `everyone-full`/`everyone-modify` | `installer/set_secure_acl.ps1` deja `logs/`, `configs/`, `ssl/`, `secrets/` con permisos solo para la cuenta de servicio + Administradores. |
| Regla de firewall creada siempre, sin restriccion de perfil ni alcance | `installer/setup_firewall.ps1` es **condicional**: solo crea la regla si `server.host` no es loopback, y la restringe a los perfiles `Private,Domain` (nunca `Public`). Task del instalador destildada por defecto. |
| Sin autenticacion — cualquier proceso local o pagina web podia llamar a la API | Toda ruta `/api/v1/*` requiere el header `X-API-Key` (excepto `GET /` y `GET /health`, para liveness/monitoring). Ver [Autenticacion](#autenticacion-y-api-key). |
| CORS `allow_origins=["*"]` + `Access-Control-Allow-Private-Network: true` siempre | CORS acepta cualquier origen (haciendo eco del `Origin` recibido, nunca un `*` literal) - el control de acceso real es el header `X-API-Key`, no una whitelist de dominios. `Access-Control-Allow-Private-Network` nunca se emite. |
| `server.host` por defecto `0.0.0.0` (expuesto a toda la red) | Por defecto `127.0.0.1` — el servicio solo escucha localhost salvo que se configure explicitamente lo contrario. |
| `GET /devices/network/{ip}:{port}/status` sin validar destino (oraculo SSRF) | `internal/netguard` valida contra el allowlist de `network_scan` (subredes/puertos permitidos) antes de intentar conectar. |
| Sin limite de tamaño de upload/imagen | `security.max_upload_bytes` (default 50 MB) y `security.max_image_bytes` (default 5 MB) aplicados por middleware. |

### Autenticacion y API key

La API key vive en `secrets/apikey.key` (ruta configurable via `security.api_key_path`), se genera sola en el primer arranque (32 bytes aleatorios, `crypto/rand`, base64url) y se reutiliza en arranques siguientes. Se compara con comparacion de tiempo constante (`crypto/subtle`), nunca con `==`.

**Configurar la API key en Arya Market:** despues de instalar el servicio, obtener la key con:

```powershell
& "C:\Program Files\AryaPrinter\aryaprinter.exe" --show-api-key
```

> Pendiente de documentar: el mecanismo exacto para cargar esa key en la configuracion de Arya Market (que campo, que pantalla) depende de como el equipo de Arya Market lo haya expuesto del lado de la app web — no estaba definido al escribir esta seccion. Actualizar este parrafo con el paso concreto una vez que este resuelto.

Cada request a `/api/v1/*` (salvo `/` y `/health`) debe incluir:

```
X-API-Key: <la key de secrets/apikey.key>
```

Se puede desactivar con `security.auth_enabled: false` en `settings.yaml` — pensado solo para desarrollo local, nunca para produccion.

## HTTPS en localhost

Igual mecanismo que el servicio Python: [mkcert](https://github.com/FiloSottile/mkcert) genera un certificado confiable por Chrome/Firefox/Edge sin ningun prompt manual.

```powershell
# Instala la CA de mkcert + genera ssl/server.crt y ssl/server.key
& "C:\Program Files\AryaPrinter\aryaprinter.exe" --setup-ssl

# Elimina la CA y los certificados
& "C:\Program Files\AryaPrinter\aryaprinter.exe" --remove-ssl
```

Si `ssl/server.crt` y `ssl/server.key` existen al iniciar, el servicio arranca en HTTPS; si no, en HTTP. Si el certificado vence en 30 dias o menos, `--setup-ssl` lo renueva solo.

## Configuracion

Archivo: `configs/settings.yaml`. Ver comentarios inline en el archivo y en `internal/config/types.go` para el detalle de cada campo — la diferencia mas importante respecto al servicio Python es `server.host: "127.0.0.1"` (era `"0.0.0.0"`) y el bloque `security`, que no tiene equivalente en Python.

## Testing

```powershell
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# Todo el modulo
go test ./...

# Solo los tests end-to-end (stack HTTP completo: middlewares + fakes de internal/contract)
go test ./test/e2e/... -v

# Gate completo (build + vet + test)
.\scripts\build.ps1
```

`test/e2e` no depende de hardware real ni de las implementaciones concretas de `internal/printsvc`/`internal/winspool`/`internal/history`/`internal/document`: usa sus propios fakes de las interfaces en `internal/contract`, así que corre incluso si esos paquetes todavía no existen o están en desarrollo.

## Build y distribucion

1. Compilar el binario: `go build -o dist\aryaprinter.exe .\cmd\aryaprinter`.
2. Instalar [Inno Setup 6](https://jrsoftware.org/isdl.php).
3. Abrir `installer/installer.iss` en el compilador de Inno Setup, `Build > Compile` (F9).
4. Resultado: `installer/installer_output/AryaPrinterService_Setup_v<version>.exe`.

### Que hace el instalador

- Instala en `C:\Program Files\AryaPrinter`.
- Registra el servicio de Windows (`aryaprinter.exe --install-service`), bajo la cuenta virtual `NT SERVICE\AryaESCPOSService` — nunca SYSTEM.
- Restringe permisos de `logs/`, `configs/`, `ssl/`, `secrets/` a esa cuenta + Administradores (`installer/set_secure_acl.ps1`).
- Opcionalmente (Task destildada por defecto) agrega la excepcion de firewall, solo si `server.host` no es loopback.
- Opcionalmente (Task destildada por defecto) configura HTTPS local con mkcert.
- Avisa (sin bloquear) si no encuentra LibreOffice o SumatraPDF/PDFtoPrinter en las rutas habituales.
- Desinstalacion limpia desde el Panel de Control: detiene y elimina el servicio, la regla de firewall y la CA de mkcert. `configs/` y `secrets/` (la API key) se conservan para no invalidar una configuracion ya cargada en Arya Market ni forzar una reconfiguracion en una reinstalacion.

### Instalacion silenciosa

```powershell
.\AryaPrinterService_Setup_v2.0.0.exe /VERYSILENT
```

## Troubleshooting

| Problema | Solucion |
|---|---|
| Puerto 58181 en uso | `netstat -ano \| findstr :58181` y matar el proceso que lo ocupa. |
| `401 Unauthorized` en toda la API | Falta el header `X-API-Key`, o no coincide con `secrets/apikey.key` — verificar con `--show-api-key`. |
| USB no detecta impresoras | Verificar `libs/libusb-1.0.dll` e instalar el driver WinUSB con [Zadig](https://zadig.akeo.ie/) sobre el dispositivo. |
| `POST /api/v1/print/document` devuelve 503 | Falta LibreOffice y/o SumatraPDF/PDFtoPrinter — ver [Requisitos](#requisitos). |
| El servicio no aparece en `services.msc` tras instalar | Confirmar que el instalador corrio como Administrador; revisar el log de instalacion de Inno Setup. |
| Firewall bloquea la conexion desde otra PC | Solo aplica si `server.host` no es loopback a proposito — ver `installer/setup_firewall.ps1`; por defecto el servicio solo escucha en `127.0.0.1` y no necesita regla de firewall. |

## Stack tecnologico

- Go 1.26, `net/http` (sin framework web externo).
- `golang.org/x/sys/windows` — Service Control Manager, Windows Spooler.
- `go.bug.st/serial` — comunicacion serial.
- `github.com/go-pdf/fpdf`, `github.com/boombuler/barcode` — generacion de PDF/codigos de barras donde aplica.
- `gopkg.in/yaml.v3` — configuracion.
- `gopkg.in/natefinch/lumberjack.v2` — rotacion de logs.
- Inno Setup 6 — instalador Windows.
- mkcert — certificados HTTPS locales confiables.
