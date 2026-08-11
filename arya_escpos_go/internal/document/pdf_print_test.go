package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aryaescpos/internal/contract"
)

func TestPrintPDF_RequiresPrinterTool(t *testing.T) {
	if findPDFToPrinter() == "" && findSumatraPDF() == "" {
		t.Skip("requires PDFtoPrinter.exe or SumatraPDF.exe, not available in this environment")
	}
	t.Skip("PDFtoPrinter/SumatraPDF found but exercising an actual print requires a real Windows printer; not run automatically")
}

func TestPrintPDF_RejectsPrinterNameLookingLikeAFlag(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "fake.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n%%EOF"), 0o644); err != nil {
		t.Fatalf("write fake pdf: %v", err)
	}

	cases := []string{"-print-settings", "/copies", "-", "/"}
	for _, printerName := range cases {
		err := PrintPDF(context.Background(), pdfPath, printerName, contract.DocumentPrintOptions{})
		if err == nil {
			t.Fatalf("PrintPDF(printerName=%q) = nil error, want rejection", printerName)
		}
		apiErr, ok := err.(*contract.APIError)
		if !ok {
			t.Fatalf("PrintPDF(printerName=%q) error type = %T, want *contract.APIError", printerName, err)
		}
		if apiErr.HTTPStatus != 400 {
			t.Fatalf("PrintPDF(printerName=%q) HTTPStatus = %d, want 400", printerName, apiErr.HTTPStatus)
		}
		if apiErr.Code != "invalid_printer_name" {
			t.Fatalf("PrintPDF(printerName=%q) Code = %q, want %q", printerName, apiErr.Code, "invalid_printer_name")
		}
	}
}

func TestPrintPDF_NoToolInstalledReturnsServiceUnavailable(t *testing.T) {
	if findPDFToPrinter() != "" || findSumatraPDF() != "" {
		t.Skip("a PDF printing tool is installed in this environment; cannot exercise the not-found path")
	}

	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "fake.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n%%EOF"), 0o644); err != nil {
		t.Fatalf("write fake pdf: %v", err)
	}

	err := PrintPDF(context.Background(), pdfPath, "Some Printer", contract.DocumentPrintOptions{})
	if err == nil {
		t.Fatal("expected an error when no PDF printing tool is installed")
	}
	apiErr, ok := err.(*contract.APIError)
	if !ok {
		t.Fatalf("error type = %T, want *contract.APIError", err)
	}
	if apiErr.HTTPStatus != 503 {
		t.Fatalf("HTTPStatus = %d, want 503", apiErr.HTTPStatus)
	}
	if apiErr.Code != "pdf_printer_not_found" {
		t.Fatalf("Code = %q, want %q", apiErr.Code, "pdf_printer_not_found")
	}
}
