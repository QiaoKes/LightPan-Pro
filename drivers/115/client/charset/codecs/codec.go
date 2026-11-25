// Package codecs implements the pinned Python text codec contract.
package codecs

import (
	"fmt"
	"strings"
)

func Canonical(name string) (string, error) {
	var norm strings.Builder
	pending := false
	for _, c := range name {
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' {
			if pending && norm.Len() > 0 {
				norm.WriteByte('_')
			}
			pending = false
			norm.WriteRune(c)
		} else {
			pending = true
		}
	}
	n := norm.String()
	for _, u := range []string{"utf-7", "utf-8", "utf-8-sig", "utf-16", "utf-16-le", "utf-16-be", "utf-32", "utf-32-le", "utf-32-be"} {
		if n == strings.ReplaceAll(u, "-", "_") {
			return u, nil
		}
	}
	if c, ok := aliases[n]; ok {
		return c, nil
	}
	for _, c := range aliases {
		if strings.ReplaceAll(c, "-", "_") == n {
			return c, nil
		}
	}
	return "", fmt.Errorf("unknown encoding %q", name)
}
func Supported() []string { return append([]string(nil), supported...) }
func IsMultiByte(name string) bool {
	n, e := Canonical(name)
	return e == nil && (strings.HasPrefix(n, "utf") || isMultiByte(n))
}
func Decode(raw []byte, name string, ignore bool) (string, error) {
	n, e := Canonical(name)
	if e != nil {
		return "", e
	}
	if t, ok := singleByte[n]; ok {
		var b strings.Builder
		for _, v := range raw {
			r := t[v]
			if r < 0 {
				if ignore {
					continue
				}
				return "", fmt.Errorf("%s: undefined byte %x", n, v)
			}
			b.WriteRune(r)
		}
		return b.String(), nil
	}
	if strings.HasPrefix(n, "utf") {
		return decodeUnicode(raw, n, ignore)
	}
	return decodeMulti(raw, n, ignore)
}
func Encode(text, name string, ignore bool) ([]byte, error) {
	n, e := Canonical(name)
	if e != nil {
		return nil, e
	}
	if table, ok := singleEncode[n]; ok {
		out := []byte{}
		for _, r := range text {
			if b, ok := table[r]; ok {
				out = append(out, b)
			} else if !ignore {
				return nil, fmt.Errorf("%s: unencodable rune %U", n, r)
			}
		}
		return out, nil
	}
	if strings.HasPrefix(n, "utf") {
		return encodeUnicode(text, n, ignore)
	}
	return encodeMulti(text, n, ignore)
}
