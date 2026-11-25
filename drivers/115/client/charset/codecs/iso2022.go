package codecs

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed iso2022_maps.json.gz
var isoMapData []byte
var isoOnce sync.Once
var isoMaps map[string]map[string]map[string]string

// decodeISO2022 follows CPython 3.12.13 Modules/cjkcodecs/_codecs_iso2022.c.
func decodeISO2022(raw []byte, name string, ignoreErrors bool) (string, error) {
	isoOnce.Do(func() {
		r, e := gzip.NewReader(bytes.NewReader(isoMapData))
		if e != nil {
			panic(e)
		}
		defer r.Close()
		if e = json.NewDecoder(r).Decode(&isoMaps); e != nil {
			panic(e)
		}
	})
	maps, ok := isoMaps[name]
	if !ok {
		return "", fmt.Errorf("unknown ISO2022 codec %s", name)
	}
	g := [3]byte{'B', 'B', 'B'}
	shifted, through := false, false
	kr := name == "iso2022_kr"
	g2 := name == "iso2022_jp_2"
	var out strings.Builder
	end := func(c byte) bool { return c == '@' || c >= 'A' && c <= 'Z' }
	for i := 0; i < len(raw); {
		c := raw[i]
		n := 1
		bad := false
		if through {
			out.WriteRune(rune(c))
			i++
			if end(c) {
				through = false
			}
			continue
		}
		switch c {
		case 27:
			if i+1 >= len(raw) {
				n = len(raw) - i
				bad = true
				break
			}
			h := raw[i+1]
			if h == '(' || h == ')' || h == '$' || h == '.' || h == '&' {
				l := 0
				j := 1
				for ; j < 16; j++ {
					if i+j >= len(raw) {
						break
					}
					if end(raw[i+j]) {
						l = j + 1
						break
					}
					if !kr && raw[i+j] == '&' && i+j+1 < len(raw) && raw[i+j+1] == '@' {
						j += 2
					}
				}
				if l == 0 {
					bad = true
					if j < 16 {
						n = len(raw) - i
					}
					break
				}
				n = l
				plane := -1
				var cs byte
				switch l {
				case 3:
					cs = raw[i+2]
					switch h {
					case '$':
						cs |= 128
						plane = 0
					case '(':
						plane = 0
					case ')':
						plane = 1
					case '.':
						if g2 {
							plane = 2
						}
					}
				case 4:
					if h == '$' {
						cs = raw[i+3] | 128
						if raw[i+2] == '(' {
							plane = 0
						} else if raw[i+2] == ')' {
							plane = 1
						}
					}
				case 6:
					if !kr && raw[i+3] == 27 && raw[i+4] == '$' && raw[i+5] == 'B' {
						cs = 'B' | 128
						plane = 0
					}
				}
				key := string([]byte{cs & 127, 's'})
				if cs&128 != 0 {
					key = string([]byte{cs & 127, 'd'})
				}
				_, valid := maps[key]
				if plane < 0 || cs != 'B' && !valid {
					bad = true
				} else {
					g[plane] = cs
				}
			} else if g2 && h == 'N' {
				n = 3
				if len(raw)-i < 3 {
					n = len(raw) - i
					bad = true
					break
				}
				v := raw[i+2]
				if g[2] != 'B' && g[2] != 'A' && g[2] != 'F' {
					return "", fmt.Errorf("%s internal codec error", name)
				}
				if g[2] == 'B' {
					if v >= 128 {
						bad = true
					} else {
						out.WriteByte(v)
					}
				} else {
					key := string([]byte{g[2], 's'})
					s, yes := maps[key][strconv.Itoa(int(v))]
					if !yes {
						bad = true
					} else {
						out.WriteString(s)
					}
				}
			} else {
				out.WriteByte(27)
				through = true
			}
		case 14, 15:
			if kr {
				shifted = c == 14
			} else {
				out.WriteByte(c)
			}
		case 10:
			shifted = false
			out.WriteByte(c)
		default:
			if c < 32 {
				out.WriteByte(c)
			} else if c >= 128 {
				bad = true
			} else {
				cs := g[0]
				if shifted {
					cs = g[1]
				}
				if cs == 'B' {
					out.WriteByte(c)
				} else {
					key := string([]byte{cs & 127, 's'})
					v := int(c)
					if cs&128 != 0 {
						key = string([]byte{cs & 127, 'd'})
						n = 2
						if len(raw)-i < 2 {
							bad = true
							break
						}
						v = v*256 + int(raw[i+1])
					}
					s, yes := maps[key][strconv.Itoa(v)]
					if !yes {
						bad = true
					} else {
						out.WriteString(s)
					}
				}
			}
		}
		if bad && !ignoreErrors {
			return "", fmt.Errorf("%s invalid sequence at byte %d", name, i)
		}
		i += n
	}
	return out.String(), nil
}
