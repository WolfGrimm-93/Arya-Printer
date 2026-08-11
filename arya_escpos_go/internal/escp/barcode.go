package escp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/code39"
	"github.com/boombuler/barcode/ean"
	xdraw "golang.org/x/image/draw"

	"aryaescpos/internal/contract"
)

const (
	defaultBarcodeType   = "code39"
	defaultBarcodeWidth  = 300
	defaultBarcodeHeight = 48
	// blackIdx is the palette index meaning "print this dot" in the 1-bit
	// conversion below: monoPalette[0] = white, monoPalette[1] = black.
	blackIdx = 1

	// maxBarcodeDimensionDots and maxBarcodePixels are hard upper bounds on
	// width_dots/height_dots (BarcodeItem fields that come straight off the
	// HTTP request body, no image decode involved) checked BEFORE
	// image.NewRGBA/image.NewPaletted ever allocate a pixel buffer. Unlike
	// the header-image path in internal/escpos, there is no decode step to
	// gate on here — a client can request an arbitrarily large canvas with
	// nothing but two integers, so this is the only guard. 4096 per axis
	// and 16,000,000 total pixels are both far beyond any real barcode
	// label (defaults are 300x48 = 14,400 pixels).
	maxBarcodeDimensionDots = 4096
	maxBarcodePixels        = 16_000_000
)

var monoPalette = color.Palette{color.White, color.Black}

// Barcode renders item as ESC/P double-density raster graphics (ESC * 1) and
// appends it to the buffer, mirroring matrix_command_builder.py's barcode().
//
// Behavioral contract preserved from the Python service, without exception:
// if rendering fails for ANY reason — unsupported Type, data invalid for the
// symbology, dimensions too small, anything — this degrades to a plain-text
// label "[TYPE: data]" (encoded with the Builder's configured
// character-replacement encoding) instead of returning an error that would
// abort the whole print request. Callers must not expect an error return
// here; there isn't one, by design.
func (b *Builder) Barcode(item contract.BarcodeItem) *Builder {
	barcodeType := item.Type
	if barcodeType == "" {
		barcodeType = defaultBarcodeType
	}
	widthDots := item.WidthDots
	if widthDots <= 0 {
		widthDots = defaultBarcodeWidth
	}
	heightDots := item.HeightDots
	if heightDots <= 0 {
		heightDots = defaultBarcodeHeight
	}

	img, err := renderBarcode1Bit(item.Data, barcodeType, widthDots, heightDots)
	if err != nil {
		label := fmt.Sprintf("[%s: %s]", strings.ToUpper(barcodeType), item.Data)
		b.buf.Write(b.encode(label))
		b.buf.WriteByte(0x0a)
		return b
	}

	b.buf.Write(imageToESCPRaster(img))
	b.buf.WriteByte(0x0a)
	return b
}

// renderBarcode1Bit encodes data as the given symbology, stretches it
// directly to widthDots x heightDots (ignoring the barcode's native aspect
// ratio — this is a direct stretch, same as the Python service's
// img.resize((width_dots, height_dots), Image.LANCZOS) applied
// unconditionally), and dithers it to 1-bit.
//
// Supported types: code39, code128, ean13, ean8, upca/upc.
func renderBarcode1Bit(data, barcodeType string, widthDots, heightDots int) (*image.Paletted, error) {
	// Hard dimension guard FIRST, before any encoding or buffer allocation
	// work: width_dots/height_dots come straight from the HTTP request body
	// with no decode step to gate on (unlike internal/escpos's image path),
	// so this is the only thing standing between a two-integer request body
	// and an attempted multi-gigabyte allocation.
	if widthDots <= 0 || heightDots <= 0 {
		return nil, fmt.Errorf("escp: invalid barcode dimensions %dx%d", widthDots, heightDots)
	}
	if widthDots > maxBarcodeDimensionDots || heightDots > maxBarcodeDimensionDots {
		return nil, fmt.Errorf("escp: barcode dimensions %dx%d exceed the maximum of %d dots per axis", widthDots, heightDots, maxBarcodeDimensionDots)
	}
	if int64(widthDots)*int64(heightDots) > maxBarcodePixels {
		return nil, fmt.Errorf("escp: barcode dimensions %dx%d (%d pixels) exceed the maximum of %d pixels", widthDots, heightDots, widthDots*heightDots, maxBarcodePixels)
	}

	var bc barcode.Barcode
	var err error

	switch strings.ToLower(barcodeType) {
	case "code39":
		// includeChecksum=true matches python-barcode==0.15.1's Code39
		// class, whose add_checksum parameter defaults to True and is never
		// overridden by matrix_command_builder.py — so the Python service
		// always embeds a mod-43 check character. Passing false here (as an
		// earlier version of this code did) produced a DIFFERENT bar
		// pattern than Python for the same data: a scanner tuned to the
		// Python service's output would misread or reject the Go output.
		bc, err = code39.Encode(data, true, true) // checksum included, full ASCII mode
	case "code128":
		bc, err = code128.Encode(data)
	case "ean13":
		if l := len(data); l != 12 && l != 13 {
			return nil, fmt.Errorf("escp: ean13 requires 12 or 13 digits, got %d", l)
		}
		bc, err = ean.Encode(data)
	case "ean8":
		if l := len(data); l != 7 && l != 8 {
			return nil, fmt.Errorf("escp: ean8 requires 7 or 8 digits, got %d", l)
		}
		bc, err = ean.Encode(data)
	case "upca", "upc":
		bc, err = encodeUPCA(data)
	default:
		return nil, fmt.Errorf("escp: unsupported barcode type %q", barcodeType)
	}
	if err != nil {
		return nil, err
	}

	stretched := image.NewRGBA(image.Rect(0, 0, widthDots, heightDots))
	xdraw.CatmullRom.Scale(stretched, stretched.Bounds(), bc, bc.Bounds(), xdraw.Over, nil)

	palette := image.NewPaletted(image.Rect(0, 0, widthDots, heightDots), monoPalette)
	stddraw.FloydSteinberg.Draw(palette, palette.Bounds(), stretched, image.Point{})
	return palette, nil
}

// encodeUPCA renders a real, scannable UPC-A barcode even though
// github.com/boombuler/barcode has no dedicated UPC-A encoder (verified:
// neither the v1.1.0 release pinned in go.sum nor the latest commit on its
// default branch has a upcean/upca package — there is no such package to
// import here).
//
// UPC-A is not a distinct symbology from EAN-13; it is EAN-13 with an
// implicit leading "0". A UPC-A number (11 data digits + 1 check digit, or
// just the 11 data digits) prepended with "0" is, digit for digit, a valid
// EAN-13 number whose check digit matches, and the two standards define
// IDENTICAL bar patterns for that shared digit sequence — this is exactly
// how EAN-13 was designed (backward compatible with UPC-A) and exactly how
// python-barcode's own UPCA class is implemented (as an EAN-13 subclass
// with a "0" prefix). So encoding "0"+data through the EAN encoder below
// produces a bar pattern indistinguishable from a real UPC-A barcode, and
// scanners read it as the original UPC-A number.
func encodeUPCA(data string) (barcode.BarcodeIntCS, error) {
	if l := len(data); l != 11 && l != 12 {
		return nil, fmt.Errorf("escp: upca requires 11 or 12 digits, got %d", l)
	}
	return ean.Encode("0" + data)
}

// imageToESCPRaster converts a 1-bit image to ESC/P double-density raster
// commands (ESC * 1, 120 dpi horizontal / 60 dpi vertical), one 8-pixel-tall
// band at a time. Each band is followed by a real line feed — critical for
// line registration on real dot-matrix printers, not just a formatting
// nicety — matching _image_to_escp_raster exactly, including the LF-per-band
// (not only at the end).
func imageToESCPRaster(img *image.Paletted) []byte {
	var result bytes.Buffer
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	for rowStart := 0; rowStart < height; rowStart += 8 {
		cols := make([]byte, width)
		for x := 0; x < width; x++ {
			var bt byte
			for bit := 0; bit < 8; bit++ {
				y := rowStart + bit
				if y < height {
					if img.Pix[img.PixOffset(x, y)] == blackIdx {
						bt |= 1 << uint(7-bit)
					}
				}
			}
			cols[x] = bt
		}

		nL := byte(len(cols) & 0xff)
		nH := byte((len(cols) >> 8) & 0xff)
		result.Write([]byte{0x1b, '*', 0x01, nL, nH})
		result.Write(cols)
		result.WriteByte(0x0a)
	}

	return result.Bytes()
}
