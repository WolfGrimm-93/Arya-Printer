# Libs Folder

Esta carpeta contiene librerías binarias necesarias para el funcionamiento de Arya ESCPOS en Windows.

## libusb-1.0.dll

**Propósito:** Permite la comunicación con impresoras USB a través de PyUSB.

**Versión:** 1.0.29 (o superior)

**Descarga:** https://github.com/libusb/libusb/releases

**Instalación:**
1. Descargar libusb-1.0.29 (o la última versión)
2. Extraer el archivo `MS64/dll/libusb-1.0.dll` (para 64-bit)
3. Colocar en esta carpeta `libs/`

**Nota para Windows:**
- Si usas impresoras USB, también necesitas instalar el driver WinUSB usando Zadig
- Ver: https://zadig.akeo.ie/

## Uso en desarrollo

El script `main.py` agrega automáticamente esta carpeta al PATH del sistema para que PyUSB pueda encontrar la DLL.

## Uso en producción

El script `build_executable.py` incluye automáticamente esta carpeta en el ejecutable compilado.
