package charset

import (
	"sort"
	"strings"
)

type CoherenceMatch struct {
	Language string
	Ratio    float64
}

func EncodingLanguages(name string) []string {
	return append([]string{}, encodingLanguageProfiles[name]...)
}
func MBEncodingLanguages(name string) []string {
	switch {
	case strings.HasPrefix(name, "shift_"), strings.HasPrefix(name, "iso2022_jp"), strings.HasPrefix(name, "euc_j"), name == "cp932":
		return []string{"Japanese"}
	case strings.HasPrefix(name, "gb"), name == "big5", name == "cp950", name == "big5hkscs", name == "hz":
		return []string{"Chinese"}
	case strings.HasPrefix(name, "iso2022_kr"), name == "johab", name == "cp949", name == "euc_kr":
		return []string{"Korean"}
	}
	return []string{}
}
func coherenceRound(v float64) float64 { return pythonRound(v, 4) }
func sortCoherence(v []CoherenceMatch) {
	sort.SliceStable(v, func(i, j int) bool { return v[i].Ratio > v[j].Ratio })
}
func MergeCoherenceRatios(results [][]CoherenceMatch) []CoherenceMatch {
	out := []CoherenceMatch{}
	indices := map[string]int{}
	counts := []int{}
	for _, result := range results {
		for _, r := range result {
			i, ok := indices[r.Language]
			if !ok {
				i = len(out)
				indices[r.Language] = i
				out = append(out, CoherenceMatch{Language: r.Language})
				counts = append(counts, 0)
			}
			out[i].Ratio += r.Ratio
			counts[i]++
		}
	}
	for i := range out {
		out[i].Ratio = coherenceRound(out[i].Ratio / float64(counts[i]))
	}
	sortCoherence(out)
	return out
}
func alphaUnicodeSplit(text string) []string {
	keys := []string{}
	layers := [][]rune{}
	for _, r := range PythonRunes(text) {
		info := CharInfoFor(r)
		if !info.Alpha || info.Range == "" {
			continue
		}
		target := -1
		for i, key := range keys {
			if !IsSuspiciouslySuccessiveRange(key, info.Range) {
				target = i
				break
			}
		}
		if target < 0 {
			target = len(keys)
			keys = append(keys, info.Range)
			layers = append(layers, nil)
		}
		layers[target] = append(layers[target], r)
	}
	out := []string{}
	for _, layer := range layers {
		out = append(out, PythonLower(string(layer)))
	}
	return out
}
func alphabetLanguages(chars []rune, ignore bool) []string {
	sourceAccents := false
	set := map[rune]bool{}
	for _, r := range chars {
		set[r] = true
		if CharInfoFor(r).Accentuated {
			sourceAccents = true
		}
	}
	matches := []CoherenceMatch{}
	for _, p := range languageProfiles {
		accents, latin := false, true
		seen := map[rune]bool{}
		n := 0
		for _, r := range p.chars {
			n++
			info := CharInfoFor(r)
			accents = accents || info.Accentuated
			latin = latin && info.Latin
			seen[r] = true
		}
		if ignore && !latin || sourceAccents && !accents {
			continue
		}
		count := 0
		for r := range seen {
			if set[r] {
				count++
			}
		}
		ratio := float64(count) / float64(n)
		if ratio >= .2 {
			matches = append(matches, CoherenceMatch{p.name, ratio})
		}
	}
	sortCoherence(matches)
	out := []string{}
	for _, m := range matches {
		out = append(out, m.Language)
	}
	return out
}
func charactersPopularityCompare(language string, chars []rune) float64 {
	profile := ""
	for _, p := range languageProfiles {
		if p.name == language {
			profile = p.chars
			break
		}
	}
	if profile == "" || len(chars) == 0 {
		return 0
	}
	rank := map[rune]int{}
	target := []rune(profile)
	for i, r := range target {
		rank[r] = i
	}
	lr, orr := []int{}, []int{}
	for i, r := range chars {
		if j, ok := rank[r]; ok {
			lr = append(lr, j)
			orr = append(orr, i)
		}
	}
	approved := 0
	expectedProjectionRatio := float64(len(target)) / float64(len(chars))
	for i, l := range lr {
		o := orr[i]
		projection := int(float64(o) * expectedProjectionRatio)
		delta := projection - l
		if delta < 0 {
			delta = -delta
		}
		if len(target) <= 26 && delta > 4 {
			continue
		}
		if len(target) > 26 && float64(delta) < float64(len(target))/3 || l == 0 {
			approved++
			continue
		}
		before, after := 0, 0
		for j, ll := range lr {
			if ll < l {
				if orr[j] < o {
					before++
					if 5*before >= 2*l {
						approved++
						break
					}
				}
			} else if orr[j] >= o {
				after++
				if 5*after >= 2*(len(target)-l) {
					approved++
					break
				}
			}
		}
	}
	return float64(approved) / float64(len(chars))
}
func CoherenceRatio(text string, threshold float64, inclusion []string) []CoherenceMatch {
	langs := []string{}
	ignore := false
	removed := false
	for _, l := range inclusion {
		if l == "Latin Based" && !removed {
			ignore = true
			removed = true
		} else {
			langs = append(langs, l)
		}
	}
	results := []CoherenceMatch{}
	sufficient := 0
	for _, layer := range alphaUnicodeSplit(text) {
		chars := []rune{}
		counts := map[rune]int{}
		n := 0
		for _, r := range PythonRunes(layer) {
			n++
			if counts[r] == 0 {
				chars = append(chars, r)
			}
			counts[r]++
		}
		if n <= 32 {
			continue
		}
		sort.SliceStable(chars, func(i, j int) bool { return counts[chars[i]] > counts[chars[j]] })
		candidates := langs
		if len(candidates) == 0 {
			candidates = alphabetLanguages(chars, ignore)
		}
		for _, l := range candidates {
			ratio := charactersPopularityCompare(l, chars)
			if ratio < threshold {
				continue
			}
			if ratio >= .8 {
				sufficient++
			}
			results = append(results, CoherenceMatch{l, coherenceRound(ratio)})
			if sufficient >= 3 {
				break
			}
		}
	}
	indices := map[string]int{}
	filtered := []CoherenceMatch{}
	duplicate := false
	for _, r := range results {
		name := strings.ReplaceAll(r.Language, "—", "")
		if i, ok := indices[name]; ok {
			duplicate = true
			if r.Ratio > filtered[i].Ratio {
				filtered[i].Ratio = r.Ratio
			}
		} else {
			indices[name] = len(filtered)
			filtered = append(filtered, CoherenceMatch{name, r.Ratio})
		}
	}
	if duplicate {
		results = filtered
	}
	sortCoherence(results)
	return results
}
