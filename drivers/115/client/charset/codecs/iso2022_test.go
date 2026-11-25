package codecs

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"testing"
)

//go:embed iso2022_vectors.json.gz
var isoVectors []byte

func TestISO2022Oracle(t *testing.T) {
	r, e := gzip.NewReader(bytes.NewReader(isoVectors))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	var rows [][]json.RawMessage
	if e = json.NewDecoder(r).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	for _, v := range rows {
		var name, h, want string
		var ignore, bad bool
		json.Unmarshal(v[0], &name)
		json.Unmarshal(v[1], &h)
		json.Unmarshal(v[2], &ignore)
		json.Unmarshal(v[3], &want)
		json.Unmarshal(v[4], &bad)
		raw, _ := hex.DecodeString(h)
		got, err := decodeISO2022(raw, name, ignore)
		if (err != nil) != bad || err == nil && got != want {
			t.Fatalf("%s %s ignore=%v got=%q err=%v want=%q bad=%v", name, h, ignore, got, err, want, bad)
		}
	}
}
