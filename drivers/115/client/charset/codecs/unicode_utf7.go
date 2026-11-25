// UTF7 preserves CPython code-unit semantics, including unpaired surrogates.
// Such scalars use WTF-8 bytes in Go strings, avoiding replacement characters;
// Encode accepts that representation for a lossless UTF7 round trip.
package codecs

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func decodeUTF7(raw []byte, ignore bool) (string, error) {
	var b strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] != '+' {
			if raw[i] < 128 {
				b.WriteByte(raw[i])
			} else if !ignore {
				return "", fmt.Errorf("invalid UTF7 byte")
			}
			i++
			continue
		}
		i++
		if i == len(raw) {
			break
		}
		if raw[i] == '-' {
			b.WriteByte('+')
			i++
			continue
		}
		var bits uint32
		nbits := 0
		units := []rune{}
		start := i
		for i < len(raw) {
			v := strings.IndexByte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", raw[i])
			if v < 0 {
				break
			}
			bits = bits<<6 | uint32(v)
			nbits += 6
			i++
			if nbits >= 16 {
				nbits -= 16
				units = append(units, rune(bits>>uint(nbits)&65535))
			}
		}
		valid := i > start && nbits < 6 && (bits&((1<<uint(nbits))-1)) == 0
		if len(units) > 0 && units[len(units)-1] >= 0xd800 && units[len(units)-1] <= 0xdbff && (!valid || i == len(raw)) {
			units = units[:len(units)-1]
			if i == len(raw) {
				valid = false
			}
		}
		// Python commits complete code units even when the shifted tail is malformed.
		for j := 0; j < len(units); j++ {
			r := units[j]
			if r >= 0xd800 && r <= 0xdbff && j+1 < len(units) && units[j+1] >= 0xdc00 && units[j+1] <= 0xdfff {
				r = 0x10000 + (r-0xd800)*1024 + units[j+1] - 0xdc00
				j++
			}
			if r >= 0xd800 && r <= 0xdfff {
				b.WriteByte(byte(0xe0 | r>>12))
				b.WriteByte(byte(0x80 | (r>>6)&63))
				b.WriteByte(byte(0x80 | r&63))
			} else {
				b.WriteRune(r)
			}
		}
		if !valid {
			if !ignore {
				return "", fmt.Errorf("invalid UTF7 shift")
			}
			if i < len(raw) {
				i++
			}
		} else if i < len(raw) && raw[i] == '-' {
			i++
		}
	}
	return b.String(), nil
}
func encodeUTF7(text string, ignore bool) ([]byte, error) {
	var b strings.Builder
	var shifted []rune
	flush := func(next rune) {
		if len(shifted) > 0 {
			data := []byte{}
			for _, r := range shifted {
				if r > 65535 {
					r -= 65536
					h := 0xd800 + r/1024
					l := 0xdc00 + r%1024
					data = append(data, byte(h>>8), byte(h), byte(l>>8), byte(l))
				} else {
					data = append(data, byte(r>>8), byte(r))
				}
			}
			b.WriteByte('+')
			b.WriteString(base64.RawStdEncoding.EncodeToString(data))
			if next == 0 || next == '-' || next >= 'A' && next <= 'Z' || next >= 'a' && next <= 'z' || next >= '0' && next <= '9' || next == '+' || next == '/' {
				b.WriteByte('-')
			}
			shifted = nil
		}
	}
	for _, r := range textRunes(text) {
		if r >= 32 && r <= 125 && r != '\\' || r == '\n' || r == '\r' || r == '\t' {
			if r == '+' && len(shifted) > 0 {
				shifted = append(shifted, r)
				continue
			}
			flush(r)
			if r == '+' && len(shifted) == 0 {
				b.WriteString("+-")
			} else {
				b.WriteRune(r)
			}
		} else {
			shifted = append(shifted, r)
		}
	}
	flush(0)
	return []byte(b.String()), nil
}
