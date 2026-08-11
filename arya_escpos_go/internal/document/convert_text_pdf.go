package document

import (
	"os"
	"strings"

	"github.com/go-pdf/fpdf"

	"aryaescpos/internal/contract"
)

// Fixed A4-in-points page geometry, matching the Python service's PyMuPDF
// page (fitz.open().new_page(width=595, height=842)) and its text rect
// fitz.Rect(50, 50, 545, 792).
const (
	textPageWidthPt  = 595.0
	textPageHeightPt = 842.0
	textMarginPt     = 50.0
	textFontSizePt   = 10.0
	textLineHeightPt = 12.0 // ~1.2x font size, standard single-spacing for 10pt monospace
)

// ConvertTextToPDF converts a plain-text file (.txt/.log/.csv) at inputPath
// into a single-page A4 PDF with a monospaced (Courier) 10pt font, mirroring
// the Python service's _convert_text_to_pdf.
//
// KNOWN LIMITATION, inherited on purpose from the Python service: the text
// is never paginated. PyMuPDF's page.insert_textbox() silently clips
// anything that doesn't fit the fixed rect instead of flowing to a new page,
// and that is exactly what happens here too — SetAutoPageBreak(false) means
// content that overflows the bottom margin is laid out past the visible
// page area and simply does not appear when the PDF is rendered/printed.
// This is a pre-existing contract of the service, not a bug introduced by
// this port; a paginated version is out of scope for this v1.
//
// The returned path is a temp file owned by the caller, who must remove it
// once done.
func ConvertTextToPDF(inputPath string) (string, error) {
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return "", contract.BadRequest("invalid_text_file", "No se pudo leer el archivo de texto: "+err.Error())
	}

	// Best-effort UTF-8 decoding equivalent to Python's
	// open(..., encoding="utf-8", errors="ignore"): invalid byte sequences
	// are dropped rather than causing a failure.
	text := strings.ToValidUTF8(string(raw), "")

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           fpdf.SizeType{Wd: textPageWidthPt, Ht: textPageHeightPt},
	})
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(textMarginPt, textMarginPt, textMarginPt)
	pdf.AddPage()
	pdf.SetFont("Courier", "", textFontSizePt)
	pdf.SetXY(textMarginPt, textMarginPt)

	cellWidth := textPageWidthPt - 2*textMarginPt
	pdf.MultiCell(cellWidth, textLineHeightPt, text, "", "L", false)

	if err := pdf.Error(); err != nil {
		return "", contract.BadGateway("text_conversion_failed", "No se pudo generar el PDF del texto", err.Error())
	}

	outPath, err := newTempPDFPath("aryaescpos-text")
	if err != nil {
		return "", contract.BadGateway("text_conversion_failed", "No se pudo reservar el archivo PDF de salida", err.Error())
	}
	if err := pdf.OutputFileAndClose(outPath); err != nil {
		os.Remove(outPath)
		return "", contract.BadGateway("text_conversion_failed", "No se pudo generar el PDF del texto", err.Error())
	}
	return outPath, nil
}
