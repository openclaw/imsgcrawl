package messages

import (
	"strings"
	"testing"
)

func TestNormalizePhoneMatchesClawdexShape(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"+1 (415) 734-7847", "14157347847"},
		{"0043 664 104 2436", "436641042436"},
		{"opaque", ""},
	} {
		if got := NormalizePhone(tc.in); got != tc.want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLooksPhoneLikeAllowsShortCodesButRejectsOpaqueIDs(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"42777", true},
		{"+1 (415) 734-7847", true},
		{"service123", false},
		{"person@example.test", false},
		{"opaque", false},
	} {
		if got := LooksPhoneLike(tc.in); got != tc.want {
			t.Fatalf("LooksPhoneLike(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestDecodeAttributedBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
	}{
		{name: "short", text: "Hello world"},
		{name: "long", text: "This is a longer message that exercises the multi-byte length path in the streamtyped attributedBody decoder."},
		{name: "extended length", text: strings.Repeat("x", 300)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeAttributedBody(makeStreamtypedAttributedBody(tc.text))
			if got != tc.text {
				t.Fatalf("got %q, want %q", got, tc.text)
			}
		})
	}
}

func TestDecodeAttributedBodyRejectsOverflowLength(t *testing.T) {
	body := []byte("\x04\x0bstreamtyped\x84\x01+")
	body = append(body, 0x83, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f)
	if text, ok := decodeAttributedBodyValue(body); ok || text != "" {
		t.Fatalf("overflow body decoded as %q, available=%v", text, ok)
	}
}

func makeStreamtypedAttributedBody(text string) []byte {
	var out []byte
	out = append(out, "\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84"...)
	out = append(out, "\x12NSAttributedString"...)
	out = append(out, "\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString\x01\x94"...)
	out = append(out, "\x84\x01+"...)
	out = appendTestStreamtypedLength(out, len(text))
	out = append(out, text...)
	out = append(out, 0x86)
	return out
}

func appendTestStreamtypedLength(out []byte, length int) []byte {
	switch {
	case length <= 0x7f:
		return append(out, byte(length))
	case length <= 0xffff:
		return append(out, 0x81, byte(length), byte(length>>8))
	case uint64(length) <= 0xffffffff:
		return append(out, 0x82, byte(length), byte(length>>8), byte(length>>16), byte(length>>24))
	default:
		return append(out, 0x83, byte(length), byte(length>>8), byte(length>>16), byte(length>>24), byte(length>>32), byte(length>>40), byte(length>>48), byte(length>>56))
	}
}
