// gdiprint.go adds native GDI printing (gdi32.dll, plus winspool.drv's
// DocumentPropertiesW) on top of the RAW printing already in winspool.go.
// It replicates arya_escpos/src/utils/pdf_printer.py::print_pdf_native's
// win32ui/win32con path — CreateDC -> apply DEVMODE -> StartDoc -> per page
// (StartPage -> scale + center + StretchDIBits -> EndPage) -> EndDoc -> DeleteDC
// — using raw golang.org/x/sys/windows.NewLazyDLL bindings, same style and
// no-CGO constraint as the rest of this package.
//
// internal/pdfrender renders PDF pages to *image.RGBA bitmaps; this file
// only knows how to put an already-rendered image.Image on a printer DC. The
// two are wired together by internal/document/pdf_print.go.
package winspool

import (
	"fmt"
	"image"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"aryaescpos/internal/contract"
)

// --- DEVMODE field flags/values (wingdi.h) ---------------------------------

const (
	dmOrientationFlag uint32 = 0x00000001 // DM_ORIENTATION
	dmCopiesFlag      uint32 = 0x00000100 // DM_COPIES
	dmColorFlag       uint32 = 0x00000800 // DM_COLOR
	dmDuplexFlag      uint32 = 0x00001000 // DM_DUPLEX

	dmOrientPortrait  int16 = 1 // DMORIENT_PORTRAIT
	dmOrientLandscape int16 = 2 // DMORIENT_LANDSCAPE

	dmDuplexSimplex  int16 = 1 // DMDUP_SIMPLEX
	dmDuplexVertical int16 = 3 // DMDUP_VERTICAL

	dmColorMonochrome int16 = 1 // DMCOLOR_MONOCHROME
	dmColorColorValue int16 = 2 // DMCOLOR_COLOR

	dmSpecVersion = 0x0401 // DM_SPECVERSION, stable since Windows 95/NT4

	documentPropertiesOutBuffer uint32 = 0x00000002 // DM_OUT_BUFFER
	documentPropertiesInBuffer  uint32 = 0x00000008 // DM_IN_BUFFER
)

const (
	cchDeviceName = 32 // CCHDEVICENAME
	cchFormName   = 32 // CCHFORMNAME
)

// devmodeW mirrors DEVMODEW (wingdi.h) field for field, matching the layout
// convention documented in winspool.go's header comment (no explicit
// #pragma pack in the Win32 header, so ordinary Go struct layout on amd64
// already matches: 220 bytes total, no manual padding needed).
type devmodeW struct {
	DeviceName    [cchDeviceName]uint16
	SpecVersion   uint16
	DriverVersion uint16
	Size          uint16
	DriverExtra   uint16
	Fields        uint32

	// Printer-relevant view of the union that in DEVMODE also covers
	// display-only fields (dmPosition/dmDisplayOrientation/
	// dmDisplayFixedOutput) — same 16 bytes either way.
	Orientation   int16
	PaperSize     int16
	PaperLength   int16
	PaperWidth    int16
	Scale         int16
	Copies        int16
	DefaultSource int16
	PrintQuality  int16

	Color       int16
	Duplex      int16
	YResolution int16
	TTOption    int16
	Collate     int16

	FormName [cchFormName]uint16

	LogPixels        uint16
	BitsPerPel       uint32
	PelsWidth        uint32
	PelsHeight       uint32
	DisplayFlags     uint32 // union with dmNup
	DisplayFrequency uint32
	ICMMethod        uint32
	ICMIntent        uint32
	MediaType        uint32
	DitherType       uint32
	Reserved1        uint32
	Reserved2        uint32
	PanningWidth     uint32
	PanningHeight    uint32
}

// docInfoW mirrors DOCINFOW (wingdi.h), gdi32's StartDocW request — distinct
// from winspool.go's docInfo1W (DOC_INFO_1W, winspool.drv's StartDocPrinterW
// request): different API, different struct shape (this one carries cbSize
// and fwType, no Datatype/OutputFile that matter for GDI printing).
type docInfoW struct {
	CbSize   int32
	DocName  *uint16
	Output   *uint16
	Datatype *uint16
	FwType   uint32
}

// bitmapInfoHeader mirrors BITMAPINFOHEADER (wingdi.h). Only used with
// biBitCount=32 and biCompression=BI_RGB, which needs no color table, so
// this alone (without a trailing bmiColors array) is a valid BITMAPINFO for
// StretchDIBits.
type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

const (
	biRGB        uint32  = 0          // BI_RGB
	dibRGBColors uintptr = 0          // DIB_RGB_COLORS
	srcCopyROP   uintptr = 0x00CC0020 // SRCCOPY

	gdiHorzRes = 8  // HORZRES
	gdiVertRes = 10 // VERTRES
)

var (
	gdi32DLL = windows.NewLazyDLL("gdi32.dll")

	procCreateDCW     = gdi32DLL.NewProc("CreateDCW")
	procStartDocW     = gdi32DLL.NewProc("StartDocW")
	procStartPage     = gdi32DLL.NewProc("StartPage")
	procEndPage       = gdi32DLL.NewProc("EndPage")
	procEndDoc        = gdi32DLL.NewProc("EndDoc")
	procAbortDoc      = gdi32DLL.NewProc("AbortDoc")
	procDeleteDC      = gdi32DLL.NewProc("DeleteDC")
	procGetDeviceCaps = gdi32DLL.NewProc("GetDeviceCaps")
	procStretchDIBits = gdi32DLL.NewProc("StretchDIBits")

	// DocumentPropertiesW lives in winspool.drv, not gdi32.dll, so it's
	// bound off winspoolDLL, already declared in winspool.go (same
	// package).
	procDocumentPropertiesW = winspoolDLL.NewProc("DocumentPropertiesW")
)

// applyPrintOptions sets DM_ORIENTATION/DM_COPIES/DM_DUPLEX/DM_COLOR on dm
// from opts, using exactly the same mapping as pdf_printer.py's
// print_pdf_native: copies clamped to a minimum of 1, "landscape" (case
// insensitive) selects landscape orientation, "duplex" selects
// DMDUP_VERTICAL, "grayscale" selects DMCOLOR_MONOCHROME, everything else
// defaults to portrait/simplex/color.
func applyPrintOptions(dm *devmodeW, opts contract.DocumentPrintOptions) {
	dm.Fields |= dmOrientationFlag | dmCopiesFlag | dmDuplexFlag | dmColorFlag

	if strings.EqualFold(opts.Orientation, "landscape") {
		dm.Orientation = dmOrientLandscape
	} else {
		dm.Orientation = dmOrientPortrait
	}

	copies := opts.Copies
	if copies < 1 {
		copies = 1
	}
	dm.Copies = int16(copies)

	if strings.EqualFold(opts.Duplex, "duplex") {
		dm.Duplex = dmDuplexVertical
	} else {
		dm.Duplex = dmDuplexSimplex
	}

	if strings.EqualFold(opts.Color, "grayscale") {
		dm.Color = dmColorMonochrome
	} else {
		dm.Color = dmColorColorValue
	}
}

// buildDevMode returns a byte buffer holding a devmodeW ready to pass to
// CreateDCW, with orientation/copies/duplex/color applied on top of it.
//
// It first asks the driver for its own default DEVMODE via
// DocumentPropertiesW (mirroring what pywin32's hDC.GetDevMode() does under
// the hood) so paper size, driver-private settings, etc. all come from the
// driver instead of being left zeroed — only the four fields the caller
// asked for are overridden. If the driver doesn't cooperate (unusual, but
// not fatal — some virtual/legacy drivers don't implement
// DocumentPropertiesW), it falls back to a minimal hand-built DEVMODE
// carrying just those four fields plus the bookkeeping fields drivers
// generally expect to be populated (DeviceName/SpecVersion/DriverVersion/
// Size).
func buildDevMode(hPrinter windows.Handle, printerName string, opts contract.DocumentPrintOptions) ([]byte, error) {
	printerNamePtr, err := windows.UTF16PtrFromString(printerName)
	if err != nil {
		return nil, fmt.Errorf("winspool: invalid printer name %q: %w", printerName, err)
	}

	sizeRet, _, _ := procDocumentPropertiesW.Call(
		0, uintptr(hPrinter), uintptr(unsafe.Pointer(printerNamePtr)),
		0, 0, uintptr(documentPropertiesOutBuffer),
	)
	size := int32(sizeRet)
	if size <= 0 {
		return minimalDevMode(printerName, opts)
	}

	buf := make([]byte, size)
	r1, _, _ := procDocumentPropertiesW.Call(
		0, uintptr(hPrinter), uintptr(unsafe.Pointer(printerNamePtr)),
		uintptr(unsafe.Pointer(&buf[0])), 0, uintptr(documentPropertiesOutBuffer),
	)
	if int32(r1) < 0 {
		return minimalDevMode(printerName, opts)
	}

	dm := (*devmodeW)(unsafe.Pointer(&buf[0]))
	applyPrintOptions(dm, opts)

	// Let the driver validate/merge the modified fields back into the same
	// buffer — same round-trip pywin32's hDC.ResetDC(devmode) performs.
	// Best-effort: if this fails we still print with the buffer as-is.
	procDocumentPropertiesW.Call(
		0, uintptr(hPrinter), uintptr(unsafe.Pointer(printerNamePtr)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&buf[0])),
		uintptr(documentPropertiesInBuffer|documentPropertiesOutBuffer),
	)

	return buf, nil
}

func minimalDevMode(printerName string, opts contract.DocumentPrintOptions) ([]byte, error) {
	buf := make([]byte, unsafe.Sizeof(devmodeW{}))
	dm := (*devmodeW)(unsafe.Pointer(&buf[0]))

	name, err := windows.UTF16FromString(printerName)
	if err != nil {
		return nil, fmt.Errorf("winspool: invalid printer name %q: %w", printerName, err)
	}
	n := len(name)
	if n > cchDeviceName {
		n = cchDeviceName
	}
	copy(dm.DeviceName[:], name[:n])
	dm.DeviceName[cchDeviceName-1] = 0

	dm.SpecVersion = dmSpecVersion
	dm.DriverVersion = 1
	dm.Size = uint16(unsafe.Sizeof(devmodeW{}))
	dm.DriverExtra = 0

	applyPrintOptions(dm, opts)
	return buf, nil
}

// GDIPrintJob is an open native Windows print job created by
// BeginGDIPrintJob: CreateDCW + StartDocW already done, one or more pages
// pending via PrintPageImage, closed by End (success) or Abort (failure
// partway through).
type GDIPrintJob struct {
	hdc windows.Handle
}

// BeginGDIPrintJob opens printerName via CreateDCW (with a DEVMODE built
// from opts, see buildDevMode) and starts a print job named docName on it.
// Returns contract.BadGateway if the printer can't be opened (not found,
// broken driver) or the job can't be started.
func BeginGDIPrintJob(printerName, docName string, opts contract.DocumentPrintOptions) (*GDIPrintJob, error) {
	hPrinter, err := openPrinter(printerName)
	if err != nil {
		return nil, err
	}
	devModeBuf, dmErr := buildDevMode(hPrinter, printerName, opts)
	closePrinter(hPrinter)
	if dmErr != nil {
		return nil, dmErr
	}

	printerNamePtr, err := windows.UTF16PtrFromString(printerName)
	if err != nil {
		return nil, fmt.Errorf("winspool: invalid printer name %q: %w", printerName, err)
	}
	driverPtr, err := windows.UTF16PtrFromString("WINSPOOL")
	if err != nil {
		return nil, err
	}

	var devModePtr uintptr
	if len(devModeBuf) > 0 {
		devModePtr = uintptr(unsafe.Pointer(&devModeBuf[0]))
	}

	r1, _, e1 := procCreateDCW.Call(
		uintptr(unsafe.Pointer(driverPtr)),
		uintptr(unsafe.Pointer(printerNamePtr)),
		0,
		devModePtr,
	)
	if r1 == 0 {
		return nil, contract.BadGateway(
			"gdi_create_dc_failed",
			fmt.Sprintf("No se pudo abrir la impresora %q para impresion nativa (GDI)", printerName),
			fmt.Sprintf("CreateDCW failed: %v", e1),
		)
	}
	hdc := windows.Handle(r1)

	docNamePtr, err := windows.UTF16PtrFromString(docName)
	if err != nil {
		procDeleteDC.Call(uintptr(hdc))
		return nil, fmt.Errorf("winspool: invalid doc name %q: %w", docName, err)
	}
	di := docInfoW{
		CbSize:  int32(unsafe.Sizeof(docInfoW{})),
		DocName: docNamePtr,
	}

	r1, _, e1 = procStartDocW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&di)))
	if int32(r1) <= 0 {
		procDeleteDC.Call(uintptr(hdc))
		return nil, contract.BadGateway(
			"gdi_start_doc_failed",
			fmt.Sprintf("No se pudo iniciar el trabajo de impresion nativa en %q", printerName),
			fmt.Sprintf("StartDocW failed: %v", e1),
		)
	}

	return &GDIPrintJob{hdc: hdc}, nil
}

// PrintPageImage renders one page of img onto the print job: StartPage,
// scale img to fit the printer's imprintable area while preserving aspect
// ratio and centering it (same formula as print_pdf_native: scale =
// min(printerW/imgW, printerH/imgH)), StretchDIBits, EndPage.
func (j *GDIPrintJob) PrintPageImage(img image.Image) error {
	r1, _, e1 := procStartPage.Call(uintptr(j.hdc))
	if int32(r1) <= 0 {
		return contract.BadGateway("gdi_start_page_failed", "StartPage fallo durante la impresion nativa", fmt.Sprintf("%v", e1))
	}

	printerW, _, _ := procGetDeviceCaps.Call(uintptr(j.hdc), uintptr(gdiHorzRes))
	printerH, _, _ := procGetDeviceCaps.Call(uintptr(j.hdc), uintptr(gdiVertRes))

	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	if imgW <= 0 || imgH <= 0 || printerW == 0 || printerH == 0 {
		procEndPage.Call(uintptr(j.hdc))
		return fmt.Errorf("winspool: invalid dimensions for StretchDIBits (image %dx%d, printer %dx%d)", imgW, imgH, printerW, printerH)
	}

	scaleX := float64(printerW) / float64(imgW)
	scaleY := float64(printerH) / float64(imgH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	scaledW := int(float64(imgW) * scale)
	scaledH := int(float64(imgH) * scale)
	x := (int(printerW) - scaledW) / 2
	y := (int(printerH) - scaledH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	bgra, stride := toBGRA(img)

	bmi := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(imgW),
		Height:      -int32(imgH), // negative = top-down DIB, matches image.RGBA's row order (see comment on toBGRA)
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
		SizeImage:   uint32(stride * imgH),
	}

	r1, _, e1 = procStretchDIBits.Call(
		uintptr(j.hdc),
		uintptr(x), uintptr(y), uintptr(scaledW), uintptr(scaledH),
		0, 0, uintptr(imgW), uintptr(imgH),
		uintptr(unsafe.Pointer(&bgra[0])),
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		srcCopyROP,
	)
	if r1 == 0 {
		procEndPage.Call(uintptr(j.hdc))
		return contract.BadGateway("gdi_stretchdibits_failed", "No se pudo dibujar la pagina en la impresora", fmt.Sprintf("StretchDIBits failed: %v", e1))
	}

	r1, _, e1 = procEndPage.Call(uintptr(j.hdc))
	if int32(r1) <= 0 {
		return contract.BadGateway("gdi_end_page_failed", "EndPage fallo durante la impresion nativa", fmt.Sprintf("%v", e1))
	}
	return nil
}

// End finishes the print job (EndDoc) and always releases the DC
// (DeleteDC), regardless of whether EndDoc succeeded — same "always clean up
// the handle" guarantee as RawPrint in winspool.go.
func (j *GDIPrintJob) End() error {
	r1, _, e1 := procEndDoc.Call(uintptr(j.hdc))
	procDeleteDC.Call(uintptr(j.hdc))
	if int32(r1) <= 0 {
		return contract.BadGateway("gdi_end_doc_failed", "EndDoc fallo al finalizar la impresion nativa", fmt.Sprintf("%v", e1))
	}
	return nil
}

// Abort cancels the print job (AbortDoc) and releases the DC (DeleteDC).
// Call this instead of End when a page failed partway through, matching
// print_pdf_native's except-block hDC.AbortDoc() + finally-block
// hDC.DeleteDC(). Both calls are best-effort: there is nothing more useful
// to do with their errors at this point than to make sure the handle isn't
// leaked.
func (j *GDIPrintJob) Abort() {
	procAbortDoc.Call(uintptr(j.hdc))
	procDeleteDC.Call(uintptr(j.hdc))
}

// toBGRA converts img to a tightly packed top-down BGRA buffer (4 bytes/
// pixel, no row padding needed since width*4 is always a multiple of 4),
// the byte order a 32bpp BI_RGB Windows DIB expects. Takes a fast path when
// img is already an *image.RGBA with no cropping (true for everything
// internal/pdfrender produces), otherwise falls back to img.At() so this
// still works for any image.Image a test throws at it.
func toBGRA(img image.Image) ([]byte, int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	stride := w * 4
	buf := make([]byte, stride*h)

	if rgba, ok := img.(*image.RGBA); ok && rgba.Stride == stride && bounds.Min == (image.Point{}) {
		for row := 0; row < h; row++ {
			srcRow := rgba.Pix[row*rgba.Stride : row*rgba.Stride+stride]
			dstRow := buf[row*stride : row*stride+stride]
			for i := 0; i < stride; i += 4 {
				dstRow[i+0] = srcRow[i+2] // B
				dstRow[i+1] = srcRow[i+1] // G
				dstRow[i+2] = srcRow[i+0] // R
				dstRow[i+3] = 0           // reserved, BI_RGB ignores this byte
			}
		}
		return buf, stride
	}

	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			buf[i+0] = byte(b >> 8)
			buf[i+1] = byte(g >> 8)
			buf[i+2] = byte(r >> 8)
			buf[i+3] = 0
			i += 4
		}
	}
	return buf, stride
}
