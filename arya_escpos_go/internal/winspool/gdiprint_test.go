package winspool

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"aryaescpos/internal/contract"
)

// --- struct layout sanity checks -------------------------------------------
//
// These pin devmodeW/bitmapInfoHeader to the exact sizes of the Win32
// structs they mirror (DEVMODEW=220 bytes, BITMAPINFOHEADER=40 bytes,
// per wingdi.h) so a future field addition/reordering that breaks binary
// compatibility with GDI fails a test instead of corrupting memory that a
// real printer driver reads.

func TestDevmodeWSize(t *testing.T) {
	if got, want := unsafe.Sizeof(devmodeW{}), uintptr(220); got != want {
		t.Fatalf("unsafe.Sizeof(devmodeW{}) = %d, want %d (sizeof(DEVMODEW))", got, want)
	}
}

func TestBitmapInfoHeaderSize(t *testing.T) {
	if got, want := unsafe.Sizeof(bitmapInfoHeader{}), uintptr(40); got != want {
		t.Fatalf("unsafe.Sizeof(bitmapInfoHeader{}) = %d, want %d (sizeof(BITMAPINFOHEADER))", got, want)
	}
}

// --- applyPrintOptions -------------------------------------------------------

func TestApplyPrintOptions_Defaults(t *testing.T) {
	var dm devmodeW
	applyPrintOptions(&dm, contract.DocumentPrintOptions{})

	if dm.Fields != dmOrientationFlag|dmCopiesFlag|dmDuplexFlag|dmColorFlag {
		t.Errorf("Fields = %#x, want all four flags set", dm.Fields)
	}
	if dm.Orientation != dmOrientPortrait {
		t.Errorf("Orientation = %d, want DMORIENT_PORTRAIT (%d)", dm.Orientation, dmOrientPortrait)
	}
	if dm.Copies != 1 {
		t.Errorf("Copies = %d, want 1 (default/minimum)", dm.Copies)
	}
	if dm.Duplex != dmDuplexSimplex {
		t.Errorf("Duplex = %d, want DMDUP_SIMPLEX (%d)", dm.Duplex, dmDuplexSimplex)
	}
	if dm.Color != dmColorColorValue {
		t.Errorf("Color = %d, want DMCOLOR_COLOR (%d)", dm.Color, dmColorColorValue)
	}
}

func TestApplyPrintOptions_AllNonDefaults(t *testing.T) {
	var dm devmodeW
	applyPrintOptions(&dm, contract.DocumentPrintOptions{
		Orientation: "landscape",
		Copies:      3,
		Duplex:      "duplex",
		Color:       "grayscale",
	})

	if dm.Orientation != dmOrientLandscape {
		t.Errorf("Orientation = %d, want DMORIENT_LANDSCAPE (%d)", dm.Orientation, dmOrientLandscape)
	}
	if dm.Copies != 3 {
		t.Errorf("Copies = %d, want 3", dm.Copies)
	}
	if dm.Duplex != dmDuplexVertical {
		t.Errorf("Duplex = %d, want DMDUP_VERTICAL (%d)", dm.Duplex, dmDuplexVertical)
	}
	if dm.Color != dmColorMonochrome {
		t.Errorf("Color = %d, want DMCOLOR_MONOCHROME (%d)", dm.Color, dmColorMonochrome)
	}
}

func TestApplyPrintOptions_CaseInsensitive(t *testing.T) {
	var dm devmodeW
	applyPrintOptions(&dm, contract.DocumentPrintOptions{
		Orientation: "LANDSCAPE",
		Duplex:      "Duplex",
		Color:       "GrayScale",
	})

	if dm.Orientation != dmOrientLandscape {
		t.Errorf("Orientation = %d, want DMORIENT_LANDSCAPE (%d) regardless of case", dm.Orientation, dmOrientLandscape)
	}
	if dm.Duplex != dmDuplexVertical {
		t.Errorf("Duplex = %d, want DMDUP_VERTICAL (%d) regardless of case", dm.Duplex, dmDuplexVertical)
	}
	if dm.Color != dmColorMonochrome {
		t.Errorf("Color = %d, want DMCOLOR_MONOCHROME (%d) regardless of case", dm.Color, dmColorMonochrome)
	}
}

func TestApplyPrintOptions_CopiesClampedToMinimumOne(t *testing.T) {
	for _, copies := range []int{0, -1, -100} {
		var dm devmodeW
		applyPrintOptions(&dm, contract.DocumentPrintOptions{Copies: copies})
		if dm.Copies != 1 {
			t.Errorf("Copies=%d: applyPrintOptions set dm.Copies = %d, want clamped to 1", copies, dm.Copies)
		}
	}
}

// --- minimalDevMode ----------------------------------------------------------

func TestMinimalDevMode_FieldsPopulated(t *testing.T) {
	buf, err := minimalDevMode("MyPrinter", contract.DocumentPrintOptions{Copies: 2})
	if err != nil {
		t.Fatalf("minimalDevMode() error = %v", err)
	}
	if len(buf) != int(unsafe.Sizeof(devmodeW{})) {
		t.Fatalf("minimalDevMode() buffer len = %d, want %d", len(buf), unsafe.Sizeof(devmodeW{}))
	}

	dm := (*devmodeW)(unsafe.Pointer(&buf[0]))
	if dm.SpecVersion != dmSpecVersion {
		t.Errorf("SpecVersion = %#x, want %#x", dm.SpecVersion, dmSpecVersion)
	}
	if dm.DriverVersion != 1 {
		t.Errorf("DriverVersion = %d, want 1", dm.DriverVersion)
	}
	if dm.Size != uint16(unsafe.Sizeof(devmodeW{})) {
		t.Errorf("Size = %d, want %d", dm.Size, unsafe.Sizeof(devmodeW{}))
	}
	if dm.DriverExtra != 0 {
		t.Errorf("DriverExtra = %d, want 0", dm.DriverExtra)
	}
	if dm.Copies != 2 {
		t.Errorf("Copies = %d, want 2 (applyPrintOptions should still run)", dm.Copies)
	}

	name := utf16ToString(dm.DeviceName[:])
	if name != "MyPrinter" {
		t.Errorf("DeviceName = %q, want %q", name, "MyPrinter")
	}
}

func TestMinimalDevMode_LongNameTruncatedAndNullTerminated(t *testing.T) {
	longName := strings.Repeat("A", 40) // longer than CCHDEVICENAME (32)
	buf, err := minimalDevMode(longName, contract.DocumentPrintOptions{})
	if err != nil {
		t.Fatalf("minimalDevMode() error = %v", err)
	}

	dm := (*devmodeW)(unsafe.Pointer(&buf[0]))
	if dm.DeviceName[cchDeviceName-1] != 0 {
		t.Fatalf("DeviceName[%d] = %d, want 0 (must stay null-terminated even when truncated)", cchDeviceName-1, dm.DeviceName[cchDeviceName-1])
	}
	name := utf16ToString(dm.DeviceName[:])
	if len(name) >= cchDeviceName {
		t.Errorf("DeviceName decoded length = %d, want < %d", len(name), cchDeviceName)
	}
	if !strings.HasPrefix(longName, name) {
		t.Errorf("DeviceName = %q, want a prefix of the original name", name)
	}
}

func utf16ToString(buf []uint16) string {
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(utf16.Decode(buf[:n]))
}

// --- toBGRA -------------------------------------------------------------------

func TestToBGRA_FastPath(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	// Row 0: red, green. Row 1: blue, white.
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	img.Set(0, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	buf, stride := toBGRA(img)
	if stride != 8 {
		t.Fatalf("stride = %d, want 8 (2 px * 4 bytes)", stride)
	}
	if len(buf) != stride*2 {
		t.Fatalf("len(buf) = %d, want %d", len(buf), stride*2)
	}

	// Row 0, pixel 0 (red) -> B,G,R,_ = 0,0,255,_
	if buf[0] != 0 || buf[1] != 0 || buf[2] != 255 {
		t.Errorf("row0 px0 (red) BGR = %d,%d,%d, want 0,0,255", buf[0], buf[1], buf[2])
	}
	// Row 0, pixel 1 (green) -> B,G,R,_ = 0,255,0,_
	if buf[4] != 0 || buf[5] != 255 || buf[6] != 0 {
		t.Errorf("row0 px1 (green) BGR = %d,%d,%d, want 0,255,0", buf[4], buf[5], buf[6])
	}
	// Row 1 (offset = stride), pixel 0 (blue) -> B,G,R,_ = 255,0,0,_
	if buf[8] != 255 || buf[9] != 0 || buf[10] != 0 {
		t.Errorf("row1 px0 (blue) BGR = %d,%d,%d, want 255,0,0", buf[8], buf[9], buf[10])
	}
	// No row flipping: toBGRA must preserve the top-down order as-is
	// (negative biHeight in bitmapInfoHeader is what tells GDI these rows
	// are already top-down, see PrintPageImage).
}

func TestToBGRA_FallbackPathForNonRGBAImages(t *testing.T) {
	// image.NRGBA doesn't hit the *image.RGBA fast path, exercising the
	// generic img.At()-based loop instead.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	img.Set(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 255})

	buf, stride := toBGRA(img)
	if stride != 8 {
		t.Fatalf("stride = %d, want 8", stride)
	}
	if buf[0] != 30 || buf[1] != 20 || buf[2] != 10 {
		t.Errorf("px0 BGR = %d,%d,%d, want 30,20,10", buf[0], buf[1], buf[2])
	}
	if buf[4] != 60 || buf[5] != 50 || buf[6] != 40 {
		t.Errorf("px1 BGR = %d,%d,%d, want 60,50,40", buf[4], buf[5], buf[6])
	}
}

func TestToBGRA_FastPathSkippedForCroppedSubImage(t *testing.T) {
	// A SubImage has a non-zero Bounds().Min, which must route through the
	// fallback path (the fast path assumes Pix[0] is pixel (0,0)).
	base := image.NewRGBA(image.Rect(0, 0, 4, 4))
	base.Set(2, 2, color.RGBA{R: 9, G: 8, B: 7, A: 255})
	sub := base.SubImage(image.Rect(2, 2, 4, 4))

	buf, stride := toBGRA(sub)
	if stride != 8 {
		t.Fatalf("stride = %d, want 8 (2 px wide * 4 bytes)", stride)
	}
	if buf[0] != 7 || buf[1] != 8 || buf[2] != 9 {
		t.Errorf("px0 BGR = %d,%d,%d, want 7,8,9", buf[0], buf[1], buf[2])
	}
}

// --- BeginGDIPrintJob / not-found path ---------------------------------------

func TestBeginGDIPrintJob_UnknownPrinterReturnsError(t *testing.T) {
	_, err := BeginGDIPrintJob("This Printer Definitely Does Not Exist 12345", "test.pdf", contract.DocumentPrintOptions{})
	if err == nil {
		t.Fatal("BeginGDIPrintJob() with an unknown printer: expected an error, got nil")
	}
	if _, ok := err.(*contract.APIError); !ok {
		t.Fatalf("error type = %T, want *contract.APIError", err)
	}
}
