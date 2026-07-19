package messages

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"
)

func decodeAttributedBody(body []byte) string {
	text, _ := decodeAttributedBodyValue(body)
	return text
}

func decodeAttributedBodyValue(body []byte) (string, bool) {
	if !bytes.HasPrefix(body, []byte("\x04\x0bstreamtyped")) {
		return "", false
	}
	marker := []byte("\x84\x01+")
	idx := bytes.Index(body, marker)
	if idx < 0 {
		return "", false
	}
	pos := idx + len(marker)
	if pos >= len(body) {
		return "", false
	}
	textLen, pos, ok := decodeStreamtypedLength(body, pos)
	if !ok || textLen < 0 || pos > len(body) {
		return "", false
	}
	for pos < len(body) && (body[pos] == 0x00 || body[pos] == 0x92 || (body[pos] >= 0x80 && body[pos] <= 0xbf)) {
		pos++
	}
	if pos > len(body) || textLen > len(body)-pos {
		return "", false
	}
	end := pos + textLen
	return cleanDecodedText(body[pos:end]), true
}

func decodeStreamtypedLength(body []byte, pos int) (int, int, bool) {
	if pos >= len(body) {
		return 0, pos, false
	}
	first := body[pos]
	if first <= 0x7f {
		return int(first), pos + 1, true
	}
	var width int
	switch first {
	case 0x81:
		width = 2
	case 0x82:
		width = 4
	case 0x83:
		width = 8
	default:
		return 0, pos, false
	}
	pos++
	if pos+width > len(body) {
		return 0, pos, false
	}
	var n uint64
	for i := 0; i < width; i++ {
		n |= uint64(body[pos+i]) << (8 * i)
	}
	if n > uint64(^uint(0)>>1) {
		return 0, pos, false
	}
	return int(n), pos + width, true
}

func cleanDecodedText(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xff && raw[1] == 0xfe {
		return decodeUTF16(raw[2:], true)
	}
	if len(raw) >= 2 && raw[0] == 0xfe && raw[1] == 0xff {
		return decodeUTF16(raw[2:], false)
	}
	text := string(raw)
	for len(text) > 0 && !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func decodeUTF16(raw []byte, littleEndian bool) string {
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		if littleEndian {
			units = append(units, uint16(raw[i])|uint16(raw[i+1])<<8)
		} else {
			units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
		}
	}
	return string(utf16.Decode(units))
}
