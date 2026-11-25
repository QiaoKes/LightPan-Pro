package charset

type messState struct {
	wordInternalLigature, wordLastLigature                                                                       bool
	sp, symbols, punct, ta, acc, up, unprint, dup, dupN, sr, susp, cjk, uncommon, kata, half, kc, ku, arab, isol int
	escape                                                                                                       bool
	last                                                                                                         rune
	prev                                                                                                         CharInfo
	hasPrev                                                                                                      bool
	lastRange                                                                                                    string
	hasRange                                                                                                     bool
	words, badChars, wordChars, foreign, invalid                                                                 int
	word                                                                                                         []CharInfo
	bad, invalidWord, foreignWatch                                                                               bool
	auN, auChunk, auSwitch, auFinal                                                                              int
	auBuf, auASCII                                                                                               bool
	auPrev                                                                                                       CharInfo
}

func (s *messState) wordFeed(i CharInfo) {
	if i.Alpha {
		if s.wordLastLigature {
			s.wordInternalLigature = true
		}
		s.wordLastLigature = i.IsLigature
		s.word = append(s.word, i)
		if !i.IsGlyph && (!i.Latin || i.Accentuated) {
			s.foreignWatch = true
		}
		return
	}
	n := len(s.word)
	if n == 0 {
		return
	}
	internal := s.wordInternalLigature
	if i.IsSentenceOpenPunctuation || (i.IsSuperscript && internal) {
		s.bad = true
		s.invalidWord = true
	}
	if i.Space || i.Punct || i.Sep {
		s.words++
		s.wordChars += n
		a, g, u := 0, 0, 0
		non := false
		for _, v := range s.word {
			if v.Accentuated {
				a++
			}
			if v.IsGlyph {
				g++
			}
			if v.Upper {
				u++
			}
			non = non || (v.Alpha && !v.IsASCII)
		}
		last := s.word[n-1]
		if n >= 4 {
			if float64(a)/float64(n) >= .5 {
				s.bad = true
			} else if last.Accentuated && last.Upper && u != n {
				s.foreign++
				s.bad = true
			} else if g == 1 {
				s.bad = true
				s.foreign++
			} else if non && s.word[0].Lower && u == n-1 {
				s.foreign++
				s.bad = true
			}
		}
		if n >= 24 && s.foreignWatch && !(u > 0 && float64(u)/float64(n) <= .3) {
			s.foreign++
			s.bad = true
		}
		if s.bad {
			s.badChars += n
		}
		if s.invalidWord {
			s.invalid++
		}
		s.word = nil
		s.wordInternalLigature = false
		s.wordLastLigature = false
		s.bad = false
		s.invalidWord = false
		s.foreignWatch = false
	} else if !containsRune("<>-=~|_", i.Character) && !i.Digit && i.Sym {
		s.bad = true
		s.word = append(s.word, i)
	}
}
func containsRune(s string, c rune) bool {
	for _, v := range s {
		if v == c {
			return true
		}
	}
	return false
}
func (s *messState) auFeed(i CharInfo) {
	sep := !(i.Alpha && i.CaseVariable)
	if sep && s.auChunk > 0 {
		if s.auChunk <= 64 && !i.Digit && !s.auASCII {
			s.auFinal += s.auSwitch
		}
		s.auSwitch = 0
		s.auChunk = 0
		s.auBuf = false
		s.auN++
		s.auASCII = true
		return
	}
	if !i.IsASCII {
		s.auASCII = false
	}
	if s.auChunk > 0 {
		if i.Upper && s.auPrev.Lower || i.Lower && s.auPrev.Upper {
			if s.auBuf {
				s.auSwitch += 2
				s.auBuf = false
			} else {
				s.auBuf = true
			}
		} else {
			s.auBuf = false
		}
	}
	s.auN++
	s.auChunk++
	s.auPrev = i
}
func (s *messState) feed(i CharInfo, ascii bool) {
	s.up++
	if i.Character == 27 {
		s.escape = true
	}
	if !i.Printable && !i.Space && i.Character != 26 && i.Character != 0xfeff {
		s.unprint++
	}
	s.wordFeed(i)
	if !ascii {
		s.auFeed(i)
	}
	if i.Printable {
		s.sp++
		if i.Character != s.last && !i.Safe {
			if i.Punct {
				s.punct++
			} else if !i.Digit && i.Sym && !i.Emoticon {
				s.symbols += 2
			}
		}
		s.last = i.Character
		if !ascii {
			s.sr++
			if i.Space || i.Punct || i.Safe {
				s.hasRange = false
			} else {
				if s.hasRange && (s.lastRange != i.Range || s.lastRange == "") && IsSuspiciouslySuccessiveRange(s.lastRange, i.Range) {
					s.susp++
				}
				s.hasRange = true
				s.lastRange = i.Range
			}
		}
	}
	if ascii || !i.Alpha {
		return
	}
	s.ta++
	if i.Accentuated {
		s.acc++
	}
	if i.Latin {
		s.dupN++
		if s.hasPrev && i.Accentuated && s.prev.Accentuated {
			if i.Upper && s.prev.Upper {
				s.dup++
			}
			if i.Unaccented == s.prev.Unaccented {
				s.dup++
			}
		}
		s.prev = i
		s.hasPrev = true
	}
	if i.IsCJK {
		s.cjk++
		s.kc++
		if !i.CommonCJK {
			s.uncommon++
			s.ku++
		}
	} else if i.IsKatakana {
		s.kata++
		if i.IsHalfwidthKatakana {
			s.half++
		}
	}
	if i.IsArabic {
		s.arab++
		if i.Flags&(1<<8) != 0 {
			s.isol++
		}
	}
}
func fraction(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
func (s *messState) ratio() float64 {
	r := 0.
	p := fraction(s.punct+s.symbols, s.sp)
	if p >= .3 {
		r += p
	}
	if s.ta >= 8 {
		p = fraction(s.acc, s.ta)
		if p >= .35 {
			r += p
		}
	}
	if s.escape {
		r++
	} else {
		r += fraction(s.unprint*8, s.up)
	}
	r += fraction(s.dup*2, s.dupN)
	if s.sr > 13 {
		r += fraction(s.susp*2, s.sr)
	}
	if s.invalid > 0 {
		r++
	} else if s.words > 10 || s.foreign != 0 {
		r += fraction(s.badChars, s.wordChars)
	}
	if s.cjk >= 4 {
		p = fraction(s.uncommon, s.cjk)
		if p > .5 {
			r += p / 5
		}
	}
	if s.half >= 4 && s.half == s.kata && s.kc >= 3 && s.kc == s.ku {
		r++
	}
	r += fraction(s.auFinal, s.auN)
	if s.arab >= 8 {
		r += fraction(s.isol, s.arab)
	}
	return r
}

// MessRatio ports charset_normalizer.md.mess_ratio, including block early exit.
func MessRatio(text string, maximumThreshold float64) float64 {
	chars := PythonRunes(text)
	step := 128
	if len(chars) < 511 {
		step = 32
	} else if len(chars) < 1024 {
		step = 64
	}
	ascii := true
	for _, c := range chars {
		if c >= 128 {
			ascii = false
			break
		}
	}
	s := messState{auASCII: true, last: -1}
	stopped := false
	for start := 0; start < len(chars); start += step {
		end := start + step
		if end > len(chars) {
			end = len(chars)
		}
		for _, c := range chars[start:end] {
			s.feed(CharInfoFor(c), ascii)
		}
		if s.ratio() >= maximumThreshold {
			stopped = true
			break
		}
	}
	if !stopped {
		i := CharInfoFor('\n')
		s.wordFeed(i)
		if !ascii {
			s.auFeed(i)
		}
		s.up++
	}
	return pythonRound(s.ratio(), 3)
}
