package escp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boombuler/barcode/code39"

	"aryaescpos/internal/contract"
)

func newBuilder(t *testing.T) *Builder {
	t.Helper()
	b, err := New("cp850")
	if err != nil {
		t.Fatalf("New(\"cp850\") unexpected error: %v", err)
	}
	return b
}

func TestInit(t *testing.T) {
	b := newBuilder(t)
	b.Init()
	want := []byte{0x1b, '@'}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Init() = % x, want % x", got, want)
	}
}

func TestFont(t *testing.T) {
	cases := []struct {
		font string
		want byte
	}{
		{FontRoman, 0x00},
		{FontSansSerif, 0x01},
		{"unknown", 0x00}, // silent fallback to roman, not an error
		{"", 0x00},
	}
	for _, c := range cases {
		b := newBuilder(t)
		b.Font(c.font)
		want := []byte{0x1b, 'k', c.want}
		if got := b.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("Font(%q) = % x, want % x", c.font, got, want)
		}
	}
}

func TestCPI(t *testing.T) {
	cases := []struct {
		cpi  int
		want []byte
	}{
		{10, []byte{0x1b, 'P'}},
		{12, []byte{0x1b, 'M'}},
		{15, []byte{0x1b, 'g'}},
		{9999, nil}, // unrecognized value is a silent no-op, not an error/default
	}
	for _, c := range cases {
		b := newBuilder(t)
		b.CPI(c.cpi)
		got := b.Bytes()
		if !bytes.Equal(got, c.want) {
			t.Errorf("CPI(%d) = % x, want % x", c.cpi, got, c.want)
		}
	}
}

func TestLineSpacingSixth(t *testing.T) {
	b := newBuilder(t)
	b.LineSpacingSixth()
	want := []byte{0x1b, '2'}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("LineSpacingSixth() = % x, want % x", got, want)
	}
}

func TestTextReplacesUnencodableRunes(t *testing.T) {
	b := newBuilder(t)
	b.Text("A€B") // € has no CP850 mapping -> replaced with '?', never an error
	want := []byte{'A', '?', 'B'}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Text(\"A€B\") = % x, want % x", got, want)
	}
}

func TestNewlineAndFormFeed(t *testing.T) {
	b := newBuilder(t)
	b.Newline(2)
	b.FormFeed()
	want := []byte{0x0a, 0x0a, 0x0c}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Newline(2)+FormFeed() = % x, want % x", got, want)
	}
}

func TestFullMatrixSequence(t *testing.T) {
	b := newBuilder(t)
	b.Init().Font(FontRoman).CPI(10).LineSpacingSixth().Text("Hi").Newline(2).FormFeed()
	want := []byte{0x1b, '@', 0x1b, 'k', 0x00, 0x1b, 'P', 0x1b, '2', 'H', 'i', 0x0a, 0x0a, 0x0c}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("full sequence = % x, want % x", got, want)
	}
}

func TestNewUnsupportedEncoding(t *testing.T) {
	if _, err := New("shift-jis"); err == nil {
		t.Fatal("New(\"shift-jis\") expected error, got nil")
	}
}

func TestBarcodeCode39Renders(t *testing.T) {
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "12345", Type: "code39", WidthDots: 160, HeightDots: 32})
	got := b.Bytes()
	if len(got) == 0 {
		t.Fatal("Barcode() produced no output")
	}
	// A real raster render starts with ESC * 1, never with the fallback '['.
	if got[0] != 0x1b || got[1] != '*' {
		t.Fatalf("Barcode() did not render as raster graphics, got first bytes % x", got[:4])
	}
	// heightDots=32 -> 4 bands of 8 rows, each ESC * 1 nL nH <160 bytes> + LF,
	// plus one trailing LF appended by Barcode() itself after the raster.
	bandLen := 5 + 160 + 1 // header(5) + cols(widthDots) + LF
	wantLen := bandLen*4 + 1
	if len(got) != wantLen {
		t.Fatalf("Barcode() length = %d, want %d", len(got), wantLen)
	}
}

func TestBarcodeUnsupportedTypeFallsBackToLabel(t *testing.T) {
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "12345", Type: "code93"}) // not one of the supported symbologies
	got := b.Bytes()
	want := []byte("[CODE93: 12345]\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("Barcode() fallback = %q, want %q", got, want)
	}
}

func TestCode39IncludesChecksumByDefault(t *testing.T) {
	// python-barcode==0.15.1's Code39 class defaults add_checksum=True, and
	// matrix_command_builder.py never overrides it, so the Go encoder must
	// always request a checksum too -- otherwise the two services produce
	// different bar patterns for identical input, and a scanner tuned to
	// one would misread the other. renderBarcode1Bit always stretches to a
	// fixed widthDots/heightDots, which would hide a checksum regression in
	// the final raster bytes -- so this compares the *unstretched* native
	// barcode width directly against the library, independent of the
	// stretch step.
	withChecksum, err := code39.Encode("12345", true, true)
	if err != nil {
		t.Fatalf("code39.Encode(checksum=true) unexpected error: %v", err)
	}
	withoutChecksum, err := code39.Encode("12345", false, true)
	if err != nil {
		t.Fatalf("code39.Encode(checksum=false) unexpected error: %v", err)
	}
	if withChecksum.Bounds().Dx() <= withoutChecksum.Bounds().Dx() {
		t.Fatalf("expected a checksum character to widen the native barcode: %d (with) vs %d (without)",
			withChecksum.Bounds().Dx(), withoutChecksum.Bounds().Dx())
	}
}

func TestBarcodeUPCARendersElevenDigits(t *testing.T) {
	// 11 data digits (no check digit) -- ean.Encode computes and appends it.
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "03600029145", Type: "upca", WidthDots: 160, HeightDots: 32})
	got := b.Bytes()
	if len(got) == 0 {
		t.Fatal("Barcode() upca (11 digits) produced no output")
	}
	if got[0] != 0x1b || got[1] != '*' {
		t.Fatalf("Barcode() upca (11 digits) did not render as raster graphics, got % x", got[:4])
	}
}

func TestBarcodeUPCARendersTwelveDigitsWithCheckDigit(t *testing.T) {
	// 12 digits including the correct check digit (real-world Wrigley's Gum
	// UPC-A: 0-36000-29145-2).
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "036000291452", Type: "upc", WidthDots: 160, HeightDots: 32})
	got := b.Bytes()
	if got[0] != 0x1b || got[1] != '*' {
		t.Fatalf("Barcode() upc (12 digits) did not render as raster graphics, got % x", got[:4])
	}
}

func TestBarcodeUPCAInvalidLengthFallsBackToLabel(t *testing.T) {
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "123", Type: "upca"})
	got := string(b.Bytes())
	if !strings.HasPrefix(got, "[UPCA: 123]") {
		t.Fatalf("Barcode() fallback = %q, want prefix [UPCA: 123]", got)
	}
}

func TestBarcodeDimensionsAboveMaxFallBackToLabel(t *testing.T) {
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "12345", Type: "code39", WidthDots: 999999, HeightDots: 999999})
	got := string(b.Bytes())
	if !strings.HasPrefix(got, "[CODE39: 12345]") {
		t.Fatalf("Barcode() with oversized dimensions = %q, want fallback label prefix [CODE39: 12345]", got)
	}
}

func TestBarcodeDimensionsJustWithinMaxStillRenders(t *testing.T) {
	b := newBuilder(t)
	b.Barcode(contract.BarcodeItem{Data: "12345", Type: "code39", WidthDots: 4000, HeightDots: 8})
	got := b.Bytes()
	if got[0] != 0x1b || got[1] != '*' {
		t.Fatalf("Barcode() with in-budget large dimensions did not render as raster, got % x", got[:4])
	}
}

func TestBarcodeInvalidDataFallsBackToLabel(t *testing.T) {
	b := newBuilder(t)
	// EAN13 requires 12-13 numeric digits; this is neither.
	b.Barcode(contract.BarcodeItem{Data: "not-a-number", Type: "ean13"})
	got := string(b.Bytes())
	if !strings.HasPrefix(got, "[EAN13: not-a-number]") {
		t.Fatalf("Barcode() fallback = %q, want prefix [EAN13: not-a-number]", got)
	}
}

func TestBarcodeDefaultsWhenFieldsOmitted(t *testing.T) {
	b := newBuilder(t)
	// No Type/WidthDots/HeightDots set: must default to code39/300/48 and
	// still render successfully (never abort/error out).
	b.Barcode(contract.BarcodeItem{Data: "ABC123"})
	got := b.Bytes()
	if len(got) == 0 {
		t.Fatal("Barcode() with defaults produced no output")
	}
	if got[0] != 0x1b || got[1] != '*' {
		t.Fatalf("Barcode() with defaults did not render as raster, got % x", got[:4])
	}
}
