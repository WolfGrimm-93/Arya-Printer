// Package escpos builds raw ESC/POS byte sequences for thermal receipt
// printers, replicating the behavior of arya_escpos/src/core/command_builder.py
// byte-for-byte for the subset of commands the /api/v1/print endpoint uses
// (init, align, text, newline, cut, raster image). Encoding is always IBM
// CP850, matching the Python service's hardcoded CommandBuilder(encoding="cp850").
package escpos

import (
	"bytes"
	"fmt"
)

// Alignment values accepted by Builder.Align, mirroring command_builder.py's
// align_map keys exactly (invalid values are a caller error, same as the
// Python KeyError on an unknown alignment).
const (
	AlignLeft   = "left"
	AlignCenter = "center"
	AlignRight  = "right"
)

// Builder accumulates raw ESC/POS command bytes. It is not safe for
// concurrent use; each print job should use its own Builder.
type Builder struct {
	buf bytes.Buffer
}

// New returns an empty Builder ready to accept commands.
func New() *Builder {
	return &Builder{}
}

// Init writes ESC @ (0x1B 0x40), resetting the printer to its power-on state.
func (b *Builder) Init() *Builder {
	b.buf.Write([]byte{0x1b, 0x40})
	return b
}

// Align writes ESC a n (0x1B 0x61 n) selecting left/center/right text
// alignment. Returns an error for any value outside AlignLeft/AlignCenter/
// AlignRight, mirroring the Python service's align_map[alignment] KeyError.
func (b *Builder) Align(alignment string) error {
	var n byte
	switch alignment {
	case AlignLeft:
		n = 0x00
	case AlignCenter:
		n = 0x01
	case AlignRight:
		n = 0x02
	default:
		return fmt.Errorf("escpos: invalid alignment %q", alignment)
	}
	b.buf.Write([]byte{0x1b, 0x61, n})
	return nil
}

// Text encodes s as CP850 and appends it to the buffer, unsanitized. It
// mirrors Python's text.encode("cp850") (strict mode): if s contains a rune
// with no CP850 representation, it returns an error instead of silently
// dropping or replacing it — same failure mode as the Python service, which
// lets the UnicodeEncodeError propagate uncaught.
func (b *Builder) Text(s string) error {
	encoded, err := EncodeCP850Strict(s)
	if err != nil {
		return err
	}
	b.buf.Write(encoded)
	return nil
}

// Newline writes n line feeds (0x0A). n <= 0 is a no-op.
func (b *Builder) Newline(n int) *Builder {
	for i := 0; i < n; i++ {
		b.buf.WriteByte(0x0a)
	}
	return b
}

// Cut writes GS V (0x1D 0x56): partial cut (0x01) if partial is true, full
// cut (0x00) otherwise.
func (b *Builder) Cut(partial bool) *Builder {
	if partial {
		b.buf.Write([]byte{0x1d, 0x56, 0x01})
	} else {
		b.buf.Write([]byte{0x1d, 0x56, 0x00})
	}
	return b
}

// Bytes returns the accumulated command bytes.
func (b *Builder) Bytes() []byte {
	return b.buf.Bytes()
}
