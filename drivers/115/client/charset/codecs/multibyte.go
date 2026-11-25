package codecs

// Tables are generated offline from CPython 3.12 incremental codecs. They retain
// decoder buffering and encoder shift states, including ignore-error recovery.
import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

//go:embed multibyte_big5.json.gz multibyte_big5hkscs.json.gz multibyte_cp932.json.gz multibyte_cp949.json.gz multibyte_cp950.json.gz multibyte_euc_jis_2004.json.gz multibyte_euc_jisx0213.json.gz multibyte_euc_jp.json.gz multibyte_euc_kr.json.gz multibyte_gb18030.json.gz multibyte_gb2312.json.gz multibyte_gbk.json.gz multibyte_hz.json.gz multibyte_iso2022_jp.json.gz multibyte_iso2022_jp_1.json.gz multibyte_iso2022_jp_2.json.gz multibyte_iso2022_jp_2004.json.gz multibyte_iso2022_jp_3.json.gz multibyte_iso2022_jp_ext.json.gz multibyte_iso2022_kr.json.gz multibyte_johab.json.gz multibyte_shift_jis.json.gz multibyte_shift_jis_2004.json.gz multibyte_shift_jisx0213.json.gz multibyte_*.bin.gz
var multiData embed.FS

type multiTransition struct {
	Next  int
	Text  string
	Valid bool
}

func (t *multiTransition) UnmarshalJSON(b []byte) error {
	var a []json.RawMessage
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	if len(a) != 3 {
		return fmt.Errorf("invalid decoder table")
	}
	if err := json.Unmarshal(a[0], &t.Next); err != nil {
		return err
	}
	if err := json.Unmarshal(a[1], &t.Text); err != nil {
		return err
	}
	return json.Unmarshal(a[2], &t.Valid)
}

type multiTable struct {
	P []string            `json:"p"`
	G [][3]int            `json:"g"`
	K map[uint32]string   `json:"k"`
	D [][]multiTransition `json:"d"`
	F []bool              `json:"f"`
}

var multiCache sync.Map

func isMultiByte(name string) bool {
	switch name {
	case "big5", "big5hkscs", "cp932", "cp949", "cp950", "euc_jis_2004", "euc_jisx0213", "euc_jp", "euc_kr", "gb18030", "gb2312", "gbk", "hz", "iso2022_jp", "iso2022_jp_1", "iso2022_jp_2", "iso2022_jp_2004", "iso2022_jp_3", "iso2022_jp_ext", "iso2022_kr", "johab", "shift_jis", "shift_jis_2004", "shift_jisx0213":
		return true
	}
	return false
}
func loadMulti(name string) (*multiTable, error) {
	key := name
	if v, ok := multiCache.Load(key); ok {
		return v.(*multiTable), nil
	}
	raw, err := multiData.ReadFile("multibyte_" + name + ".json.gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	b, err := io.ReadAll(z)
	if err != nil {
		return nil, err
	}
	var t multiTable
	err = json.Unmarshal(b, &t)
	if err != nil {
		return nil, err
	}
	v, _ := multiCache.LoadOrStore(key, &t)
	return v.(*multiTable), nil
}
func decodeMulti(raw []byte, name string, ignoreErrors bool) (string, error) {
	if strings.HasPrefix(name, "iso2022_") {
		return decodeISO2022(raw, name, ignoreErrors)
	}
	t, err := loadMulti(name)
	if err != nil {
		return "", err
	}
	state := 0
	var out bytes.Buffer
	for i := 0; i < len(raw); {
		if state != 0 && len(t.P) > state && ((name == "gb18030" && len(t.P[state]) == 2 && raw[i] >= 48 && raw[i] <= 57) || (name == "euc_kr" && t.P[state] == "a4" && raw[i] == 0xd4)) {
			state = 0
			i--
			continue
		}

		if state == 0 && ((name == "gb18030" && i+1 < len(raw) && raw[i] >= 0x80 && raw[i+1] >= 0x30 && raw[i+1] <= 0x39) || (name == "euc_kr" && i+1 < len(raw) && raw[i] == 0xa4 && raw[i+1] == 0xd4)) {
			n := 4
			if name == "euc_kr" {
				n = 8
			}
			if len(raw)-i < n {
				if !ignoreErrors {
					return "", fmt.Errorf("%s: incomplete sequence", name)
				}
				break
			}
			valid := false
			if name == "gb18030" {
				b := raw[i : i+4]
				if b[0] >= 0x81 && b[0] <= 0xfe && b[2] >= 0x81 && b[2] <= 0xfe && b[3] >= 0x30 && b[3] <= 0x39 {
					ix := ((int(b[0])-129)*10+int(b[1])-48)*1260 + (int(b[2])-129)*10 + int(b[3]) - 48
					for _, r := range t.G {
						if ix >= r[0] && ix <= r[1] {
							out.WriteRune(rune(r[2] + ix - r[0]))
							valid = true
							break
						}
					}
				}
			} else {
				b := raw[i : i+8]
				if b[2] == 0xa4 && b[4] == 0xa4 && b[6] == 0xa4 {
					if v, ok := t.K[uint32(b[3])<<16|uint32(b[5])<<8|uint32(b[7])]; ok {
						out.WriteString(v)
						valid = true
					}
				}
			}
			if valid {
				i += n
				continue
			}
			if !ignoreErrors {
				return "", fmt.Errorf("%s: invalid sequence at byte %d", name, i)
			}
			i++
			continue
		}
		tr := t.D[state][raw[i]]
		if !ignoreErrors && !tr.Valid {
			return "", fmt.Errorf("%s: invalid sequence at byte %d", name, i)
		}
		out.WriteString(tr.Text)
		state = tr.Next
		i++
	}
	if !ignoreErrors && !t.F[state] {
		return "", fmt.Errorf("%s: incomplete sequence", name)
	}
	return out.String(), nil
}

type multiEncoderEntry struct {
	Rune  rune
	Next  uint32
	Bytes string
}
type multiEncoderTable struct {
	Rows   [][]multiEncoderEntry
	Final  []string
	Ignore []multiEncoderEntry
}

var multiEncoderCache sync.Map

func loadMultiEncoder(name string) (*multiEncoderTable, error) {
	if v, ok := multiEncoderCache.Load(name); ok {
		return v.(*multiEncoderTable), nil
	}
	raw, err := multiData.ReadFile("multibyte_" + name + ".bin.gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	b, err := io.ReadAll(z)
	if err != nil {
		return nil, err
	}
	p := 0
	u32 := func() uint32 { v := binary.LittleEndian.Uint32(b[p : p+4]); p += 4; return v }
	u16 := func() int { v := int(binary.LittleEndian.Uint16(b[p : p+2])); p += 2; return v }
	payload := func(n int) string { v := string(b[p : p+n]); p += n; return v }
	n := int(u32())
	t := &multiEncoderTable{Rows: make([][]multiEncoderEntry, n), Final: make([]string, n), Ignore: make([]multiEncoderEntry, n)}
	for i := 0; i < n; i++ {
		count := int(u32())
		row := make([]multiEncoderEntry, count)
		for j := range row {
			cp := u32()
			next := u32()
			ln := u16()
			row[j] = multiEncoderEntry{rune(cp), next, payload(ln)}
		}
		t.Rows[i] = row
		u32()
		t.Final[i] = payload(u16())
		next := u32()
		ln := u16()
		t.Ignore[i] = multiEncoderEntry{Next: next, Bytes: payload(ln)}
	}
	v, _ := multiEncoderCache.LoadOrStore(name, t)
	return v.(*multiEncoderTable), nil
}
func encodeMulti(text string, name string, ignoreErrors bool) ([]byte, error) {
	t, err := loadMultiEncoder(name)
	if err != nil {
		return nil, err
	}
	var ranges [][3]int
	if name == "gb18030" {
		d, e := loadMulti(name)
		if e != nil {
			return nil, e
		}
		ranges = d.G
	}
	state := 0
	var out []byte
	for _, r := range text {
		row := t.Rows[state]
		i := sort.Search(len(row), func(i int) bool { return row[i].Rune >= r })
		if i < len(row) && row[i].Rune == r {
			v := row[i]
			out = append(out, v.Bytes...)
			state = int(v.Next)
			continue
		}
		found := false
		for _, v := range ranges {
			if int(r) >= v[2] && int(r) <= v[2]+v[1]-v[0] {
				ix := v[0] + int(r) - v[2]
				b4 := byte(ix%10 + 48)
				ix /= 10
				b3 := byte(ix%126 + 129)
				ix /= 126
				b2 := byte(ix%10 + 48)
				b1 := byte(ix/10 + 129)
				out = append(out, b1, b2, b3, b4)
				found = true
				break
			}
		}
		if found {
			continue
		}
		if !ignoreErrors {
			return nil, fmt.Errorf("%s: cannot encode U+%04X", name, r)
		}
		v := t.Ignore[state]
		out = append(out, v.Bytes...)
		state = int(v.Next)
	}
	return append(out, t.Final[state]...), nil
}
