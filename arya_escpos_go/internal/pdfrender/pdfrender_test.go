package pdfrender

import (
	"path/filepath"
	"testing"

	"github.com/go-pdf/fpdf"
)

// pageWidthPt/pageHeightPt match convert_text_pdf.go's fixed A4-in-points
// page geometry, giving this test a page size independent of fpdf's default
// unit/format assumptions.
const (
	pageWidthPt  = 595.0
	pageHeightPt = 842.0
	testDPI      = 150.0
)

// newTestPDF writes a minimal one-page PDF (via github.com/go-pdf/fpdf,
// already a project dependency, see internal/document/convert_text_pdf.go)
// to a temp file and returns its path.
func newTestPDF(t *testing.T) string {
	t.Helper()

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           fpdf.SizeType{Wd: pageWidthPt, Ht: pageHeightPt},
	})
	pdf.AddPage()
	pdf.SetFont("Courier", "", 12)
	pdf.SetXY(50, 50)
	pdf.Cell(200, 20, "pdfrender test page")

	if err := pdf.Error(); err != nil {
		t.Fatalf("building test PDF: %v", err)
	}

	path := filepath.Join(t.TempDir(), "pdfrender-test.pdf")
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatalf("writing test PDF: %v", err)
	}
	return path
}

func TestPageCount(t *testing.T) {
	path := newTestPDF(t)

	count, err := PageCount(path)
	if err != nil {
		t.Fatalf("PageCount() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("PageCount() = %d, want 1", count)
	}
}

func TestPageCount_MissingFile(t *testing.T) {
	if _, err := PageCount(filepath.Join(t.TempDir(), "does-not-exist.pdf")); err == nil {
		t.Fatal("PageCount() on a missing file: expected an error, got nil")
	}
}

func TestRenderPageToImage(t *testing.T) {
	path := newTestPDF(t)

	img, err := RenderPageToImage(path, 0, testDPI)
	if err != nil {
		t.Fatalf("RenderPageToImage() error = %v", err)
	}

	bounds := img.Bounds()
	dpi := float64(testDPI)
	wantW := int(float64(pageWidthPt) * dpi / 72)
	wantH := int(float64(pageHeightPt) * dpi / 72)

	// Allow a small tolerance for PDFium's own rounding of points -> pixels.
	const tolerance = 3
	if abs(bounds.Dx()-wantW) > tolerance {
		t.Errorf("rendered width = %d, want ~%d (+/-%d)", bounds.Dx(), wantW, tolerance)
	}
	if abs(bounds.Dy()-wantH) > tolerance {
		t.Errorf("rendered height = %d, want ~%d (+/-%d)", bounds.Dy(), wantH, tolerance)
	}
}

func TestRenderPageToImage_InvalidPageIndex(t *testing.T) {
	path := newTestPDF(t)

	if _, err := RenderPageToImage(path, 5, testDPI); err == nil {
		t.Fatal("RenderPageToImage() with an out-of-range page index: expected an error, got nil")
	}
}

func TestRenderPageToImage_MissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.pdf")
	if _, err := RenderPageToImage(missing, 0, testDPI); err == nil {
		t.Fatal("RenderPageToImage() on a missing file: expected an error, got nil")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
