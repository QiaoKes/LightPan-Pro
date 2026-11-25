package charset

import (
	"bytes"
	"litepan/drivers/115/client/charset/codecs"
	"regexp"
	"strings"
)

var declarationPattern = regexp.MustCompile(`(?i)(?:encoding|charset|coding)[:= ]{1,10}["']?([a-zA-Z0-9_-]+)["']?`)

func specifiedEncoding(raw []byte) string {
	raw = raw[:min(len(raw), 8192)]
	lowered := bytes.ToLower(raw)
	if !bytes.Contains(lowered, []byte("coding")) && !bytes.Contains(lowered, []byte("charset")) {
		return ""
	}
	ascii := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b < 128 {
			ascii = append(ascii, b)
		}
	}
	for _, m := range declarationPattern.FindAllSubmatch(ascii, -1) {
		if name, ok := declarationNames[strings.ReplaceAll(strings.ToLower(string(m[1])), "-", "_")]; ok {
			return name
		}
	}
	return ""
}
func signature(raw []byte) (string, []byte) {
	for _, m := range detectorMarks {
		if bytes.HasPrefix(raw, m.mark) {
			return m.name, m.mark
		}
	}
	return "", nil
}
func runeSlice(s string, start, end int) string {
	r := PythonRunes(s)
	start = max(0, min(start, len(r)))
	end = max(start, min(end, len(r)))
	var out strings.Builder
	for _, c := range r[start:end] {
		out.WriteString(PythonRuneString(c))
	}
	return out.String()
}

// sequenceChunks follows utils.cut_sequence_chunks, including the offset
// coordinate differences between stateful, single-byte and multibyte codecs.
func sequenceChunks(raw []byte, name string, offsets []int, size int, bom, strip bool, mark []byte, multi bool, decoded *string, deferred bool, yield func(string) bool) error {
	for _, offset := range offsets {
		var chunk string
		var err error
		if decoded != nil && *decoded != "" && strings.HasPrefix(name, "iso2022_") {
			start := offset * PythonRuneCount(*decoded) / len(raw)
			chunk = runeSlice(*decoded, start, start+size)
			if chunk == "" {
				break
			}
		} else if decoded != nil && *decoded != "" && !multi {
			chunk = runeSlice(*decoded, offset, offset+size)
			if chunk == "" {
				break
			}
		} else if deferred {
			base := raw
			if strip {
				base = base[len(mark):]
			}
			if offset >= len(base) {
				break
			}
			chunk, err = codecs.Decode(base[offset:min(offset+size, len(base))], name, false)
		} else {
			end := offset + size
			if end > len(raw)+8 {
				continue
			}
			part := raw[offset:min(end, len(raw))]
			if bom && !strip {
				part = append(append([]byte{}, mark...), part...)
			}
			chunk, err = codecs.Decode(part, name, multi)
			if err == nil && multi && offset > 0 && decoded != nil && *decoded != "" && !strings.Contains(*decoded, runeSlice(chunk, 0, min(size, 16))) {
				for j := offset; j > offset-4; j-- {
					start := j
					if start < 0 {
						start = max(0, len(raw)+start)
					}
					part = raw[start:min(end, len(raw))]
					if bom && !strip {
						part = append(append([]byte{}, mark...), part...)
					}
					chunk, err = codecs.Decode(part, name, true)
					if err != nil || strings.Contains(*decoded, runeSlice(chunk, 0, min(size, 16))) {
						break
					}
				}
			}
		}
		if err != nil {
			return err
		}
		if !yield(chunk) {
			break
		}
	}
	return nil
}
