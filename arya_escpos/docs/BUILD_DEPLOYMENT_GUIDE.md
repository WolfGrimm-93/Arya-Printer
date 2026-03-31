# 📦 Guía de Compilación y Deployment - Ejecutable Standalone

## 🎯 Objetivo

Crear un ejecutable standalone (`AryaESCPOS.exe`) que incluya Python, todas las dependencias y el código, para facilitar el deployment sin necesidad de instalar Python en las computadoras de producción.

---

## 🏗️ PARTE 1: Compilar el Ejecutable (Solo una vez, en tu PC de desarrollo)

### Paso 1: Preparar el Entorno

```powershell
# Asegúrate de estar en la carpeta del proyecto
cd "ruta\del\proyecto\arya_escpos"

# Activar virtual environment
.\venv\Scripts\activate

# Verificar que PyInstaller está instalado
pip list | findstr pyinstaller
# Si no aparece: pip install pyinstaller
```

### Paso 2: Compilar el Proyecto

```powershell
# Ejecutar script de build
python build_executable.py
```

**Esto tomará 2-5 minutos.** Verás:
- Análisis de dependencias
- Compilación del ejecutable
- Progreso en consola

### Paso 3: Verificar el Ejecutable

```powershell
# Navegar a la carpeta dist
cd dist

# Ejecutar el .exe para probar
.\AryaESCPOS.exe
```

Si todo está bien, verás:
```
2026-01-26 16:00:00 | INFO | Initializing Arya ESCPOS...
2026-01-26 16:00:01 | INFO | Database initialized
2026-01-26 16:00:02 | INFO | Starting server on 0.0.0.0:8181
```

**Abre el navegador:** http://localhost:8181/docs

**Detener:** Presiona `Ctrl+C` en la consola

### Paso 4: Resultado Final

```
arya_escpos\
└── dist\
    └── AryaESCPOS.exe  ← Este es tu ejecutable (70-100 MB aprox)
```

---

## 📦 PARTE 2: Preparar Paquete de Deployment

### Crear Carpeta de Distribución

```powershell
# Volver a la raíz del proyecto
cd ..

# Crear carpeta de deployment
New-Item -ItemType Directory -Path "AryaESCPOS_Deploy"

# Copiar archivos necesarios
Copy-Item "dist\AryaESCPOS.exe" "AryaESCPOS_Deploy\"
Copy-Item "install_service_exe.ps1" "AryaESCPOS_Deploy\"
Copy-Item "README.md" "AryaESCPOS_Deploy\"  # Opcional
```

### Estructura del Paquete

```
AryaESCPOS_Deploy\
├── AryaESCPOS.exe              ← Ejecutable compilado
├── install_service_exe.ps1     ← Script de instalación del servicio
└── README.md                   ← Instrucciones (opcional)
```

### Crear ZIP para Distribución

```powershell
# Comprimir carpeta
Compress-Archive -Path "AryaESCPOS_Deploy\*" -DestinationPath "AryaESCPOS_v1.0.zip"
```

**Resultado:** `AryaESCPOS_v1.0.zip` (15-30 MB comprimido)

---

## 🚀 PARTE 3: Deployment en Computadoras de Producción

### Requisitos en la PC de Producción

- ✅ Windows 10/11 (64-bit)
- ✅ Permisos de Administrador
- ❌ **NO requiere Python**
- ❌ **NO requiere instalar dependencias**

### Paso 1: Copiar Archivos

```powershell
# Opción A: Desde USB
Copy-Item "E:\AryaESCPOS_v1.0.zip" "C:\Temp\"
Expand-Archive -Path "C:\Temp\AryaESCPOS_v1.0.zip" -DestinationPath "C:\arya_escpos"

# Opción B: Desde red compartida
Copy-Item "\\servidor\compartido\AryaESCPOS_v1.0.zip" "C:\Temp\"
Expand-Archive -Path "C:\Temp\AryaESCPOS_v1.0.zip" -DestinationPath "C:\arya_escpos"

# Opción C: Copiar manualmente
# - Extraer ZIP en C:\arya_escpos
```

### Paso 2: Verificar Estructura

```
C:\arya_escpos\
├── AryaESCPOS.exe
└── install_service_exe.ps1
```

### Paso 3: Instalar como Servicio

```powershell
# Abrir PowerShell como Administrador
# Click derecho en el menú inicio → PowerShell (Admin)

# Permitir ejecución de scripts (primera vez)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Navegar a la carpeta
cd C:\arya_escpos

# Ejecutar script de instalación
.\install_service_exe.ps1
```

**El script te preguntará:**
1. ¿Abrir puerto 8181 en firewall? → `S` (si accedes desde otras PCs)
2. ¿Iniciar servicio ahora? → `S`
3. ¿Escanear impresoras ahora? → `S`

### Paso 4: Verificar Instalación

```powershell
# Ver estado del servicio
Get-ScheduledTask -TaskName "AryaESCPOS"

# Debería mostrar:
# TaskName                 State
# --------                 -----
# AryaESCPOS              Running

# Verificar que el API responde
Invoke-RestMethod http://localhost:8181/api/v1/devices

# Abrir Swagger UI en navegador
start http://localhost:8181/docs
```

---

## 🔄 PARTE 4: Actualizar el Servicio (Versiones Futuras)

### En tu PC de Desarrollo

```powershell
# 1. Hacer cambios en el código
# 2. Recompilar
python build_executable.py

# 3. Crear nuevo ZIP
Copy-Item "dist\AryaESCPOS.exe" "AryaESCPOS_Deploy\"
Compress-Archive -Path "AryaESCPOS_Deploy\*" -DestinationPath "AryaESCPOS_v1.1.zip" -Force
```

### En Producción

```powershell
# 1. Detener servicio
Stop-ScheduledTask -TaskName "AryaESCPOS"

# 2. Respaldar ejecutable anterior (opcional)
Copy-Item "C:\arya_escpos\AryaESCPOS.exe" "C:\arya_escpos\AryaESCPOS.exe.bak"

# 3. Copiar nuevo ejecutable
Copy-Item "E:\AryaESCPOS.exe" "C:\arya_escpos\AryaESCPOS.exe" -Force

# 4. Reiniciar servicio
Start-ScheduledTask -TaskName "AryaESCPOS"

# 5. Verificar
Invoke-RestMethod http://localhost:8181/api/v1/devices
```

**Nota:** La base de datos y configuración se mantienen intactos.

---

## 📊 Estructura de Archivos en Producción

Después de la primera ejecución:

```
C:\arya_escpos\
├── AryaESCPOS.exe              ← Ejecutable
├── install_service_exe.ps1     ← Script de instalación
├── arya_escpos.db              ← Base de datos (se crea automáticamente)
└── logs\                       ← Logs (se crean automáticamente)
    ├── arya_escpos.log
    ├── service_stdout.log
    └── service_stderr.log
```

---

## 🛠️ Comandos Útiles en Producción

### Gestión del Servicio

```powershell
# Ver estado
Get-ScheduledTask -TaskName "AryaESCPOS"

# Iniciar
Start-ScheduledTask -TaskName "AryaESCPOS"

# Detener
Stop-ScheduledTask -TaskName "AryaESCPOS"

# Reiniciar (detener + iniciar)
Stop-ScheduledTask -TaskName "AryaESCPOS"
Start-Sleep -Seconds 2
Start-ScheduledTask -TaskName "AryaESCPOS"

# Deshabilitar (no iniciará automáticamente)
Disable-ScheduledTask -TaskName "AryaESCPOS"

# Habilitar
Enable-ScheduledTask -TaskName "AryaESCPOS"

# Eliminar servicio
Unregister-ScheduledTask -TaskName "AryaESCPOS" -Confirm:$false
```

### Ver Logs

```powershell
# Logs de aplicación (en tiempo real)
Get-Content "C:\arya_escpos\logs\arya_escpos.log" -Tail 50 -Wait

# Últimas 100 líneas
Get-Content "C:\arya_escpos\logs\arya_escpos.log" -Tail 100

# Buscar errores
Get-Content "C:\arya_escpos\logs\arya_escpos.log" | Select-String "ERROR"
```

### API

```powershell
# Listar dispositivos
Invoke-RestMethod http://localhost:8181/api/v1/devices

# Escanear impresoras
Invoke-RestMethod http://localhost:8181/api/v1/devices/scan

# Ver status de impresora
Invoke-RestMethod "http://localhost:8181/api/v1/devices/win-Canon_GX6000_series/status"

# Ver configuración
Invoke-RestMethod http://localhost:8181/api/v1/config
```

---

## 🐛 Troubleshooting

### El ejecutable no inicia

```powershell
# Ejecutar manualmente para ver errores
cd C:\arya_escpos
.\AryaESCPOS.exe

# Ver logs
Get-Content "C:\arya_escpos\logs\arya_escpos.log" -Tail 50
```

### Puerto 8181 ya en uso

```powershell
# Ver qué proceso usa el puerto
netstat -ano | findstr :8181

# Matar proceso si es necesario
taskkill /PID <numero_pid> /F
```

### Servicio no se ejecuta al iniciar Windows

```powershell
# Verificar configuración de la tarea
Get-ScheduledTask -TaskName "AryaESCPOS" | fl *

# Verificar triggers
(Get-ScheduledTask -TaskName "AryaESCPOS").Triggers

# Debe mostrar: AtStartup
```

### Impresoras no se detectan

```powershell
# Escanear manualmente
Invoke-RestMethod http://localhost:8181/api/v1/devices/scan

# Verificar drivers de impresora instalados
Get-Printer

# Para impresoras Windows
Get-Printer | Select-Object Name, DriverName, PortName
```

---

## ✅ Checklist de Deployment

### En PC de Desarrollo
- [ ] Código funcionando correctamente
- [ ] PyInstaller instalado
- [ ] Ejecutar `python build_executable.py`
- [ ] Probar `dist\AryaESCPOS.exe` localmente
- [ ] Crear paquete de deployment (ZIP)

### En PC de Producción (cada una)
- [ ] Copiar ZIP a `C:\arya_escpos`
- [ ] Extraer archivos
- [ ] Abrir PowerShell como Administrador
- [ ] Ejecutar `.\install_service_exe.ps1`
- [ ] Confirmar apertura de firewall (si necesario)
- [ ] Iniciar servicio
- [ ] Escanear impresoras
- [ ] Verificar API en http://localhost:8181/docs
- [ ] Probar impresión de prueba

---

## 📞 Soporte

**Logs de errores:** `C:\arya_escpos\logs\arya_escpos.log`  
**API Docs:** `http://localhost:8181/docs`  
**Puerto:** `8181`  

---

**¡Deployment completado!** 🎉

El servicio está corriendo como tarea programada de Windows, iniciará automáticamente con el sistema, y se reiniciará en caso de fallo.
