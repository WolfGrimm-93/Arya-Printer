package document

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aryaescpos/internal/contract"
)

// pdfPrintTimeout bounds a single PDFtoPrinter/SumatraPDF invocation,
// matching the Python service's subprocess.run(..., timeout=30).
const pdfPrintTimeout = 30 * time.Second

var pdfToPrinterCandidatePaths = []string{
	`C:\Program Files\PDFtoPrinter\PDFtoPrinter.exe`,
	`C:\Program Files (x86)\PDFtoPrinter\PDFtoPrinter.exe`,
}

var sumatraPDFCandidatePaths = []string{
	`C:\Program Files\SumatraPDF\SumatraPDF.exe`,
	`C:\Program Files (x86)\SumatraPDF\SumatraPDF.exe`,
}

func findPDFToPrinter() string {
	for _, p := range pdfToPrinterCandidatePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	// Same convention as the Python service: <install dir>/libs/PDFtoPrinter.exe.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "libs", "PDFtoPrinter.exe")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func findSumatraPDF() string {
	for _, p := range sumatraPDFCandidatePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// PrintPDF prints pdfPath to printerName, trying PDFtoPrinter.exe first
// (respects opts.Copies via /copies) and falling back to SumatraPDF.exe
// (respects opts.Orientation == "landscape" via -print-settings landscape).
// See the package doc comment for why color/duplex are not fully honored by
// either tool. Returns contract.ServiceUnavailable if neither tool is
// installed, or contract.BadGateway if both were tried and failed.
func PrintPDF(ctx context.Context, pdfPath, printerName string, opts contract.DocumentPrintOptions) error {
	if err := validatePrinterNameForExec(printerName); err != nil {
		return err
	}

	copies := opts.Copies
	if copies < 1 {
		copies = 1
	}

	var attempts []string

	if exe := findPDFToPrinter(); exe != "" {
		args := []string{pdfPath, printerName}
		if copies > 1 {
			args = append(args, "/copies", strconv.Itoa(copies))
		}
		if err := runPrintTool(ctx, exe, args); err == nil {
			return nil
		} else {
			attempts = append(attempts, fmt.Sprintf("PDFtoPrinter: %v", err))
		}
	}

	if exe := findSumatraPDF(); exe != "" {
		args := []string{"-print-to", printerName, "-silent"}
		if strings.EqualFold(opts.Orientation, "landscape") {
			args = append(args, "-print-settings", "landscape")
		}
		args = append(args, pdfPath)
		if err := runPrintTool(ctx, exe, args); err == nil {
			return nil
		} else {
			attempts = append(attempts, fmt.Sprintf("SumatraPDF: %v", err))
		}
	}

	if len(attempts) > 0 {
		return contract.BadGateway("pdf_print_failed", "No se pudo imprimir el PDF con ninguna herramienta disponible", strings.Join(attempts, " | "))
	}
	return contract.ServiceUnavailable(
		"pdf_printer_not_found",
		"No hay ninguna herramienta de impresión de PDF instalada",
		"No se encontró PDFtoPrinter.exe (C:\\Program Files\\PDFtoPrinter\\, C:\\Program Files (x86)\\PDFtoPrinter\\, "+
			"o libs\\ junto al ejecutable) ni SumatraPDF.exe (C:\\Program Files\\SumatraPDF\\, C:\\Program Files (x86)\\SumatraPDF\\). "+
			"Instalá al menos una de las dos para poder imprimir documentos.",
	)
}

// validatePrinterNameForExec rejects printer names that could be
// misinterpreted as a command-line flag by PDFtoPrinter.exe/SumatraPDF.exe
// instead of as a literal printer name. exec.Command on Windows does not go
// through a shell, so this is not command-injection in the classic sense,
// but a printerName starting with "-" or "/" would still be parsed as an
// option by either tool (e.g. SumatraPDF's own flags start with "-"), which
// can alter their behavior in ways the caller never intended.
func validatePrinterNameForExec(printerName string) error {
	if strings.HasPrefix(printerName, "-") || strings.HasPrefix(printerName, "/") {
		return contract.BadRequest(
			"invalid_printer_name",
			"El nombre de la impresora no puede empezar con '-' o '/': podría interpretarse como una opción de línea de comandos en lugar de un nombre de impresora",
		)
	}
	return nil
}

func runPrintTool(ctx context.Context, exe string, args []string) error {
	cctx, cancel := context.WithTimeout(ctx, pdfPrintTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, exe, args...)
	out, err := cmd.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timeout tras %s: %s", pdfPrintTimeout, strings.TrimSpace(string(out)))
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
