// Package printsvc implements the contract service interfaces backed by
// concrete OS/hardware integrations (Windows spooler, USB/network/serial
// adapters, document conversion). This file implements contract.
// DocumentPrinter for POST /api/v1/print/document; internal/document does
// the actual conversion (LibreOffice/fpdf) and PDF printing
// (PDFtoPrinter/SumatraPDF) work, and internal/winspool (Agent B) implements
// contract.WindowsPrintQueue, consumed here only through that interface —
// this file never imports internal/winspool directly.
package printsvc

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"aryaescpos/internal/contract"
	"aryaescpos/internal/document"
)

// findRecentJobTimeout bounds how long PrintDocument waits for the spooler
// to show the just-submitted job before giving up on reporting a job_id,
// matching the Python service's get_job_id_from_queue default of 5s.
const findRecentJobTimeout = 5 * time.Second

type documentPrinter struct {
	wq contract.WindowsPrintQueue
}

// NewDocumentPrinter returns a contract.DocumentPrinter that converts
// non-PDF documents to PDF (LibreOffice headless for Word/Excel, fpdf for
// image/text), prints the resulting PDF via PDFtoPrinter/SumatraPDF, and
// uses wq — only through the contract.WindowsPrintQueue interface — to look
// up the resulting spooler job id.
func NewDocumentPrinter(wq contract.WindowsPrintQueue) contract.DocumentPrinter {
	return &documentPrinter{wq: wq}
}

func (d *documentPrinter) PrintDocument(ctx context.Context, localFilePath, originalFilename string, opts contract.DocumentPrintOptions) (*int, error) {
	docType := document.DetectType(originalFilename)

	pdfPath := localFilePath
	intermediatePath := "" // non-empty only if we created a temp PDF that must be cleaned up

	switch docType {
	case document.TypePDF:
		// No-op: print localFilePath directly, nothing to clean up.

	case document.TypeWord, document.TypeExcel:
		converted, err := document.ConvertOfficeToPDF(ctx, localFilePath)
		if err != nil {
			return nil, err
		}
		pdfPath = converted
		intermediatePath = converted

	case document.TypeImage:
		converted, err := document.ConvertImageToPDF(localFilePath)
		if err != nil {
			return nil, err
		}
		pdfPath = converted
		intermediatePath = converted

	case document.TypeText:
		converted, err := document.ConvertTextToPDF(localFilePath)
		if err != nil {
			return nil, err
		}
		pdfPath = converted
		intermediatePath = converted

	default:
		return nil, contract.BadRequest("unsupported_document_type", fmt.Sprintf("Tipo de documento no soportado: %s", originalFilename))
	}

	// Always clean up any intermediate PDF we created, even if printing
	// below fails. localFilePath itself is never touched — apiserver owns
	// its lifecycle.
	if intermediatePath != "" {
		defer func() {
			if err := os.Remove(intermediatePath); err != nil && !os.IsNotExist(err) {
				log.Printf("printsvc: no se pudo borrar el PDF intermedio %s: %v", intermediatePath, err)
			}
		}()
	}

	if err := document.PrintPDF(ctx, pdfPath, opts.PrinterName, opts); err != nil {
		return nil, err
	}

	docName := filepath.Base(pdfPath)
	jobID, err := d.wq.FindRecentJob(ctx, opts.PrinterName, docName, findRecentJobTimeout)
	if err != nil {
		// The document was already sent to the printer successfully; not
		// being able to confirm its spooler job id is a diagnostic gap, not
		// a print failure — report success with no job_id, same as the
		// Python service swallowing get_job_id_from_queue errors.
		log.Printf("printsvc: no se pudo determinar job_id para %q en %q: %v", docName, opts.PrinterName, err)
		return nil, nil
	}
	return jobID, nil
}
