package document

import (
	"path/filepath"
	"strings"
)

// Type is the coarse document category used to dispatch to the right
// converter, mirroring get_document_type() in the Python service's
// document_converter.py.
type Type string

const (
	TypePDF     Type = "pdf"     // no-op, printed as-is
	TypeWord    Type = "word"    // .doc, .docx -> LibreOffice headless
	TypeExcel   Type = "excel"   // .xls, .xlsx -> LibreOffice headless
	TypeImage   Type = "image"   // .jpg, .jpeg, .png, .bmp, .gif, .tiff -> fpdf
	TypeText    Type = "text"    // .txt, .log, .csv -> fpdf
	TypeUnknown Type = "unknown" // rejected by the caller with contract.BadRequest
)

// DetectType returns the document Type for filename based on its extension,
// using the exact same extension-to-type mapping as the Python service's
// get_document_type(). filename only needs to carry an extension; it does
// not need to exist on disk (apiserver passes the original uploaded
// filename, not the temp path, so the original extension is preserved even
// though the temp file's own name may differ).
func DetectType(filename string) Type {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return TypePDF
	case ".doc", ".docx":
		return TypeWord
	case ".xls", ".xlsx":
		return TypeExcel
	case ".jpg", ".jpeg", ".png", ".bmp", ".gif", ".tiff":
		return TypeImage
	case ".txt", ".log", ".csv":
		return TypeText
	default:
		return TypeUnknown
	}
}
