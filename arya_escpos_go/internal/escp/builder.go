// Package escp builds raw ESC/P byte sequences for dot-matrix printers,
// replicating arya_escpos/src/utils/matrix_command_builder.py byte-for-byte.
//
// Supports Epson LX-350, LX-300+II, FX-890, DFX-9000, Oki Microline, and any
// 9-pin or 24-pin printer compatible with the ESC/P protocol.
package escp

import (
	"bytes"
	"fmt"

	"aryaescpos/internal/escpos"
)

// Font values accepted by Builder.Font, mirroring matrix_command_builder.py's
// FONTS dict keys.
const (
	FontRoman       = "roman"
	FontSansSerif   = "sans_serif"
	defaultEncoding = "cp850"
)

// Builder accumulates raw ESC/P command bytes. It is not safe for
// concurrent use; each print job should use its own Builder.
type Builder struct {
	buf    bytes.Buffer
	encode func(string) []byte
}

// New returns a Builder using the given text encoding, mirroring
// MatrixCommandBuilder(encoding=...). Only "cp850" (the Python service's
// default, and the only encoding real dot-matrix printers on this fleet
// use) and "utf-8"/"utf8" are supported; any other value is an error —
// Python accepts arbitrary codec names and raises LookupError at encode
// time for an unknown one, which this mirrors by failing at construction
// instead of at Text() time.
func New(encoding string) (*Builder, error) {
	if encoding == "" {
		encoding = defaultEncoding
	}
	switch encoding {
	case "cp850":
		return &Builder{encode: escpos.EncodeCP850Replace}, nil
	case "utf-8", "utf8":
		return &Builder{encode: func(s string) []byte { return []byte(s) }}, nil
	default:
		return nil, fmt.Errorf("escp: unsupported encoding %q", encoding)
	}
}

// Init writes ESC @ (0x1B 0x40), resetting all printer settings.
func (b *Builder) Init() *Builder {
	b.buf.Write([]byte{0x1b, '@'})
	return b
}

// Font writes ESC k n selecting the typeface. Unknown font_type values
// silently default to roman (0x00), matching Python's
// FONTS.get(font_type, b"\x00") — this is a no-op fallback, not an error.
func (b *Builder) Font(fontType string) *Builder {
	f := byte(0x00)
	if fontType == FontSansSerif {
		f = 0x01
	}
	b.buf.Write([]byte{0x1b, 'k', f})
	return b
}

// CPI writes the ESC P/M/g command selecting 10/12/15 characters per inch.
// Any other value is a silent no-op — no bytes are written and no error is
// returned — matching Python's CPI_COMMANDS.get(cpi) returning None.
func (b *Builder) CPI(cpi int) *Builder {
	switch cpi {
	case 10:
		b.buf.Write([]byte{0x1b, 'P'})
	case 12:
		b.buf.Write([]byte{0x1b, 'M'})
	case 15:
		b.buf.Write([]byte{0x1b, 'g'})
	}
	return b
}

// LineSpacingSixth writes ESC 2, selecting 1/6 inch line spacing. The real
// /print/matrix flow calls this unconditionally on every job, same as the
// Python service.
func (b *Builder) LineSpacingSixth() *Builder {
	b.buf.Write([]byte{0x1b, '2'})
	return b
}

// Text encodes content using the Builder's configured encoding with
// character replacement (unencodable runes become '?'), matching Python's
// content.encode(self.encoding, errors="replace").
func (b *Builder) Text(content string) *Builder {
	b.buf.Write(b.encode(content))
	return b
}

// Newline writes count line feeds (0x0A).
func (b *Builder) Newline(count int) *Builder {
	for i := 0; i < count; i++ {
		b.buf.WriteByte(0x0a)
	}
	return b
}

// FormFeed writes FF (0x0C), advancing continuous paper to the next page.
func (b *Builder) FormFeed() *Builder {
	b.buf.WriteByte(0x0c)
	return b
}

// Bytes returns the accumulated command bytes.
func (b *Builder) Bytes() []byte {
	return b.buf.Bytes()
}
