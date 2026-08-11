package document

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertImageToPDF(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "sample.png")

	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 10), B: 128, A: 128}) // partial alpha to exercise RGB-forcing
		}
	}

	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("create sample image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode sample png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close sample image: %v", err)
	}

	pdfPath, err := ConvertImageToPDF(imgPath)
	if err != nil {
		t.Fatalf("ConvertImageToPDF() error = %v", err)
	}
	defer os.Remove(pdfPath)

	st, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("output PDF not found: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("output PDF is empty")
	}
}

func TestConvertImageToPDF_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "not-an-image.png")
	if err := os.WriteFile(badPath, []byte("this is not image data"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	if _, err := ConvertImageToPDF(badPath); err == nil {
		t.Fatal("expected error for invalid image data, got nil")
	}
}
