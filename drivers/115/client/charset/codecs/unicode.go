// Unicode codec behavior is pinned by offline CPython 3.12.13 fixtures.
// Native UTF16/32 byte order follows the running platform, matching Python's
// stateless codecs; explicit endian variants preserve any BOM as U+FEFF.
package codecs

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
)

func decodeUnicode(raw []byte, n string, ignore bool) (string, error) {
	if n == "utf-8" || n == "utf-8-sig" {
		if n == "utf-8-sig" && len(raw) >= 3 && string(raw[:3]) == "\xef\xbb\xbf" {
			raw = raw[3:]
		}
		var b strings.Builder
		for len(raw) > 0 {
			r, k := utf8.DecodeRune(raw)
			if r == utf8.RuneError && k == 1 {
				if !ignore {
					return "", fmt.Errorf("invalid UTF8")
				}
				raw = raw[1:]
				continue
			}
			b.WriteRune(r)
			raw = raw[k:]
		}
		return b.String(), nil
	}
	if n == "utf-7" {
		return decodeUTF7(raw, ignore)
	}
	width := 2
	if strings.HasPrefix(n, "utf-32") {
		width = 4
	}
	var order binary.ByteOrder = binary.NativeEndian
	if strings.HasSuffix(n, "le") {
		order = binary.LittleEndian
	}
	if strings.HasSuffix(n, "be") {
		order = binary.BigEndian
	}
	if n == "utf-16" || n == "utf-32" {
		if len(raw) >= width {
			if width == 2 && raw[0] == 254 && raw[1] == 255 || width == 4 && string(raw[:4]) == "\x00\x00\xfe\xff" {
				order = binary.BigEndian
				raw = raw[width:]
			} else if width == 2 && raw[0] == 255 && raw[1] == 254 || width == 4 && string(raw[:4]) == "\xff\xfe\x00\x00" {
				order = binary.LittleEndian
				raw = raw[width:]
			}
		}
	}
	var b strings.Builder
	for len(raw) > 0 {
		if len(raw) < width {
			if ignore {
				break
			}
			return "", fmt.Errorf("truncated unicode")
		}
		var r rune
		if width == 4 {
			r = rune(order.Uint32(raw))
			raw = raw[4:]
		} else {
			r = rune(order.Uint16(raw))
			raw = raw[2:]
			if r >= 0xd800 && r <= 0xdbff {
				if len(raw) >= 2 {
					lo := rune(order.Uint16(raw))
					if lo >= 0xdc00 && lo <= 0xdfff {
						r = 0x10000 + (r-0xd800)*1024 + lo - 0xdc00
						raw = raw[2:]
					}
				}
			}
		}
		if !utf8.ValidRune(r) {
			if ignore {
				continue
			}
			return "", fmt.Errorf("invalid unicode scalar")
		}
		b.WriteRune(r)
	}
	return b.String(), nil
}

// textRunes preserves Python surrogate code points represented as WTF-8 in Go strings.
func textRunes(text string) []rune {
	out := []rune{}
	for len(text) > 0 {
		if len(text) >= 3 && text[0] == 0xed && text[1] >= 0xa0 && text[1] <= 0xbf && text[2] >= 0x80 && text[2] <= 0xbf {
			out = append(out, rune(text[0]&15)<<12|rune(text[1]&63)<<6|rune(text[2]&63))
			text = text[3:]
			continue
		}
		r, n := utf8.DecodeRuneInString(text)
		out = append(out, r)
		text = text[n:]
	}
	return out
}
func encodeUnicode(text, n string, ignore bool) ([]byte, error) {
	if n == "utf-7" {
		return encodeUTF7(text, ignore)
	}
	var valid strings.Builder
	for _, r := range textRunes(text) {
		if r >= 0xd800 && r <= 0xdfff {
			if !ignore {
				return nil, fmt.Errorf("surrogates not allowed")
			}
			continue
		}
		valid.WriteRune(r)
	}
	text = valid.String()
	if n == "utf-8" {
		return []byte(text), nil
	}
	if n == "utf-8-sig" {
		return append([]byte{239, 187, 191}, []byte(text)...), nil
	}
	width := 2
	if strings.HasPrefix(n, "utf-32") {
		width = 4
	}
	var order binary.ByteOrder = binary.NativeEndian
	if strings.HasSuffix(n, "le") {
		order = binary.LittleEndian
	}
	if strings.HasSuffix(n, "be") {
		order = binary.BigEndian
	}
	out := []byte{}
	put := func(r rune) {
		b := make([]byte, width)
		if width == 2 {
			order.PutUint16(b, uint16(r))
		} else {
			order.PutUint32(b, uint32(r))
		}
		out = append(out, b...)
	}
	if n == "utf-16" || n == "utf-32" {
		put(0xfeff)
	}
	for _, r := range text {
		if width == 2 && r > 0xffff {
			r -= 0x10000
			put(0xd800 + r/1024)
			put(0xdc00 + r%1024)
		} else {
			put(r)
		}
	}
	return out, nil
}
