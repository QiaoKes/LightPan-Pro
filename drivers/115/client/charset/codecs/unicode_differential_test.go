package codecs

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestPythonUnicodeDifferential(t *testing.T) {
	var v struct {
		Decode []struct {
			Name, Raw, Text, Texthex string
			Ignore, Error            bool
		}
		Encode []struct {
			Name, Raw, Text, Texthex string
			Ignore, Error            bool
		}
		Aliases []struct{ Name, Canonical string }
	}
	b, e := os.ReadFile("unicode_vectors.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	b, err = io.ReadAll(zr)
	zr.Close()
	if err != nil {
		t.Fatal(err)
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, x := range v.Decode {
		raw, _ := hex.DecodeString(x.Raw)
		got, e := Decode(raw, x.Name, x.Ignore)
		if (e != nil) != x.Error || e == nil && hex.EncodeToString([]byte(got)) != x.Texthex {
			if count < 25 {
				t.Errorf("%s %s ignore=%v got=%q err=%v want=%q err=%v", x.Name, x.Raw, x.Ignore, got, e, x.Text, x.Error)
			}
			count++
		}
	}
	for _, x := range v.Encode {
		want, _ := hex.DecodeString(x.Raw)
		input, _ := hex.DecodeString(x.Texthex)
		got, e := Encode(string(input), x.Name, x.Ignore)
		if (e != nil) != x.Error || e == nil && !bytes.Equal(got, want) {
			t.Errorf("encode %s %q got %x want %x err %v", x.Name, x.Text, got, want, e)
		}
	}
	for _, x := range v.Aliases {
		got, e := Canonical(x.Name)
		if (e != nil) != (x.Canonical == "") || e == nil && got != x.Canonical {
			if count < 25 {
				t.Errorf("alias %q got %q %v want %q", x.Name, got, e, x.Canonical)
			}
			count++
		}
	}
	if count > 0 {
		t.Fatalf("%d differential mismatches", count)
	}
}
