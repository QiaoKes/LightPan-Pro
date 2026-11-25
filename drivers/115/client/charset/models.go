package charset

import (
	"litepan/drivers/115/client/charset/codecs"
	"math"
)

// charsetMatch mirrors CharsetMatch in charset-normalizer 3.5.1 models.py.
// Decoded text itself is the equivalence key: Python's hash is only an
// implementation detail and must never introduce cross-process state here.
type charsetMatch struct {
	encoding  string
	chaos     float64
	bom       bool
	languages []CoherenceMatch
	text      string
	rawLength int
	leaves    []*charsetMatch
}

func (m *charsetMatch) coherence() float64 {
	if len(m.languages) == 0 {
		return 0
	}
	return m.languages[0].Ratio
}
func (m *charsetMatch) less(other *charsetMatch) bool {
	chaosDifference := math.Abs(m.chaos - other.chaos)
	coherenceDifference := math.Abs(m.coherence() - other.coherence())
	if chaosDifference < 0.005 {
		if coherenceDifference > 0.02 {
			return m.coherence() > other.coherence()
		}
		if m.rawLength >= 10000000 {
			return m.chaos < other.chaos
		}
		return m.multiByteUsage() > other.multiByteUsage()
	}
	return m.chaos < other.chaos
}
func (m *charsetMatch) multiByteUsage() float64 {
	if m.rawLength == 0 {
		return 0
	}
	return 1 - float64(PythonRuneCount(m.text))/float64(m.rawLength)
}
func (m *charsetMatch) language() string {
	if len(m.languages) > 0 {
		return m.languages[0].Language
	}
	if m.encoding == "ascii" {
		return "English"
	}
	for _, leaf := range m.leaves {
		if leaf.encoding == "ascii" {
			return "English"
		}
	}
	languages := EncodingLanguages(m.encoding)
	if codecs.IsMultiByte(m.encoding) {
		languages = MBEncodingLanguages(m.encoding)
	}
	if len(languages) == 0 {
		return "Unknown"
	}
	for _, language := range languages {
		if language == "Latin Based" {
			return "Unknown"
		}
	}
	return languages[0]
}

type charsetMatches struct{ items []*charsetMatch }

func (matches *charsetMatches) append(m *charsetMatch) {
	if m.rawLength < 10000000 {
		for _, existing := range matches.items {
			if existing.text == m.text && existing.chaos == m.chaos {
				existing.leaves = append(existing.leaves, m)
				return
			}
		}
	}
	matches.items = append(matches.items, m)
}
func (matches *charsetMatches) best() *charsetMatch {
	if len(matches.items) == 0 {
		return nil
	}
	pythonMatchSort(matches.items)
	return matches.items[0]
}

// pythonMatchSort mirrors CPython natural-run detection and stable binary
// insertion for short lists (CPython uses a single minrun below 64).
func pythonMatchSort(items []*charsetMatch) {
	n := len(items)
	if n >= 64 {
		r, v := 0, n
		for v >= 64 {
			r |= v & 1
			v >>= 1
		}
		split := v + r
		// At the detector's <=100 candidate bound there are at most two runs.
		// Preserve a naturally longer first run before extending to minrun.
		first := 2
		descending := items[1].less(items[0])
		for first < n && items[first].less(items[first-1]) == descending {
			first++
		}
		if descending {
			for i, j := 0, first-1; i < j; i, j = i+1, j-1 {
				items[i], items[j] = items[j], items[i]
			}
		}
		if first < split {
			pythonBinaryExtend(items[:split], first)
			first = split
		}
		if first == n {
			return
		}
		pythonMatchSort(items[first:])
		pythonMerge(items, first)
		return
	}
	if n < 2 {
		return
	}

	run := 2
	if items[1].less(items[0]) {
		for run < n && items[run].less(items[run-1]) {
			run++
		}
		for i, j := 0, run-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	} else {
		for run < n && !items[run].less(items[run-1]) {
			run++
		}
	}
	for start := run; start < n; start++ {
		pivot := items[start]
		lo, hi := 0, start
		for lo < hi {
			mid := lo + (hi-lo)/2
			if pivot.less(items[mid]) {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		copy(items[lo+1:start+1], items[lo:start])
		items[lo] = pivot
	}
}

// pythonGallop returns the CPython gallop_left/right insertion position.
// Exponential probing order matters because the ranking relation is not transitive.
func pythonGallop(key *charsetMatch, a []*charsetMatch, hint int, right bool) int {
	less := func(x, y *charsetMatch) bool { return x.less(y) }
	ofs, last := 1, 0
	if (!right && less(a[hint], key)) || (right && !less(key, a[hint])) {
		limit := len(a) - hint
		for ofs < limit {
			advance := less(a[hint+ofs], key)
			if right {
				advance = !less(key, a[hint+ofs])
			}
			if !advance {
				break
			}
			last = ofs
			ofs = ofs*2 + 1
		}
		ofs = min(ofs, limit)
		last += hint
		ofs += hint
	} else {
		limit := hint + 1
		for ofs < limit {
			advance := !less(a[hint-ofs], key)
			if right {
				advance = less(key, a[hint-ofs])
			}
			if !advance {
				break
			}
			last = ofs
			ofs = ofs*2 + 1
		}
		ofs = min(ofs, limit)
		last, ofs = hint-ofs, hint-last
	}
	last++
	for last < ofs {
		mid := last + (ofs-last)/2
		advance := less(a[mid], key)
		if right {
			advance = !less(key, a[mid])
		}
		if advance {
			last = mid + 1
		} else {
			ofs = mid
		}
	}
	return ofs
}

func pythonBinaryExtend(a []*charsetMatch, start int) {
	for ; start < len(a); start++ {
		p := a[start]
		lo, hi := 0, start
		for lo < hi {
			m := (lo + hi) / 2
			if p.less(a[m]) {
				hi = m
			} else {
				lo = m + 1
			}
		}
		copy(a[lo+1:start+1], a[lo:start])
		a[lo] = p
	}
}

// pythonMerge ports CPython 3.12.13 listobject.c merge_at/merge_lo/merge_hi.
// Galloping and its comparison order are observable with this nontransitive rank.
func pythonMerge(a []*charsetMatch, split int) {
	k := pythonGallop(a[split], a[:split], 0, true)
	left := a[k:split]
	right := a[split:]
	if len(left) == 0 {
		return
	}
	right = right[:pythonGallop(left[len(left)-1], right, len(right)-1, false)]
	if len(right) == 0 {
		return
	}
	na, nb := len(left), len(right)
	mg := 7
	if na <= nb {
		tmp := append([]*charsetMatch(nil), left...)
		i, j, d := 0, split, k
		putA := func() { a[d] = tmp[i]; d++; i++; na-- }
		putB := func() { a[d] = a[j]; d++; j++; nb-- }
		finish := func() {
			if na == 1 && nb > 0 {
				copy(a[d:], a[j:j+nb])
				a[d+nb] = tmp[i]
			} else {
				copy(a[d:], tmp[i:i+na])
			}
		}
		putB()
		if nb == 0 || na == 1 {
			finish()
			return
		}
		for {
			ac, bc := 0, 0
			for {
				if a[j].less(tmp[i]) {
					putB()
					bc++
					ac = 0
					if nb == 0 {
						finish()
						return
					}
					if bc >= mg {
						break
					}
				} else {
					putA()
					ac++
					bc = 0
					if na == 1 {
						finish()
						return
					}
					if ac >= mg {
						break
					}
				}
			}
			mg++
			for {
				if mg > 1 {
					mg--
				}
				ac = pythonGallop(a[j], tmp[i:i+na], 0, true)
				copy(a[d:], tmp[i:i+ac])
				d += ac
				i += ac
				na -= ac
				if na <= 1 {
					finish()
					return
				}
				putB()
				if nb == 0 {
					finish()
					return
				}
				bc = pythonGallop(tmp[i], a[j:j+nb], 0, false)
				copy(a[d:], a[j:j+bc])
				d += bc
				j += bc
				nb -= bc
				if nb == 0 {
					finish()
					return
				}
				putA()
				if na == 1 {
					finish()
					return
				}
				if ac < 7 && bc < 7 {
					break
				}
			}
			mg++
		}
	}
	tmp := append([]*charsetMatch(nil), right...)
	i, j, d := split-1, nb-1, split+nb-1
	putA := func() { a[d] = a[i]; d--; i--; na-- }
	putB := func() { a[d] = tmp[j]; d--; j--; nb-- }
	finish := func() {
		if nb == 1 && na > 0 {
			copy(a[d-na+1:d+1], a[i-na+1:i+1])
			a[d-na] = tmp[j]
		} else {
			copy(a[d-nb+1:d+1], tmp[:nb])
		}
	}
	putA()
	if na == 0 || nb == 1 {
		finish()
		return
	}
	for {
		ac, bc := 0, 0
		for {
			if tmp[j].less(a[i]) {
				putA()
				ac++
				bc = 0
				if na == 0 {
					finish()
					return
				}
				if ac >= mg {
					break
				}
			} else {
				putB()
				bc++
				ac = 0
				if nb == 1 {
					finish()
					return
				}
				if bc >= mg {
					break
				}
			}
		}
		mg++
		for {
			if mg > 1 {
				mg--
			}
			ac = na - pythonGallop(tmp[j], a[k:k+na], na-1, true)
			copy(a[d-ac+1:d+1], a[i-ac+1:i+1])
			d -= ac
			i -= ac
			na -= ac
			if na == 0 {
				finish()
				return
			}
			putB()
			if nb == 1 {
				finish()
				return
			}
			bc = nb - pythonGallop(a[i], tmp[:nb], nb-1, false)
			copy(a[d-bc+1:d+1], tmp[j-bc+1:j+1])
			d -= bc
			j -= bc
			nb -= bc
			if nb <= 1 {
				finish()
				return
			}
			putA()
			if na == 0 {
				finish()
				return
			}
			if ac < 7 && bc < 7 {
				break
			}
		}
		mg++
	}
}
