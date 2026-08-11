package escpos

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
)

// whiteColor/monoPalette back the padding canvas and the 1-bit conversion:
// palette index 0 is white, index 1 is black (see blackIndex above).
var (
	whiteColor  = color.White
	monoPalette = color.Palette{color.White, color.Black}
)

// DefaultMaxWidth is the fallback raster width in dots (80mm paper), used
// when Image is called with maxWidth <= 0, matching the Python service's
// PrintRequest.image_width default of 576.
const DefaultMaxWidth = 576

// maxRequestedWidthDots is a hard upper bound on the caller-supplied
// maxWidth (ultimately the HTTP PrintRequest.image_width field). It is
// generous for any real thermal printer (the widest common thermal paper,
// 112mm, is well under 1000 dots at typical resolutions) and defends
// against an absurd value in that field even though, as implemented below,
// maxWidth only ever shrinks the raster canvas — it can never grow it past
// the already-bounded decoded source image (see maxImagePixels).
const maxRequestedWidthDots = 4096

// maxImagePixels bounds the decoded image's width*height, checked via
// image.DecodeConfig BEFORE the full image.Decode (which allocates a
// pixel buffer sized to whatever the file's header declares). This is a
// decompression-bomb defense: a file can be a few KB on disk yet declare
// an enormous width/height in its header, and image.Decode would happily
// allocate for that declared size. 16,000,000 pixels (e.g. ~4000x4000) is
// far beyond any real ticket header image.
const maxImagePixels = 16_000_000

// blackIndex/whiteIndex are the palette indices used by the 1-bit
// conversion below: palette[0] = white, palette[1] = black, so a Paletted
// pixel value of 1 means "print this dot" (black).
const blackIndex = 1

// Image renders src as a GS v 0 raster bit image and appends it to the
// buffer.
//
// src is a base64-encoded image, optionally prefixed with a data URI header
// ("data:image/png;base64,..."), matching the Python service's
// CommandBuilder.image() contract for string sources.
//
// Algorithm (mirrors command_builder.py's image(), see doc comments below
// for the one deliberate divergence):
//  1. Decode src to an image.Image (any format with a registered decoder:
//     PNG, JPEG, GIF, BMP).
//  2. If width > maxWidth, resize to maxWidth keeping the aspect ratio.
//     Python uses PIL's Image.LANCZOS; this uses golang.org/x/image/draw's
//     CatmullRom resampler (the highest-quality resampler available without
//     CGO). Both are high-quality bicubic-family filters, but the two are
//     NOT byte-identical — this is a documented, expected difference from
//     the Python service, not a bug.
//  3. Pad width to a multiple of 8 with a white canvas (byte alignment
//     requirement of GS v 0).
//  4. Convert to 1-bit using Floyd-Steinberg error-diffusion dithering
//     (image/draw.FloydSteinberg), the same algorithm PIL's Image.convert("1")
//     uses by default.
//  5. Emit the GS v 0 raster header (xL xH yL yH, little-endian) followed by
//     the packed 1-bpp rows, MSB first, bit=1 for a black dot.
func (b *Builder) Image(src string, maxWidth int) error {
	if maxWidth <= 0 {
		maxWidth = DefaultMaxWidth
	}
	if maxWidth > maxRequestedWidthDots {
		maxWidth = maxRequestedWidthDots
	}

	raw, err := decodeImageSource(src)
	if err != nil {
		return err
	}

	// Decompression-bomb guard: read only the header (width/height as
	// declared by the file, no pixel data) and reject before the real
	// decode below allocates a buffer sized to that declared width*height.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("escpos: failed to decode image header: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("escpos: invalid image dimensions %dx%d", cfg.Width, cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return fmt.Errorf("escpos: image dimensions %dx%d (%d pixels) exceed the maximum of %d pixels", cfg.Width, cfg.Height, cfg.Width*cfg.Height, maxImagePixels)
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("escpos: failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Step 2: resize to fit maxWidth, keeping aspect ratio.
	if width > maxWidth {
		ratio := float64(maxWidth) / float64(width)
		newHeight := int(float64(height) * ratio)
		if newHeight < 1 {
			newHeight = 1
		}
		resized := image.NewRGBA(image.Rect(0, 0, maxWidth, newHeight))
		xdraw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, xdraw.Over, nil)
		img = resized
		width, height = maxWidth, newHeight
	}

	// Step 3: pad width to a multiple of 8 with a white canvas.
	paddedWidth := (width + 7) / 8 * 8
	if paddedWidth != width {
		canvas := image.NewRGBA(image.Rect(0, 0, paddedWidth, height))
		stddraw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: whiteColor}, image.Point{}, stddraw.Src)
		stddraw.Draw(canvas, image.Rect(0, 0, width, height), img, img.Bounds().Min, stddraw.Src)
		img = canvas
		width = paddedWidth
	}

	// Step 4: convert to 1-bit with Floyd-Steinberg dithering.
	palette := image.NewPaletted(image.Rect(0, 0, width, height), monoPalette)
	stddraw.FloydSteinberg.Draw(palette, palette.Bounds(), img, image.Point{})

	// Step 5: emit GS v 0 header + packed raster bytes.
	widthBytes := width / 8
	xL := byte(widthBytes & 0xff)
	xH := byte((widthBytes >> 8) & 0xff)
	yL := byte(height & 0xff)
	yH := byte((height >> 8) & 0xff)

	b.buf.Write([]byte{0x1d, 0x76, 0x30, 0x00, xL, xH, yL, yH})

	for y := 0; y < height; y++ {
		for xByte := 0; xByte < widthBytes; xByte++ {
			var bt byte
			base := y*width + xByte*8
			for bit := 0; bit < 8; bit++ {
				if palette.Pix[base+bit] == blackIndex {
					bt |= 0x80 >> uint(bit)
				}
			}
			b.buf.WriteByte(bt)
		}
	}

	return nil
}

// decodeImageSource resolves a base64 (optionally data-URI-prefixed) string
// into raw image bytes, mirroring command_builder.py's image() string branch.
func decodeImageSource(src string) ([]byte, error) {
	s := src
	if strings.HasPrefix(s, "data:") {
		if idx := strings.IndexByte(s, ','); idx != -1 {
			s = s[idx+1:]
		}
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Python's base64.b64decode() is lenient about padding/whitespace;
		// fall back to the padding-agnostic decoder before giving up.
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil {
			return nil, fmt.Errorf("escpos: invalid base64 image data: %w", err)
		}
	}
	return raw, nil
}
