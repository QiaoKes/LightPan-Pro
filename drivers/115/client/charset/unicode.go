package charset

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed unicode_data.json.gz
var unicodeData []byte

// CharInfo holds Python 3.12 Unicode 15.0 character semantics.
type CharInfo struct {
	Character                                                                                                                                                                                                                                            rune
	Printable, Alpha, Upper, Lower, Space, Digit, IsASCII, CaseVariable, Accentuated, Latin, IsCJK, IsKatakana, IsHalfwidthKatakana, IsArabic, IsLigature, IsSuperscript, IsSentenceOpenPunctuation, IsGlyph, Punct, Sym, Sep, Emoticon, Safe, CommonCJK bool
	Flags                                                                                                                                                                                                                                                int
	Range, Unaccented                                                                                                                                                                                                                                    string
}
type unicodeRow struct {
	start, end, bits, flags int
	name                    string
}

var unicodeRows []unicodeRow
var lowerMap, accentMap, removeMap map[string]string
var casedMap, ignorableMap map[rune]bool
var secondaryMap map[string]bool
var suspiciousMap map[[2]string]bool

func init() {
	z, e := gzip.NewReader(bytes.NewReader(unicodeData))
	if e != nil {
		panic(e)
	}
	raw, e := io.ReadAll(z)
	if e != nil {
		panic(e)
	}
	_ = z.Close()
	var d struct {
		Rows                    [][]json.RawMessage
		Lower, Unaccent, Remove map[string]string
		Cased, Ignorable        []rune
		Secondary               []string
		Susp                    [][2]string
	}
	if e = json.Unmarshal(raw, &d); e != nil {
		panic(e)
	}
	for _, r := range d.Rows {
		var v unicodeRow
		_ = json.Unmarshal(r[0], &v.start)
		_ = json.Unmarshal(r[1], &v.end)
		_ = json.Unmarshal(r[2], &v.bits)
		_ = json.Unmarshal(r[3], &v.flags)
		_ = json.Unmarshal(r[4], &v.name)
		unicodeRows = append(unicodeRows, v)
	}
	removeMap = d.Remove
	casedMap = map[rune]bool{}
	ignorableMap = map[rune]bool{}
	for _, c := range d.Cased {
		casedMap[c] = true
	}
	for _, c := range d.Ignorable {
		ignorableMap[c] = true
	}
	lowerMap = d.Lower
	accentMap = d.Unaccent
	secondaryMap = map[string]bool{}
	for _, s := range d.Secondary {
		secondaryMap[s] = true
	}
	suspiciousMap = map[[2]string]bool{}
	for _, p := range d.Susp {
		suspiciousMap[p] = true
	}
}
func CharInfoFor(c rune) CharInfo {
	i := CharInfo{Character: c, Unaccented: PythonRuneString(c)}
	n := int(c)
	k := sort.Search(len(unicodeRows), func(k int) bool { return unicodeRows[k].end > n })
	if k == len(unicodeRows) || n < 0 {
		return i
	}
	v := unicodeRows[k]
	i.Flags = v.flags
	i.Range = v.name
	fields := []*bool{&i.Printable, &i.Alpha, &i.Upper, &i.Lower, &i.Space, &i.Digit, &i.IsASCII, &i.CaseVariable, &i.Accentuated, &i.Latin, &i.IsCJK, &i.IsKatakana, &i.IsHalfwidthKatakana, &i.IsArabic, &i.IsLigature, &i.IsSuperscript, &i.IsSentenceOpenPunctuation, &i.IsGlyph, &i.Punct, &i.Sym, &i.Sep, &i.Emoticon, &i.Safe, &i.CommonCJK}
	for k, p := range fields {
		*p = v.bits&(1<<k) != 0
	}
	if s, ok := accentMap[strconv.Itoa(n)]; ok {
		i.Unaccented = s
	}
	return i
}
func UnicodeRange(c rune) string            { return CharInfoFor(c).Range }
func IsUnicodeRangeSecondary(s string) bool { return secondaryMap[s] }
func IsSuspiciouslySuccessiveRange(a, b string) bool {
	if a == "" || b == "" {
		return true
	}
	return suspiciousMap[[2]string{a, b}]
}
func RemoveAccent(c rune) string {
	if s, ok := removeMap[strconv.Itoa(int(c))]; ok {
		return s
	}
	return PythonRuneString(c)
}
func PythonLower(s string) string {
	var b strings.Builder
	chars := PythonRunes(s)
	for k, c := range chars {
		if c == 'Σ' {
			before, after := false, false
			for j := k - 1; j >= 0; j-- {
				if ignorableMap[chars[j]] {
					continue
				}
				before = casedMap[chars[j]]
				break
			}
			for j := k + 1; j < len(chars); j++ {
				if ignorableMap[chars[j]] {
					continue
				}
				after = casedMap[chars[j]]
				break
			}
			if before && !after {
				b.WriteRune('ς')
				continue
			}
		}
		if t, ok := lowerMap[strconv.Itoa(int(c))]; ok {
			b.WriteString(t)
		} else {
			b.WriteString(PythonRuneString(c))
		}
	}
	return b.String()
}

// PythonRunes returns Python characters, preserving surrogate code points stored
// as WTF-8 by codecs such as UTF-7. Other malformed UTF-8 retains Go's RuneError
// behavior; decoded codec strings contain only UTF-8 or these surrogate triples.
func PythonRunes(text string) []rune {
	if utf8.ValidString(text) {
		return []rune(text)
	}
	chars := make([]rune, 0, len(text))
	for len(text) > 0 {
		c, n := pythonDecodeRune(text)
		chars = append(chars, c)
		text = text[n:]
	}
	return chars
}

// PythonRuneCount counts Python characters without allocating.
func PythonRuneCount(text string) int {
	count := 0
	for len(text) > 0 {
		_, n := pythonDecodeRune(text)
		count++
		text = text[n:]
	}
	return count
}
func pythonDecodeRune(text string) (rune, int) {
	if text[0] < utf8.RuneSelf {
		return rune(text[0]), 1
	}
	if len(text) >= 3 && text[0] == 0xed && text[1] >= 0xa0 && text[1] <= 0xbf && text[2] >= 0x80 && text[2] <= 0xbf {
		return rune(text[0]&15)<<12 | rune(text[1]&63)<<6 | rune(text[2]&63), 3
	}
	return utf8.DecodeRuneInString(text)
}

// PythonRuneString serializes a Python character, including lone surrogates.
func PythonRuneString(c rune) string {
	if c >= 0xd800 && c <= 0xdfff {
		return string([]byte{0xe0 | byte(c>>12), 0x80 | byte(c>>6)&63, 0x80 | byte(c)&63})
	}
	return string(c)
}
