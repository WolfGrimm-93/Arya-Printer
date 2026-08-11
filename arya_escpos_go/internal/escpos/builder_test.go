package escpos

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	stdimage "image"
	"image/color"
	stdpng "image/png"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	b := New()
	b.Init()
	want := []byte{0x1b, 0x40}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Init() = % x, want % x", got, want)
	}
}

func TestAlign(t *testing.T) {
	cases := []struct {
		align string
		want  []byte
	}{
		{AlignLeft, []byte{0x1b, 0x61, 0x00}},
		{AlignCenter, []byte{0x1b, 0x61, 0x01}},
		{AlignRight, []byte{0x1b, 0x61, 0x02}},
	}
	for _, c := range cases {
		b := New()
		if err := b.Align(c.align); err != nil {
			t.Fatalf("Align(%q) unexpected error: %v", c.align, err)
		}
		if got := b.Bytes(); !bytes.Equal(got, c.want) {
			t.Errorf("Align(%q) = % x, want % x", c.align, got, c.want)
		}
	}
}

func TestAlignInvalid(t *testing.T) {
	b := New()
	if err := b.Align("up"); err == nil {
		t.Fatal("Align(\"up\") expected error, got nil")
	}
}

func TestTextASCII(t *testing.T) {
	b := New()
	if err := b.Text("Hola\n"); err != nil {
		t.Fatalf("Text() unexpected error: %v", err)
	}
	want := []byte("Hola\n")
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Text() = % x, want % x", got, want)
	}
}

func TestTextCP850HighBytes(t *testing.T) {
	b := New()
	// "ñ" is 0xA4 in CP850, "€" has no CP850 representation.
	if err := b.Text("ñ"); err != nil {
		t.Fatalf("Text(\"ñ\") unexpected error: %v", err)
	}
	want := []byte{0xa4}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Text(\"ñ\") = % x, want % x", got, want)
	}
}

func TestTextUnencodableReturnsError(t *testing.T) {
	b := New()
	if err := b.Text("€"); err == nil {
		t.Fatal("Text(\"€\") expected error (no CP850 mapping), got nil")
	}
}

func TestNewline(t *testing.T) {
	b := New()
	b.Newline(3)
	want := []byte{0x0a, 0x0a, 0x0a}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Newline(3) = % x, want % x", got, want)
	}
}

func TestCut(t *testing.T) {
	b := New()
	b.Cut(false)
	want := []byte{0x1d, 0x56, 0x00}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Cut(false) = % x, want % x", got, want)
	}

	b2 := New()
	b2.Cut(true)
	want2 := []byte{0x1d, 0x56, 0x01}
	if got := b2.Bytes(); !bytes.Equal(got, want2) {
		t.Fatalf("Cut(true) = % x, want % x", got, want2)
	}
}

func TestFullTicketSequence(t *testing.T) {
	b := New()
	b.Init()
	if err := b.Text("Hello"); err != nil {
		t.Fatal(err)
	}
	if err := b.Text("\n\n\n"); err != nil {
		t.Fatal(err)
	}
	b.Cut(false)

	want := append([]byte{0x1b, 0x40}, []byte("Hello\n\n\n")...)
	want = append(want, 0x1d, 0x56, 0x00)
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("full sequence = % x, want % x", got, want)
	}
}

// makeTestPNGBase64 builds a small solid-color PNG and returns it base64
// encoded, for exercising Image() without needing a real header image file.
func makeTestPNGBase64(t *testing.T, width, height int, c color.Color) string {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := stdpng.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestImageHeaderAndSize(t *testing.T) {
	// 16x4 image: already a multiple of 8, no padding -> 2 bytes/row * 4 rows.
	src := makeTestPNGBase64(t, 16, 4, color.Black)
	b := New()
	if err := b.Image(src, 576); err != nil {
		t.Fatalf("Image() unexpected error: %v", err)
	}
	got := b.Bytes()
	if len(got) < 8 {
		t.Fatalf("Image() output too short: % x", got)
	}
	header := got[:8]
	wantHeader := []byte{0x1d, 0x76, 0x30, 0x00, 0x02, 0x00, 0x04, 0x00} // widthBytes=2, height=4
	if !bytes.Equal(header, wantHeader) {
		t.Fatalf("Image() header = % x, want % x", header, wantHeader)
	}
	wantTotal := 8 + 2*4 // header + widthBytes*height
	if len(got) != wantTotal {
		t.Fatalf("Image() total length = %d, want %d", len(got), wantTotal)
	}
	// Every pixel is black and width is already byte-aligned (no white
	// padding columns), so every data byte should be 0xff.
	for _, bt := range got[8:] {
		if bt != 0xff {
			t.Fatalf("Image() data byte = %#x, want 0xff (all-black source)", bt)
		}
	}
}

func TestImagePaddingProducesWhiteColumns(t *testing.T) {
	// 10x1 all-black image pads to 16 dots wide: columns 10-15 must be
	// white padding, so byte 0 = 0xff (cols 0-7 black) and byte 1 = 0xc0
	// (cols 8-9 black, cols 10-15 white).
	src := makeTestPNGBase64(t, 10, 1, color.Black)
	b := New()
	if err := b.Image(src, 576); err != nil {
		t.Fatalf("Image() unexpected error: %v", err)
	}
	got := b.Bytes()
	data := got[8:]
	if len(data) != 2 {
		t.Fatalf("Image() data length = %d, want 2", len(data))
	}
	if data[0] != 0xff {
		t.Fatalf("Image() data[0] = %#x, want 0xff", data[0])
	}
	if data[1] != 0xc0 {
		t.Fatalf("Image() data[1] = %#x, want 0xc0 (padding columns are white)", data[1])
	}
}

func TestImageDataURIPrefix(t *testing.T) {
	raw := makeTestPNGBase64(t, 8, 8, color.White)
	src := "data:image/png;base64," + raw
	b := New()
	if err := b.Image(src, 576); err != nil {
		t.Fatalf("Image() with data URI prefix unexpected error: %v", err)
	}
	if len(b.Bytes()) == 0 {
		t.Fatal("Image() with data URI prefix produced no output")
	}
}

func TestImageResizesWhenWiderThanMaxWidth(t *testing.T) {
	src := makeTestPNGBase64(t, 100, 50, color.Black)
	b := New()
	if err := b.Image(src, 40); err != nil {
		t.Fatalf("Image() unexpected error: %v", err)
	}
	got := b.Bytes()
	widthBytes := int(got[4]) | int(got[5])<<8
	// 40 dots -> padded to multiple of 8 -> 40 already is (40/8=5).
	if widthBytes != 5 {
		t.Fatalf("Image() resized widthBytes = %d, want 5 (40 dots / 8)", widthBytes)
	}
}

func TestImageRequestedWidthIsClamped(t *testing.T) {
	// maxWidth=999999 must be clamped to maxRequestedWidthDots (4096) before
	// it's ever used to size a resize buffer. A 5000-wide source is above
	// that clamp, so it must resize down to 4096, not up to (or towards)
	// 999999.
	src := makeTestPNGBase64(t, 5000, 1, color.Black)
	b := New()
	if err := b.Image(src, 999999); err != nil {
		t.Fatalf("Image() unexpected error: %v", err)
	}
	got := b.Bytes()
	widthBytes := int(got[4]) | int(got[5])<<8
	wantWidthBytes := maxRequestedWidthDots / 8
	if widthBytes != wantWidthBytes {
		t.Fatalf("Image() widthBytes = %d, want %d (clamped to %d dots)", widthBytes, wantWidthBytes, maxRequestedWidthDots)
	}
}

// fakePNGHeaderOnly builds a syntactically valid PNG signature + IHDR chunk
// declaring width x height, with no IDAT/IEND chunks at all. image/png's
// DecodeConfig only needs to parse IHDR (for a non-palette color type) to
// report Width/Height, so this is enough to exercise the pixel-budget guard
// without needing actual pixel data for a huge image — simulating a
// decompression-bomb-style file that is tiny on disk but declares an
// enormous width/height in its header.
func fakePNGHeaderOnly(t *testing.T, width, height uint32) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], width)
	binary.BigEndian.PutUint32(ihdrData[4:8], height)
	ihdrData[8] = 8  // bit depth
	ihdrData[9] = 2  // color type: truecolor (no palette needed)
	ihdrData[10] = 0 // compression method
	ihdrData[11] = 0 // filter method
	ihdrData[12] = 0 // interlace method

	chunkType := []byte("IHDR")
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(ihdrData)))
	buf.Write(lengthBuf[:])
	buf.Write(chunkType)
	buf.Write(ihdrData)

	crc := crc32.NewIEEE()
	crc.Write(chunkType)
	crc.Write(ihdrData)
	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], crc.Sum32())
	buf.Write(crcBuf[:])

	return buf.Bytes()
}

func TestImageRejectsDecompressionBombDimensions(t *testing.T) {
	// Header declares 20000x20000 (400M pixels), well past maxImagePixels,
	// but the "file" itself is under 50 bytes -- if the guard didn't fire
	// before the real image.Decode, this would either hang or attempt to
	// allocate a huge buffer (it would actually fail decode for lack of
	// IDAT/IEND, but that's not the failure mode we're testing for: we want
	// proof the pixel-budget check fires FIRST, based on the error message).
	fakePNG := fakePNGHeaderOnly(t, 20000, 20000)
	src := base64.StdEncoding.EncodeToString(fakePNG)

	b := New()
	err := b.Image(src, 576)
	if err == nil {
		t.Fatal("Image() expected error for a 20000x20000 declared image, got nil")
	}
	if !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("Image() error = %q, want it to mention the pixel budget being exceeded", err.Error())
	}
}

func TestImageAcceptsDimensionsWithinBudget(t *testing.T) {
	// Sanity check that the guard's threshold doesn't reject legitimate
	// images: 3000x3000 = 9,000,000 pixels, under maxImagePixels (16M).
	fakePNG := fakePNGHeaderOnly(t, 3000, 3000)
	src := base64.StdEncoding.EncodeToString(fakePNG)

	b := New()
	err := b.Image(src, 576)
	// This still fails, but it MUST fail at the real image.Decode step
	// (missing IDAT/IEND), not at the pixel-budget guard -- proving the
	// guard let it through.
	if err == nil {
		t.Fatal("Image() unexpectedly succeeded on a header-only PNG with no pixel data")
	}
	if strings.Contains(err.Error(), "exceed") {
		t.Fatalf("Image() error = %q, pixel-budget guard incorrectly rejected an in-budget image", err.Error())
	}
}
