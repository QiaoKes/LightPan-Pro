package charset

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestMessOracle(t *testing.T) {
	raw, e := os.ReadFile("mess_vectors_test.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	z, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	raw, e = io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Text            string
		Threshold, Want float64
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	for k, r := range rows {
		if got := MessRatio(r.Text, r.Threshold); got != r.Want {
			t.Errorf("vector %d threshold %g got %g want %g", k, r.Threshold, got, r.Want)
		}
	}
}
func TestMessConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for k := 0; k < 16; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 30; n++ {
				MessRatio("中文éÉabc\x00", .2)
			}
		}()
	}
	wg.Wait()
}

func TestPinnedUnicode(t *testing.T) {
	for _, v := range []struct{ in, want string }{{"ΟΣ", "ος"}, {"ΟΣΑ", "οσα"}, {"AΣ\u0301", "aς\u0301"}, {"AΣ\u0301A", "aσ\u0301a"}, {"İ", "i\u0307"}, {"Σ", "σ"}} {
		if got := PythonLower(v.in); got != v.want {
			t.Errorf("lower %q got %q want %q", v.in, got, v.want)
		}
	}
	if !CharInfoFor('²').Digit || CharInfoFor('²').Alpha {
		t.Error("Python digit semantics")
	}
	if CharInfoFor('\x1c').Space {
		t.Error("CharInfo ASCII fast path must preserve source whitespace exception")
	}
	if !CharInfoFor('é').Accentuated || RemoveAccent('é') != "e" {
		t.Error("accent semantics")
	}
	if UnicodeRange('a') != "Basic Latin" || UnicodeRange('\n') != "Control character" {
		t.Error("ranges")
	}
	if IsSuspiciouslySuccessiveRange("Basic Latin", "CJK Unified Ideographs") {
		t.Error("Basic Latin CJK compatible")
	}
}

func TestSurrogateOracle(t *testing.T) {
	raw, e := os.ReadFile("surrogate_vectors_test.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Runes, Lower    []rune
		Threshold, Want float64
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	serialize := func(chars []rune) string {
		var b strings.Builder
		for _, c := range chars {
			b.WriteString(PythonRuneString(c))
		}
		return b.String()
	}
	for k, r := range rows {
		text := serialize(r.Runes)
		if got := PythonRunes(text); !slices.Equal(got, r.Runes) {
			t.Errorf("vector %d characters differ", k)
		}
		if got := PythonRuneCount(text); got != len(r.Runes) {
			t.Errorf("vector %d count %d want %d", k, got, len(r.Runes))
		}
		if got := PythonRunes(PythonLower(text)); !slices.Equal(got, r.Lower) {
			t.Errorf("vector %d lowercase differs", k)
		}
		if got := MessRatio(text, r.Threshold); got != r.Want {
			t.Errorf("vector %d mess %g want %g", k, got, r.Want)
		}
	}
	// UTF-7 +2AA- decodes to this exact surrogate triple, not three replacements.
	if got := PythonRunes("\xed\xa0\x80"); !slices.Equal(got, []rune{0xd800}) {
		t.Fatalf("WTF-8 direct fixture: %x", got)
	}
	for c := rune(0xd800); c <= 0xdfff; c++ {
		text := PythonRuneString(c)
		if got := PythonRunes(text); !slices.Equal(got, []rune{c}) {
			t.Fatalf("surrogate %x round trip %x", c, got)
		}
		info := CharInfoFor(c)
		if info.Printable || info.Alpha || info.Space || info.Range != "High Surrogates" && info.Range != "High Private Use Surrogates" && info.Range != "Low Surrogates" {
			t.Fatalf("surrogate %x properties %+v", c, info)
		}
		if info.Unaccented != text || RemoveAccent(c) != text {
			t.Fatalf("surrogate %x lost by accent helper", c)
		}
	}
}
