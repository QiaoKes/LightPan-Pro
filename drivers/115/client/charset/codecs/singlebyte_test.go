package codecs

import (
	"bytes"
	"testing"
)

func TestSingleByteTables(t *testing.T) {
	for name, table := range singleByte {
		for i, r := range table {
			s, e := Decode([]byte{byte(i)}, name, false)
			if r < 0 {
				if e == nil {
					t.Fatalf("%s %x accepted undefined", name, i)
				}
				s, e = Decode([]byte{byte(i)}, name, true)
				if e != nil || s != "" {
					t.Fatal(name, i, s, e)
				}
				continue
			}
			if e != nil || s != string(r) {
				t.Fatal(name, i, s, e)
			}
			encoded, e := Encode(s, name, false)
			if e != nil || !bytes.Equal(encoded, []byte{byte(i)}) { // Python maps duplicate decoded characters to a single preferred byte.
				if e != nil {
					t.Fatal(e)
				}
				again, e := Decode(encoded, name, false)
				if e != nil || again != s {
					t.Fatal(name, i)
				}
			}
		}
	}
}
func TestUnicodeMalformed(t *testing.T) {
	for _, v := range []struct {
		name string
		raw  []byte
		want string
	}{{"utf8", []byte{'a', 255, 'b'}, "ab"}, {"utf-16-le", []byte{97, 0, 0, 216, 98, 0}, "ab"}, {"utf-32-be", []byte{0, 0, 0, 97, 0, 17, 0, 0}, "a"}, {"cp1252", []byte{97, 129, 98}, "ab"}} {
		s, e := Decode(v.raw, v.name, true)
		if e != nil || s != v.want {
			t.Fatalf("%s got %q %v", v.name, s, e)
		}
		if _, e = Decode(v.raw, v.name, false); e == nil {
			t.Fatal("strict accepted", v.name)
		}
	}
}
func TestUnicodeRoundtrip(t *testing.T) {
	for _, n := range []string{"utf8", "utf-8-sig", "utf16", "utf-16-be", "utf32", "utf-32-le", "utf7"} {
		want := "abc 日本😀 +"
		b, e := Encode(want, n, false)
		if e != nil {
			t.Fatal(e)
		}
		got, e := Decode(b, n, false)
		if e != nil || got != want {
			t.Fatalf("%s %q %v %x", n, got, e, b)
		}
	}
}
