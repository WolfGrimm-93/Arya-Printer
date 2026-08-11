# Avisos de terceros — binarios vendorizados en `libs/`

Este directorio incluye binarios de terceros redistribuidos junto con `arya_escpos_go`,
cada uno invocado por el servicio como **proceso externo separado** (via `os/exec`), nunca
enlazado/embebido dentro del binario Go — esto es intencional: evita que la licencia
copyleft de estas herramientas se extienda al código de Arya Printer (ver razonamiento en
`Proyectos/Arya Printer/Arya Printer - Reescritura en Go (arya_escpos_go).md` en Obsidian).

## SumatraPDF.exe

- **Versión**: 3.6.1 (portable, 64-bit)
- **Autor**: Krzysztof Kowalczyk y colaboradores
- **Licencia**: GNU General Public License v3 (GPLv3)
- **Código fuente**: https://github.com/sumatrapdfreader/sumatrapdf (tag `3.6.1`)
- **Descargado desde**: https://www.sumatrapdfreader.org/dl/rel/3.6.1/SumatraPDF-3.6.1-64.zip (sitio oficial)
- **Uso en este proyecto**: fallback de impresión de PDF (`internal/document/pdf_print.go`) cuando
  el renderizado nativo (PDFium/go-pdfium, Apache-2.0) no está disponible o falla. Se ejecuta como
  proceso hijo (`-print-to <impresora> -silent`), nunca se compila ni enlaza dentro de `aryaprinter.exe`.
- **Texto completo de la licencia GPLv3**: https://www.gnu.org/licenses/gpl-3.0.txt

## libusb-1.0.dll

- **Licencia**: GNU Lesser General Public License v2.1 (LGPL-2.1)
- **Código fuente**: https://github.com/libusb/libusb
- **Uso en este proyecto**: `internal/hwadapter/usb.go` la carga dinámicamente vía
  `golang.org/x/sys/windows.NewLazyDLL` (enlace dinámico en tiempo de ejecución, no estático) —
  el modo de uso que la propia LGPL contempla explícitamente sin imponer copyleft sobre el
  programa que la consume.
- **Texto completo de la licencia LGPL-2.1**: https://www.gnu.org/licenses/lgpl-2.1.txt

## mkcert.exe

- **Licencia**: BSD 3-Clause
- **Código fuente**: https://github.com/FiloSottile/mkcert
- **Uso en este proyecto**: `internal/ssl/mkcert.go`, generación de certificados TLS locales para
  `--setup-ssl`.
