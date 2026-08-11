package escpos

import "fmt"

// cp850HighTable maps byte values 0x80-0xFF (index 0-127) to their Unicode
// rune in IBM Code Page 850 (Multilingual Latin 1). Bytes 0x00-0x7F are
// identical to ASCII (cp850 is an ASCII superset), so only the high half is
// tabulated here. Values transcribed from the standard CP850 mapping (cross
// checked against golang.org/x/text/encoding/charmap's CodePage850 table,
// which is not imported here — see cp850_test.go for the source note).
var cp850HighTable = [128]rune{
	'Ç', 'ü', 'é', 'â', 'ä', 'à', 'å', 'ç', // 0x80-0x87
	'ê', 'ë', 'è', 'ï', 'î', 'ì', 'Ä', 'Å', // 0x88-0x8F
	'É', 'æ', 'Æ', 'ô', 'ö', 'ò', 'û', 'ù', // 0x90-0x97
	'ÿ', 'Ö', 'Ü', 'ø', '£', 'Ø', '×', 'ƒ', // 0x98-0x9F
	'á', 'í', 'ó', 'ú', 'ñ', 'Ñ', 'ª', 'º', // 0xA0-0xA7
	'¿', '®', '¬', '½', '¼', '¡', '«', '»', // 0xA8-0xAF
	'░', '▒', '▓', '│', '┤', 'Á', 'Â', 'À', // 0xB0-0xB7
	'©', '╣', '║', '╗', '╝', '¢', '¥', '┐', // 0xB8-0xBF
	'└', '┴', '┬', '├', '─', '┼', 'ã', 'Ã', // 0xC0-0xC7
	'╚', '╔', '╩', '╦', '╠', '═', '╬', '¤', // 0xC8-0xCF
	'ð', 'Ð', 'Ê', 'Ë', 'È', 'ı', 'Í', 'Î', // 0xD0-0xD7
	'Ï', '┘', '┌', '█', '▄', '¦', 'Ì', '▀', // 0xD8-0xDF
	'Ó', 'ß', 'Ô', 'Ò', 'õ', 'Õ', 'µ', 'þ', // 0xE0-0xE7
	'Þ', 'Ú', 'Û', 'Ù', 'ý', 'Ý', '¯', '´', // 0xE8-0xEF
	'­', '±', '‗', '¾', '¶', '§', '÷', '¸', // 0xF0-0xF7
	'°', '¨', '·', '¹', '³', '²', '■', ' ', // 0xF8-0xFF
}

var cp850EncodeTable = buildCP850EncodeTable()

func buildCP850EncodeTable() map[rune]byte {
	m := make(map[rune]byte, 128)
	for i, r := range cp850HighTable {
		m[r] = byte(0x80 + i)
	}
	return m
}

// EncodeCP850Strict encodes s as IBM CP850 bytes, mirroring Python's
// str.encode("cp850") with the default strict error handler: it returns an
// error on the first rune with no CP850 representation instead of silently
// dropping or substituting it.
func EncodeCP850Strict(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		b, ok := cp850EncodeTable[r]
		if !ok {
			return nil, fmt.Errorf("escpos: rune %q (U+%04X) has no CP850 representation", r, r)
		}
		out = append(out, b)
	}
	return out, nil
}

// EncodeCP850Replace encodes s as IBM CP850 bytes, replacing any rune with
// no CP850 representation with '?' (0x3F) — the Go equivalent of Python's
// str.encode("cp850", errors="replace").
func EncodeCP850Replace(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		b, ok := cp850EncodeTable[r]
		if !ok {
			out = append(out, '?')
			continue
		}
		out = append(out, b)
	}
	return out
}
