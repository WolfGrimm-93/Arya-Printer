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
	"aryaescpos/internal/pdfrender"
	"aryaescpos/internal/winspool"
)

// pdfPrintTimeout bounds a single PDFtoPrinter/SumatraPDF invocation,
// matching the Python service's subprocess.run(..., timeout=30).
const pdfPrintTimeout = 30 * time.Second

// nativePrintDPI matches pdf_printer.py::print_pdf_native's
// fitz.Matrix(150/72, 150/72) — 150 DPI page rendering before it's handed to
// GDI's StretchDIBits.
const nativePrintDPI = 150.0

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
	// Bundled copy first: installer.iss packages libs/SumatraPDF.exe with
	// every install (see libs/THIRD-PARTY-NOTICES.md for the GPLv3
	// redistribution notice), so this path is expected to exist without the
	// operator installing anything separately. The Program Files paths below
	// only matter for a dev machine that already had a system-wide SumatraPDF
	// install before this bundling was added.
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "libs", "SumatraPDF.exe")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	for _, p := range sumatraPDFCandidatePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// PrintPDF prints pdfPath to printerName, trying three tiers in order and
// returning as soon as one succeeds:
//
//  1. Native: internal/pdfrender (PDFium/WebAssembly) renders each page to a
//     bitmap and internal/winspool draws it on the printer's GDI device
//     context (see printPDFNative). This is the primary method — unlike the
//     other two, it needs no external tool installed on the machine, so it
//     is expected to be available on every install.
//  2. PDFtoPrinter.exe, if found (respects opts.Copies via /copies).
//  3. SumatraPDF.exe, if found (respects opts.Orientation == "landscape" via
//     -print-settings landscape).
//
// See the package doc comment for why color/duplex are not fully honored by
// tiers 2/3 (the native tier honors all four DocumentPrintOptions fields,
// see internal/winspool's applyPrintOptions). Returns contract.BadGateway
// listing every tier's failure if all three were tried and failed.
func PrintPDF(ctx context.Context, pdfPath, printerName string, opts contract.DocumentPrintOptions) error {
	if err := validatePrinterNameForExec(printerName); err != nil {
		return err
	}

	copies := opts.Copies
	if copies < 1 {
		copies = 1
	}

	var attempts []string

	if err := printPDFNative(pdfPath, printerName, opts); err == nil {
		return nil
	} else {
		attempts = append(attempts, fmt.Sprintf("Native (PDFium+GDI): %v", err))
	}

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

	// attempts always has at least the native tier's failure by this point
	// (it never returns early to a "not installed" branch: unlike
	// PDFtoPrinter/SumatraPDF, it needs no external install to attempt),
	// so this always reports a real failure across every tier that ran,
	// rather than the old "nothing is installed" framing that no longer
	// applies now that the native method is always available.
	if len(attempts) > 0 {
		return contract.BadGateway(
			"pdf_print_failed",
			"No se pudo imprimir el PDF: fallaron todos los métodos disponibles (nativo PDFium+GDI"+
				", PDFtoPrinter y SumatraPDF si estaban presentes)",
			strings.Join(attempts, " | "),
		)
	}
	// Defensive only: printPDFNative always appends to attempts on failure,
	// so this branch should be unreachable in practice.
	return contract.ServiceUnavailable(
		"pdf_printer_not_found",
		"No se pudo imprimir el PDF por ningún método disponible",
		"Esto no debería ocurrir: el método nativo (PDFium+GDI) no depende de ninguna instalación "+
			"externa. Si se ve este error, revisar printPDFNative en internal/document/pdf_print.go.",
	)
}

// printPDFNative prints pdfPath to printerName using the primary method:
// internal/pdfrender renders every page to a bitmap at nativePrintDPI via
// PDFium (WebAssembly, no CGO, no external process) and internal/winspool
// draws each one on the printer's GDI device context. Mirrors
// pdf_printer.py::print_pdf_native page by page: on any failure partway
// through, the in-progress print job is aborted (GDIPrintJob.Abort, which
// still releases the DC) rather than left dangling — no page renders happen
// before BeginGDIPrintJob has already opened the printer, so a failure to
// open the printer itself never touches pdfrender at all.
func printPDFNative(pdfPath, printerName string, opts contract.DocumentPrintOptions) error {
	pageCount, err := pdfrender.PageCount(pdfPath)
	if err != nil {
		return fmt.Errorf("no se pudo leer el PDF: %w", err)
	}
	if pageCount < 1 {
		return fmt.Errorf("el PDF no tiene páginas")
	}

	job, err := winspool.BeginGDIPrintJob(printerName, filepath.Base(pdfPath), opts)
	if err != nil {
		return err
	}

	for i := 0; i < pageCount; i++ {
		img, err := pdfrender.RenderPageToImage(pdfPath, i, nativePrintDPI)
		if err != nil {
			job.Abort()
			return fmt.Errorf("renderizando página %d de %d: %w", i+1, pageCount, err)
		}
		if err := job.PrintPageImage(img); err != nil {
			job.Abort()
			return fmt.Errorf("imprimiendo página %d de %d: %w", i+1, pageCount, err)
		}
	}

	return job.End()
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
