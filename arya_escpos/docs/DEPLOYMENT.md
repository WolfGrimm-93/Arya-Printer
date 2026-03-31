# 🚀 Guía de Deployment - Arya ESCPOS Service

Instrucciones para desplegar el servicio Arya ESCPOS como servicio de Windows en producción.

---

## 📋 Tabla de Contenidos

1. [Requisitos Previos](#requisitos-previos)
2. [Opción 1: NSSM (Recomendado)](#opción-1-nssm-recomendado)
3. [Opción 2: Task Scheduler](#opción-2-task-scheduler)
4. [Opción 3: pythonservice](#opción-3-pythonservice)
5. [Configuración de Producción](#configuración-de-producción)
6. [Monitoreo y Logs](#monitoreo-y-logs)
7. [Troubleshooting](#troubleshooting)

---

## 📦 Requisitos Previos

### 1. Python y Dependencias

```powershell
# Verificar versión de Python
python --version  # Debe ser 3.8 o superior

# Instalar dependencias
cd "C:\ruta\arya_escpos"
pip install -r requirements.txt
```

### 2. Permisos de Administrador

El servicio **DEBE ejecutarse como Administrador** para:
- ✅ Configurar DEVMODE en impresoras Windows (duplex, color, etc.)
- ✅ Acceso completo a USB/Serial ports
- ✅ Modificar configuración de impresoras

---

## 🎯 Opción 1: NSSM (Recomendado)

**NSSM (Non-Sucking Service Manager)** es la forma más fácil y robusta de crear servicios de Windows.

### Paso 1: Descargar NSSM

1. Ir a: https://nssm.cc/download
2. Descargar **nssm 2.24** (o la última versión)
3. Extraer el ZIP
4. Copiar `nssm.exe` (de la carpeta `win64` o `win32`) a una ubicación permanente:

```powershell
# Crear carpeta para NSSM
New-Item -ItemType Directory -Force -Path "C:\Program Files\nssm"

# Copiar nssm.exe
Copy-Item "C:\ruta\descarga\nssm-2.24\win64\nssm.exe" "C:\Program Files\nssm\"

# Agregar al PATH (opcional)
$env:Path += ";C:\Program Files\nssm"
```

### Paso 2: Crear Script de Inicio

Crea un archivo `start_service.bat` en la carpeta del proyecto:

```batch
@echo off
REM start_service.bat - Script para iniciar Arya ESCPOS

REM Activar entorno virtual
call venv\Scripts\activate.bat

REM Iniciar servicio
python src\main.py

REM Si el servicio termina, pausar para ver errores
pause
```

Guarda este archivo en: `C:\arya_escpos\start_service.bat`

### Paso 3: Instalar Servicio con NSSM

```powershell
# Abrir PowerShell como Administrador
# Navegar a la carpeta de NSSM
cd "C:\Program Files\nssm"

# Instalar servicio (modo gráfico)
.\nssm.exe install AryaESCPOS
```

**Se abrirá una ventana GUI. Configura:**

#### Tab "Application":
- **Path:** `C:\arya_escpos\venv\Scripts\python.exe`
- **Startup directory:** `C:\arya_escpos`
- **Arguments:** `src\main.py`

#### Tab "Details":
- **Display name:** `Arya ESCPOS Printer Service`
- **Description:** `Servicio de impresión ESC/POS y Windows para Arya`
- **Startup type:** `Automatic`

#### Tab "Log on":
- **This account:** Seleccionar cuenta con privilegios de Administrador
- O usar: `Local System account` ✅ (Recomendado)
- Marcar: ✅ `Interact with desktop` (si necesitas ver errores)

#### Tab "Process":
- **Priority:** `Normal`

#### Tab "Shutdown":
- **Graceful timeout:** `10000` ms
- **Window close:** `5000` ms
- **Force timeout:** `3000` ms

#### Tab "Exit actions":
- **Restart:** Seleccionar "Restart application"
- **Throttle:** `5000` ms

#### Tab "I/O":
- **Output (stdout):** `C:\arya_escpos\logs\service_stdout.log`
- **Error (stderr):** `C:\arya_escpos\logs\service_stderr.log`

Click **"Install service"**

### Paso 4: Configurar Servicio (Línea de Comandos)

Alternativamente, puedes configurarlo todo desde PowerShell:

```powershell
# Abrir PowerShell como Administrador

# Instalar servicio
nssm install AryaESCPOS "C:\arya_escpos\venv\Scripts\python.exe" "src\main.py"

# Configurar carpeta de trabajo
nssm set AryaESCPOS AppDirectory "C:\arya_escpos"

# Configurar nombre y descripción
nssm set AryaESCPOS DisplayName "Arya ESCPOS Printer Service"
nssm set AryaESCPOS Description "Servicio de impresión ESC/POS y Windows para Arya"

# Configurar inicio automático
nssm set AryaESCPOS Start SERVICE_AUTO_START

# Configurar logs
nssm set AryaESCPOS AppStdout "C:\arya_escpos\logs\service_stdout.log"
nssm set AryaESCPOS AppStderr "C:\arya_escpos\logs\service_stderr.log"

# Rotar logs (máximo 10MB)
nssm set AryaESCPOS AppStdoutCreationDisposition 4
nssm set AryaESCPOS AppStderrCreationDisposition 4
nssm set AryaESCPOS AppRotateFiles 1
nssm set AryaESCPOS AppRotateOnline 1
nssm set AryaESCPOS AppRotateBytes 10485760

# Reinicio automático en caso de fallo
nssm set AryaESCPOS AppExit Default Restart
nssm set AryaESCPOS AppRestartDelay 5000

# Configurar dependencias (opcional - esperar a que red esté disponible)
nssm set AryaESCPOS DependOnService LanmanServer

# Verificar configuración
nssm dump AryaESCPOS
```

### Paso 5: Iniciar Servicio

```powershell
# Iniciar servicio
nssm start AryaESCPOS

# O desde Services.msc
services.msc
# Buscar "Arya ESCPOS Printer Service" → Click derecho → Start
```

### Paso 6: Verificar que Funciona

```powershell
# Ver estado del servicio
nssm status AryaESCPOS

# Verificar que el API responde
curl http://localhost:8181/api/v1/devices

# Ver logs en tiempo real
Get-Content "C:\arya_escpos\logs\service_stdout.log" -Wait -Tail 50
```

### Comandos Útiles de NSSM

```powershell
# Ver estado
nssm status AryaESCPOS

# Iniciar servicio
nssm start AryaESCPOS

# Detener servicio
nssm stop AryaESCPOS

# Reiniciar servicio
nssm restart AryaESCPOS

# Editar configuración (GUI)
nssm edit AryaESCPOS

# Remover servicio (CUIDADO)
nssm remove AryaESCPOS confirm

# Ver toda la configuración
nssm dump AryaESCPOS
```

---

## ⏰ Opción 2: Task Scheduler

Alternativa usando el Programador de Tareas de Windows.

### Paso 1: Crear Script VBS (para ejecutar sin ventana)

Crea `start_arya_hidden.vbs`:

```vbscript
Set WshShell = CreateObject("WScript.Shell")
WshShell.Run "C:\arya_escpos\venv\Scripts\python.exe src\main.py", 0, False
Set WshShell = Nothing
```

### Paso 2: Crear Tarea Programada

```powershell
# Abrir Task Scheduler
taskschd.msc
```

**Configuración manual:**

1. **Action → Create Task** (no Basic Task)
2. **General Tab:**
   - Name: `Arya ESCPOS Service`
   - Description: `Servicio de impresión ESC/POS`
   - ✅ Run whether user is logged on or not
   - ✅ Run with highest privileges
   - Configure for: `Windows 10`

3. **Triggers Tab:**
   - New → Begin the task: `At startup`
   - Delay task for: `30 seconds` (esperar a que red esté lista)
   - ✅ Enabled

4. **Actions Tab:**
   - New → Action: `Start a program`
   - Program/script: `C:\arya_escpos\venv\Scripts\python.exe`
   - Arguments: `src\main.py`
   - Start in: `C:\arya_escpos`

5. **Conditions Tab:**
   - ❌ Start the task only if the computer is on AC power
   - ✅ Wake the computer to run this task (opcional)

6. **Settings Tab:**
   - ✅ Allow task to be run on demand
   - ✅ Run task as soon as possible after a scheduled start is missed
   - If the task fails, restart every: `5 minutes`
   - Attempt to restart up to: `999` times
   - ❌ Stop the task if it runs longer than

### Paso 3: Crear desde PowerShell

```powershell
# Crear tarea programada
$action = New-ScheduledTaskAction -Execute "C:\arya_escpos\venv\Scripts\python.exe" `
    -Argument "src\main.py" `
    -WorkingDirectory "C:\arya_escpos"

$trigger = New-ScheduledTaskTrigger -AtStartup -RandomDelay (New-TimeSpan -Seconds 30)

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartInterval (New-TimeSpan -Minutes 5) `
    -RestartCount 999

$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName "AryaESCPOS" `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Description "Servicio de impresión ESC/POS y Windows para Arya"

# Iniciar tarea
Start-ScheduledTask -TaskName "AryaESCPOS"
```

---

## 🔧 Opción 3: pythonservice

Crear servicio nativo de Windows usando `pywin32`.

### Paso 1: Instalar pywin32

```powershell
pip install pywin32
```

### Paso 2: Crear Script de Servicio

Crea `windows_service.py`:

```python
import win32serviceutil
import win32service
import win32event
import servicemanager
import socket
import sys
import os
from pathlib import Path

# Agregar ruta del proyecto al PYTHONPATH
project_path = Path(__file__).parent
sys.path.insert(0, str(project_path))


class AryaESCPOSService(win32serviceutil.ServiceFramework):
    _svc_name_ = "AryaESCPOS"
    _svc_display_name_ = "Arya ESCPOS Printer Service"
    _svc_description_ = "Servicio de impresión ESC/POS y Windows para Arya"
    
    def __init__(self, args):
        win32serviceutil.ServiceFramework.__init__(self, args)
        self.stop_event = win32event.CreateEvent(None, 0, 0, None)
        socket.setdefaulttimeout(60)
        self.is_alive = True
    
    def SvcStop(self):
        self.ReportServiceStatus(win32service.SERVICE_STOP_PENDING)
        win32event.SetEvent(self.stop_event)
        self.is_alive = False
    
    def SvcDoRun(self):
        servicemanager.LogMsg(
            servicemanager.EVENTLOG_INFORMATION_TYPE,
            servicemanager.PYS_SERVICE_STARTED,
            (self._svc_name_, '')
        )
        self.main()
    
    def main(self):
        # Cambiar al directorio del proyecto
        os.chdir(str(project_path))
        
        # Importar y ejecutar main
        try:
            from src.main import main
            main()
        except Exception as e:
            servicemanager.LogErrorMsg(f"Error en servicio: {e}")


if __name__ == '__main__':
    if len(sys.argv) == 1:
        servicemanager.Initialize()
        servicemanager.PrepareToHostSingle(AryaESCPOSService)
        servicemanager.StartServiceCtrlDispatcher()
    else:
        win32serviceutil.HandleCommandLine(AryaESCPOSService)
```

### Paso 3: Instalar/Controlar Servicio

```powershell
# Instalar servicio
python windows_service.py install

# Iniciar servicio
python windows_service.py start

# Detener servicio
python windows_service.py stop

# Reiniciar servicio
python windows_service.py restart

# Remover servicio
python windows_service.py remove

# Debug (ejecutar en consola)
python windows_service.py debug
```

---

## ⚙️ Configuración de Producción

### 1. Archivo de Configuración

Crea `config/production.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8181
  reload: false  # ❌ Desactivar en producción
  workers: 1

database:
  url: sqlite:///C:/arya_escpos/data/arya_escpos.db
  echo: false

logging:
  level: INFO  # Cambiar a WARNING o ERROR en producción
  file: C:/arya_escpos/logs/arya_escpos.log
  rotation: 100 MB
  retention: 30 days

discovery:
  bluetooth_enabled: false  # Desactivar si no se usa
  auto_scan_interval: 300  # Escanear cada 5 minutos en vez de 30 segundos

devices:
  auto_reconnect: true
  connection_timeout: 10
  retry_attempts: 5
```

### 2. Variables de Entorno

Crea `.env` (si usas python-dotenv):

```env
ENVIRONMENT=production
LOG_LEVEL=INFO
DATABASE_URL=sqlite:///C:/arya_escpos/data/arya_escpos.db
```

### 3. Estructura de Carpetas Recomendada

```
C:\arya_escpos\
├── venv\                      # Virtual environment
├── src\                       # Código fuente
├── config\
│   ├── settings.yaml          # Config por defecto
│   └── production.yaml        # Config producción
├── data\
│   └── arya_escpos.db         # Base de datos
├── logs\
│   ├── arya_escpos.log        # Logs de aplicación
│   ├── service_stdout.log     # Logs de NSSM stdout
│   └── service_stderr.log     # Logs de NSSM stderr
├── start_service.bat
├── windows_service.py
└── requirements.txt
```

---

## 📊 Monitoreo y Logs

### Ver Logs en Tiempo Real

```powershell
# Logs de aplicación
Get-Content "C:\arya_escpos\logs\arya_escpos.log" -Wait -Tail 50

# Logs de servicio (NSSM)
Get-Content "C:\arya_escpos\logs\service_stdout.log" -Wait -Tail 50
Get-Content "C:\arya_escpos\logs\service_stderr.log" -Wait -Tail 50
```

### Event Viewer

1. Abrir Event Viewer: `eventvwr.msc`
2. Navegar a: **Windows Logs → Application**
3. Filtrar por Source: `AryaESCPOS` o `NSSM`

### Verificar Estado del Servicio

```powershell
# PowerShell
Get-Service AryaESCPOS

# Services GUI
services.msc

# NSSM
nssm status AryaESCPOS

# Task Scheduler
Get-ScheduledTask -TaskName "AryaESCPOS"
```

### Monitorear API

```powershell
# Verificar que responde
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices"

# Verificar configuración
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/config"

# Verificar status de impresora
Invoke-RestMethod -Uri "http://localhost:8181/api/v1/devices/win-Canon_GX6000_series/status"
```

---

## 🔥 Firewall y Seguridad

### Abrir Puerto en Firewall

```powershell
# Permitir puerto 8181
New-NetFirewallRule -DisplayName "Arya ESCPOS API" `
    -Direction Inbound `
    -LocalPort 8181 `
    -Protocol TCP `
    -Action Allow

# O desde GUI
wf.msc
# Nueva Regla → Puerto → TCP → 8181 → Permitir conexión
```

### Restringir Acceso (Opcional)

Si solo quieres acceso local, cambia en `config/production.yaml`:

```yaml
server:
  host: 127.0.0.1  # Solo localhost
  port: 8181
```

---

## 🐛 Troubleshooting

### El servicio no inicia

```powershell
# Ver logs de error
Get-Content "C:\arya_escpos\logs\service_stderr.log" -Tail 50

# Ejecutar manualmente para ver errores
cd "C:\arya_escpos"
.\venv\Scripts\activate
python src\main.py
```

### Puerto 8181 ya en uso

```powershell
# Ver qué proceso usa el puerto
netstat -ano | findstr :8181

# Matar proceso (si es necesario)
taskkill /PID <PID> /F
```

### Permisos insuficientes

- Asegúrate de que el servicio se ejecuta como SYSTEM o con cuenta de Administrador
- Verifica permisos de carpetas (logs, data)

### Base de datos bloqueada

```powershell
# SQLite no permite múltiples escritores
# Asegúrate de que solo hay una instancia del servicio corriendo
Get-Process python | Where-Object {$_.Path -like "*arya_escpos*"}
```

---

## 🔄 Actualizar el Servicio

```powershell
# Detener servicio
nssm stop AryaESCPOS

# Actualizar código (git pull, copiar archivos, etc.)
cd C:\arya_escpos
git pull

# Actualizar dependencias
.\venv\Scripts\activate
pip install -r requirements.txt --upgrade

# Reiniciar servicio
nssm start AryaESCPOS
```

---

## 📝 Checklist de Deployment

- [ ] Python 3.8+ instalado
- [ ] Dependencias instaladas (`pip install -r requirements.txt`)
- [ ] Configuración de producción creada
- [ ] Estructura de carpetas creada (logs, data)
- [ ] NSSM instalado y configurado
- [ ] Servicio instalado como SYSTEM o cuenta Admin
- [ ] Servicio configurado para inicio automático
- [ ] Logs configurados y rotación habilitada
- [ ] Puerto 8181 abierto en firewall (si se necesita acceso remoto)
- [ ] Servicio iniciado y funcionando
- [ ] API respondiendo en http://localhost:8181/docs
- [ ] Impresoras detectadas (`/api/v1/devices/scan`)
- [ ] Status de impresoras funcionando
- [ ] Prueba de impresión exitosa

---

**Deployment completado!** 🎉

El servicio Arya ESCPOS ahora está ejecutándose como servicio de Windows en segundo plano, se iniciará automáticamente con el sistema, y se reiniciará automáticamente en caso de fallo.
