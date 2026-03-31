# 📦 Guía para Crear el Instalador Setup.exe

## 🎯 Objetivo

Crear un instalador profesional **AryaESCPOS_Setup_v1.0.0.exe** que:
- ✅ Se instala automáticamente en `C:\Program Files\AryaESCPOS`
- ✅ Configura el servicio de Windows automáticamente
- ✅ Abre el firewall (opcional)
- ✅ Escanea impresoras (opcional)
- ✅ Tiene asistente de instalación profesional
- ✅ Permite desinstalación completa

---

## 📥 PASO 1: Instalar Inno Setup (Solo una vez)

### Descargar Inno Setup

1. Ve a: https://jrsoftware.org/isdl.php
2. Descarga: **Inno Setup 6.3.3** (versión estable más reciente)
3. Descarga también: **Inno Setup QuickStart Pack** (incluye ayuda y ejemplos)

### Instalar

```
- Ejecutar: innosetup-6.3.3.exe
- Siguiente → Siguiente → Instalar
- Instalación típica: C:\Program Files (x86)\Inno Setup 6
```

**Opcional:** Descargar el pack de idiomas español:
- https://jrsoftware.org/files/istrans/
- Descargar: Spanish.isl (ya está incluido en versiones recientes)

---

## 🏗️ PASO 2: Compilar tu Aplicación (Ya lo hiciste)

Ya tienes compilado en:
```
arya_escpos\dist\AryaESCPOS\
├── AryaESCPOS.exe
└── _internal\
```

✅ **Este paso ya está completo**

---

## 🎨 PASO 3: Compilar el Instalador

### Opción A: Desde la GUI de Inno Setup (Más fácil)

1. **Abrir Inno Setup Compiler**
   - Inicio → Inno Setup 6 → Inno Setup Compiler

2. **Abrir el script**
   - File → Open
   - Seleccionar: `arya_escpos\installer.iss`

3. **Compilar**
   - Build → Compile (o presionar F9)
   - Esperar 30-60 segundos

4. **¡Listo!**
   - El instalador estará en: `installer_output\AryaESCPOS_Setup_v1.0.0.exe`

### Opción B: Desde la línea de comandos (Automatizado)

```powershell
# Desde PowerShell en la carpeta del proyecto
cd "ruta\del\proyecto\arya_escpos"

# Compilar el instalador
& "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" installer.iss
```

---

## 📦 PASO 4: Resultado Final

Tendrás un archivo:

```
installer_output\
└── AryaESCPOS_Setup_v1.0.0.exe  (40-50 MB comprimido)
```

---

## 🚀 PASO 5: Usar el Instalador

### En la PC de Producción

1. **Copiar** `AryaESCPOS_Setup_v1.0.0.exe` a la PC
   - Por USB, red, o descarga

2. **Ejecutar como Administrador**
   - Click derecho → Ejecutar como administrador
   - Si aparece UAC (Control de Cuentas) → Sí

3. **Asistente de Instalación**
   ```
   Paso 1: Bienvenida → Siguiente
   Paso 2: Carpeta de destino → C:\Program Files\AryaESCPOS (Siguiente)
   Paso 3: Opciones:
           ☑ Instalar como servicio de Windows
           ☑ Abrir puerto 8181 en Firewall
           ☑ Escanear impresoras automáticamente
           → Siguiente
   Paso 4: Instalar
   Paso 5: ¡Listo! → Finalizar
   ```

4. **Verificar**
   - Se abre automáticamente: http://localhost:8181/docs
   - El servicio ya está corriendo

---

## 🔄 Actualizar Versión (Futuras versiones)

### 1. Actualizar el código

```powershell
# Hacer cambios en src/
# ...

# Recompilar ejecutable
python build_executable.py
```

### 2. Actualizar versión en installer.iss

```iss
#define MyAppVersion "1.1.0"  ; ← Cambiar aquí
```

### 3. Recompilar instalador

```powershell
& "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" installer.iss
```

### 4. Distribuir

```
installer_output\AryaESCPOS_Setup_v1.1.0.exe
```

**Al instalar sobre una versión anterior:**
- El instalador detecta la versión previa
- Detiene el servicio
- Actualiza archivos
- Reinicia el servicio
- ¡La base de datos y configuración se mantienen!

---

## 🎨 Personalización del Instalador (Opcional)

### Agregar Icono

1. Crea o descarga un archivo `icon.ico`
2. Guárdalo en: `assets\icon.ico`
3. En `installer.iss`, descomenta:
   ```iss
   SetupIconFile=assets\icon.ico
   ```

### Agregar Imágenes del Asistente

```iss
WizardImageFile=assets\wizard.bmp        ; 164x314 pixels
WizardSmallImageFile=assets\wizard-small.bmp  ; 55x58 pixels
```

### Cambiar Información

Edita en `installer.iss`:
```iss
#define MyAppPublisher "Tu Empresa"
#define MyAppURL "https://tu-sitio.com"
```

---

## 📊 Comparación: Antes vs Ahora

### ❌ Antes (Manual)

1. Copiar carpeta `AryaESCPOS` manualmente
2. Copiar script PowerShell
3. Ejecutar PowerShell como Admin
4. Permitir ejecución de scripts
5. Configurar manualmente
6. Verificar instalación

**Total: 6 pasos técnicos**

### ✅ Ahora (Instalador)

1. Ejecutar `AryaESCPOS_Setup_v1.0.0.exe`
2. Siguiente → Siguiente → Instalar

**Total: 2 clicks**

---

## 🛠️ Comandos Útiles del Instalador

### Instalación Silenciosa (Sin asistente)

```powershell
# Instalar sin interacción
.\AryaESCPOS_Setup_v1.0.0.exe /SILENT

# Instalar completamente invisible
.\AryaESCPOS_Setup_v1.0.0.exe /VERYSILENT

# Instalar en carpeta específica
.\AryaESCPOS_Setup_v1.0.0.exe /DIR="D:\MisCosas\AryaESCPOS"

# Instalar + seleccionar todas las tareas + silencioso
.\AryaESCPOS_Setup_v1.0.0.exe /VERYSILENT /TASKS="installservice,openfirewall,scanprinters"
```

### Desinstalación

```powershell
# Desde Panel de Control → Programas y características
# O directamente:
"C:\Program Files\AryaESCPOS\unins000.exe"

# Desinstalación silenciosa
"C:\Program Files\AryaESCPOS\unins000.exe" /SILENT
```

---

## 🐛 Troubleshooting

### Error: "Cannot find file dist\AryaESCPOS\AryaESCPOS.exe"

**Solución:** Asegúrate de haber compilado primero con:
```powershell
python build_executable.py
```

### Error: "Spanish.isl not found"

**Solución:** Cambia en `installer.iss`:
```iss
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
```

### El instalador no crea el servicio

**Solución:** Ejecutar el instalador como Administrador
- Click derecho → Ejecutar como administrador

### Puerto 8181 no se abre en firewall

**Solución:** Ejecutar manualmente:
```powershell
netsh advfirewall firewall add rule name="Arya ESCPOS Service" dir=in action=allow protocol=TCP localport=8181
```

---

## ✅ Checklist Final

- [ ] Inno Setup instalado
- [ ] Ejecutable compilado (`python build_executable.py`)
- [ ] Script `installer.iss` en la raíz del proyecto
- [ ] Compilar instalador (F9 en Inno Setup o ISCC.exe)
- [ ] Verificar que se creó `installer_output\AryaESCPOS_Setup_v1.0.0.exe`
- [ ] Probar instalación en máquina de prueba
- [ ] Verificar que el servicio inicia automáticamente
- [ ] Verificar API en http://localhost:8181/docs
- [ ] Distribuir el instalador

---

## 📞 Información de Soporte

**Archivo instalador:** `AryaESCPOS_Setup_v1.0.0.exe`  
**Tamaño:** ~40-50 MB  
**Carpeta de instalación:** `C:\Program Files\AryaESCPOS`  
**Puerto:** 8181  
**Servicio:** Task Scheduler → AryaESCPOS  
**Logs:** `C:\Program Files\AryaESCPOS\logs\`  

---

**¡Ya tienes un instalador profesional!** 🎉
