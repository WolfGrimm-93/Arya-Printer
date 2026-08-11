// Package pdfrender renders PDF pages to bitmaps using PDFium (Chrome's PDF
// engine) via github.com/klippa-app/go-pdfium in its "webassembly" mode
// (github.com/tetratelabs/wazero, no CGO): PDFium is compiled to a .wasm
// module embedded in the go-pdfium module itself (go:embed) and executed in
// a sandboxed runtime, so this package needs neither a system PDFium/MuPDF
// install nor CGO, and can't crash the host process the way a CGO binding
// could.
//
// This replaces PyMuPDF (fitz) from the Python service's
// arya_escpos/src/utils/pdf_printer.py::print_pdf_native, which is AGPL-3.0
// (or a paid Artifex commercial license) and therefore not embeddable here
// without relicensing the whole Go service under AGPL. PDFium itself is
// BSD-3-Clause and go-pdfium is Apache-2.0 — both fully permissive.
package pdfrender

import (
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// instanceTimeout bounds how long a caller waits for a free PDFium worker
// from the pool before giving up.
const instanceTimeout = 30 * time.Second

var (
	poolOnce sync.Once
	pool     pdfium.Pool
	poolErr  error
)

// getPool lazily initializes a single package-wide webassembly worker pool
// on first use and reuses it for the lifetime of the process (aryaprinter
// runs as a long-lived Windows service/foreground process). Compiling the
// embedded PDFium .wasm module takes real time, so paying that cost once
// per process instead of once per print job matters.
func getPool() (pdfium.Pool, error) {
	poolOnce.Do(func() {
		pool, poolErr = webassembly.Init(webassembly.Config{
			MinIdle:  1,
			MaxIdle:  1,
			MaxTotal: 2,
		})
	})
	return pool, poolErr
}

// PageCount returns the number of pages in the PDF at pdfPath, matching
// Python's len(fitz.open(pdf_path)).
func PageCount(pdfPath string) (int, error) {
	p, err := getPool()
	if err != nil {
		return 0, fmt.Errorf("pdfrender: init pdfium pool: %w", err)
	}

	instance, err := p.GetInstance(instanceTimeout)
	if err != nil {
		return 0, fmt.Errorf("pdfrender: get pdfium instance: %w", err)
	}
	defer instance.Close()

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return 0, fmt.Errorf("pdfrender: read %s: %w", pdfPath, err)
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return 0, fmt.Errorf("pdfrender: open %s: %w", pdfPath, err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return 0, fmt.Errorf("pdfrender: get page count of %s: %w", pdfPath, err)
	}
	return count.PageCount, nil
}

// RenderPageToImage renders page pageIndex (0-based) of the PDF at pdfPath
// to an RGBA bitmap at the given DPI. Matches
// pdf_printer.py::print_pdf_native's fitz.Matrix(dpi/72, dpi/72) +
// page.get_pixmap(matrix=mat, alpha=False) call (150 DPI in production use,
// see internal/document/pdf_print.go).
//
// The returned image owns its pixel buffer: go-pdfium's webassembly mode
// hands back a *image.RGBA whose Pix slice is a direct view into the
// WebAssembly module's linear memory, invalidated as soon as the render
// result is released, so the pixels are copied out before that happens.
func RenderPageToImage(pdfPath string, pageIndex int, dpi float64) (image.Image, error) {
	p, err := getPool()
	if err != nil {
		return nil, fmt.Errorf("pdfrender: init pdfium pool: %w", err)
	}

	instance, err := p.GetInstance(instanceTimeout)
	if err != nil {
		return nil, fmt.Errorf("pdfrender: get pdfium instance: %w", err)
	}
	defer instance.Close()

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("pdfrender: read %s: %w", pdfPath, err)
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return nil, fmt.Errorf("pdfrender: open %s: %w", pdfPath, err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	render, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIndex,
			},
		},
		DPI: int(dpi),
	})
	if err != nil {
		return nil, fmt.Errorf("pdfrender: render page %d of %s: %w", pageIndex, pdfPath, err)
	}
	defer render.Cleanup()

	src := render.Result.Image
	out := image.NewRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out, nil
}
