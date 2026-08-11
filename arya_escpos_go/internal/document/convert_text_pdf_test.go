package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertTextToPDF(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "notes.txt")
	content := "Linea 1\nLinea 2 con acentos: ñ á é\nLinea 3\n"
	if err := os.WriteFile(txtPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample text: %v", err)
	}

	pdfPath, err := ConvertTextToPDF(txtPath)
	if err != nil {
		t.Fatalf("ConvertTextToPDF() error = %v", err)
	}
	defer os.Remove(pdfPath)

	st, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("output PDF not found: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("output PDF is empty")
	}
	if !strings.HasSuffix(pdfPath, ".pdf") {
		t.Fatalf("output path %q does not end in .pdf", pdfPath)
	}
}

func TestConvertTextToPDF_InvalidUTF8IsDroppedNotFatal(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "bad-encoding.txt")
	// 0xFF is not valid UTF-8 on its own.
	content := append([]byte("valid text "), 0xFF, 0xFE)
	content = append(content, []byte(" more valid text")...)
	if err := os.WriteFile(txtPath, content, 0o644); err != nil {
		t.Fatalf("write sample text: %v", err)
	}

	pdfPath, err := ConvertTextToPDF(txtPath)
	if err != nil {
		t.Fatalf("ConvertTextToPDF() error = %v, want success (invalid bytes should be dropped, best-effort)", err)
	}
	defer os.Remove(pdfPath)

	if st, err := os.Stat(pdfPath); err != nil || st.Size() == 0 {
		t.Fatalf("expected non-empty output PDF, stat err = %v", err)
	}
}
