package codecs

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestMultibytePythonVectors(t *testing.T) {
	f, err := os.Open("multibyte_vectors.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	b, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]json.RawMessage
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var n, h, want string
		var valid bool
		json.Unmarshal(row[0], &n)
		json.Unmarshal(row[1], &h)
		json.Unmarshal(row[2], &want)
		json.Unmarshal(row[3], &valid)
		raw, _ := hex.DecodeString(h)
		got, err := decodeMulti(raw, n, true)
		if err != nil || got != want {
			t.Fatalf("%s %s ignore got %q err %v want %q", n, h, got, err, want)
		}
		got, err = decodeMulti(raw, n, false)
		if (err == nil) != valid || valid && got != want {
			t.Fatalf("%s %s strict got %q err %v want %q valid %v", n, h, got, err, want, valid)
		}
	}
}

func TestMultibytePythonEncodeVectors(t *testing.T) {
	f, err := os.Open("multibyte_encode_vectors.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	b, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]json.RawMessage
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var n, text, want string
		var valid bool
		json.Unmarshal(row[0], &n)
		json.Unmarshal(row[1], &text)
		json.Unmarshal(row[2], &want)
		json.Unmarshal(row[3], &valid)
		got, err := encodeMulti(text, n, true)
		if err != nil || hex.EncodeToString(got) != want {
			t.Fatalf("%s %q ignore got %x err %v want %s", n, text, got, err, want)
		}
		got, err = encodeMulti(text, n, false)
		if (err == nil) != valid || valid && hex.EncodeToString(got) != want {
			t.Fatalf("%s %q strict got %x err %v want %s valid %v", n, text, got, err, want, valid)
		}
	}
}
