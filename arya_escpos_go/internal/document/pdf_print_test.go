package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aryaescpos/internal/contract"
)

func TestPrintPDF_RequiresPrinterTool(t *testing.T) {
	// The native tier (internal/pdfrender + internal/winspool) needs no
	// external tool install, unlike the PDFtoPrinter.exe/SumatraPDF.exe
	// fallback tiers — but exercising any tier's actual success path still
	// needs a real Windows printer to send bits to, which this environment
	// may or may not have. Not run automatically either way.
	t.Skip("exercising an actual successful print requires a real Windows printer; not run automatically")
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

// TestPrintPDF_AllTiersFailReturnsBadGateway exercises the "everything
// failed" path with a nonexistent printer name and a deliberately invalid
// PDF (just a header/EOF, no real page data). This is deterministic
// regardless of the environment: the native tier can't open the fake PDF at
// all (fails in pdfrender.PageCount before ever touching the printer), and
// PDFtoPrinter.exe/SumatraPDF.exe — if installed — would fail too, since
// "Some Printer" doesn't exist. Since the native tier no longer depends on
// any external install (unlike the old PDFtoPrinter/SumatraPDF-only setup),
// PrintPDF now always has at least one real attempt to report, so the
// failure surfaces as contract.BadGateway, not the old
// contract.ServiceUnavailable "nothing is installed" case.
func TestPrintPDF_AllTiersFailReturnsBadGateway(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "fake.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n%%EOF"), 0o644); err != nil {
		t.Fatalf("write fake pdf: %v", err)
	}

	err := PrintPDF(context.Background(), pdfPath, "Some Printer", contract.DocumentPrintOptions{})
	if err == nil {
		t.Fatal("expected an error when every print tier fails")
	}
	apiErr, ok := err.(*contract.APIError)
	if !ok {
		t.Fatalf("error type = %T, want *contract.APIError", err)
	}
	if apiErr.HTTPStatus != 502 {
		t.Fatalf("HTTPStatus = %d, want 502", apiErr.HTTPStatus)
	}
	if apiErr.Code != "pdf_print_failed" {
		t.Fatalf("Code = %q, want %q", apiErr.Code, "pdf_print_failed")
	}
}
