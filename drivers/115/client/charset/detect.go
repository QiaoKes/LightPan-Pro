// Package charset ports the default charset-normalizer 3.5.1 detection flow.
package charset

import (
	"litepan/drivers/115/client/charset/codecs"
	"strings"
)

type Detection struct {
	Encoding, Language string
	Confidence         float64
	Found              bool
}

func Detect(raw []byte) Detection {
	match := detectMatch(raw)
	if match == nil {
		return Detection{}
	}
	encoding := match.encoding
	confidence := 1 - match.chaos
	if confidence >= .9 && encoding != "utf_8" && encoding != "ascii" && !match.bom && len(raw) < 32 {
		confidence -= .2
	}
	if encoding == "utf_8" && match.bom {
		encoding = "utf_8_sig"
	}
	if legacy, ok := legacyCorrespondence[encoding]; ok {
		encoding = legacy
	}
	language := match.language()
	if language == "Unknown" {
		language = ""
	}
	return Detection{encoding, language, confidence, true}
}

// Decode follows the reference text-open operation, rather than normalization
// output(): declarations remain intact, errors are ignored and newlines universal.
func Decode(raw []byte) (string, error) {
	detected := Detect(raw)
	name := detected.Encoding
	if !detected.Found {
		name = "utf_8"
	}
	text, err := codecs.Decode(raw, name, true)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), nil
}
func detectMatch(raw []byte) *charsetMatch {
	length := len(raw)
	if length == 0 {
		return &charsetMatch{encoding: "utf_8"}
	}
	steps, size := 5, 512
	if length <= steps*size {
		steps = 1
		size = length
	}
	if steps > 1 && length/steps < size {
		size = length / steps
	}
	large := length >= 10000000
	specified := specifiedEncoding(raw)
	sig, mark := signature(raw)
	priority := []string{}
	if specified != "" {
		priority = append(priority, specified)
	}
	if sig != "" {
		priority = append([]string{sig}, priority...)
	}
	priority = append(priority, "ascii")
	hasUTF := false
	for _, e := range priority {
		if e == "utf_8" {
			hasUTF = true
		}
	}
	if !hasUTF {
		priority = append(priority, "utf_8")
	}
	candidates := append([]string{}, priority...)
	for _, multi := range []bool{true, false} {
		for _, e := range detectorCandidates {
			if codecs.IsMultiByte(e) == multi {
				candidates = append(candidates, e)
			}
		}
	}
	tested, skip := map[string]bool{}, map[string]bool{}
	definitive, mbDefinitive := false, false
	targetSet := map[string]bool{}
	postCount := 0
	var fallbackASCII, fallbackUTF, fallbackSpecified *charsetMatch
	results, early := charsetMatches{}, charsetMatches{}
	messCache := map[string]float64{}
	for _, name := range candidates {
		if tested[name] {
			continue
		}
		tested[name] = true
		bom := sig == name
		strip := bom && name != "utf_16" && name != "utf_32"
		if (name == "utf_16" || name == "utf_32" || name == "utf_7") && !bom {
			continue
		}
		if skip[name] {
			continue
		}
		multi := codecs.IsMultiByte(name)
		targets := EncodingLanguages(name)
		if multi {
			targets = MBEncodingLanguages(name)
		}
		if definitive {
			intersects := false
			for _, t := range targets {
				if targetSet[t] {
					intersects = true
				}
			}
			if !intersects {
				continue
			}
		}
		if definitive && !multi && postCount >= 7 {
			continue
		}
		if mbDefinitive && !multi {
			continue
		}
		deferred := !multi && !large
		base := raw
		if strip {
			base = raw[len(mark):]
		}
		var decoded *string
		var err error
		if large && !multi {
			_, err = codecs.Decode(raw[:min(500000, length)], name, false)
			if strip {
				_, err = codecs.Decode(raw[len(mark):min(500000, length)], name, false)
			}
		} else if !deferred {
			var text string
			if name == "utf_7" && bom {
				text, err = codecs.Decode(raw, name, false)
				text = strings.TrimPrefix(text, "\ufeff")
			} else {
				text, err = codecs.Decode(base, name, false)
			}
			decoded = &text
		}
		if err != nil {
			continue
		}
		offsets := []int{}
		start := 0
		if bom {
			start = len(mark)
		}
		for i := start; i < length; i += length / steps {
			offsets = append(offsets, i)
		}
		maxGiveUp := max(len(offsets)/4, 2)
		giveUp := 0
		sum := 0.
		chunks := []string{}
		err = sequenceChunks(raw, name, offsets, size, bom, strip, mark, multi, decoded, deferred, func(chunk string) bool {
			chunks = append(chunks, chunk)
			ratio, ok := messCache[chunk]
			if !ok {
				ratio = MessRatio(chunk, .2)
				messCache[chunk] = ratio
			}
			sum += ratio
			if ratio >= .2 {
				giveUp++
			}
			return giveUp < maxGiveUp && !(bom && !strip)
		})
		lazyFailure := err != nil
		if lazyFailure && deferred {
			continue
		}
		if lazyFailure {
			giveUp = maxGiveUp
		}
		mean := 0.
		if len(chunks) > 0 {
			mean = sum / float64(len(chunks))
		}
		if !lazyFailure && large && !multi && mean < .2 && giveUp < maxGiveUp {
			if _, err = codecs.Decode(raw[min(50000, length):], name, false); err != nil {
				continue
			}
		}
		if mean >= .2 || giveUp >= maxGiveUp {
			for _, similar := range detectorSimilar[name] {
				skip[similar] = true
			}
			if !lazyFailure && (name == "ascii" || name == "utf_8" || name == specified || name == "utf_16" || name == "utf_32") {
				text := ""
				if decoded != nil {
					text = *decoded
				} else {
					text, err = codecs.Decode(base, name, false)
				}
				if err != nil {
					continue
				}
				entry := &charsetMatch{encoding: name, chaos: .2, bom: bom, text: text, rawLength: length}
				if large {
					entry.text = ""
				}
				if name == specified {
					fallbackSpecified = entry
				} else if name == "ascii" {
					fallbackASCII = entry
				} else {
					fallbackUTF = entry
				}
			}
			continue
		}
		if deferred {
			text, e := codecs.Decode(base, name, false)
			if e != nil {
				continue
			}
			decoded = &text
		}
		ratios := [][]CoherenceMatch{}
		if name != "ascii" {
			for _, chunk := range chunks {
				ratios = append(ratios, CoherenceRatio(chunk, .1, targets))
			}
		}
		merged := MergeCoherenceRatios(ratios)
		text := ""
		if decoded != nil {
			text = *decoded
		}
		// A nil decoded payload for a large single-byte candidate stays lazy.
		// Its prefix and remaining bytes were strictly validated above; models
		// disables text-based deduplication and usage ranking at this size.
		current := &charsetMatch{encoding: name, chaos: mean, bom: bom, languages: merged, text: text, rawLength: length}
		if large && name != specified && name != "ascii" && name != "utf_8" {
			current.text = ""
		}
		results.append(current)
		if definitive && !multi && mean < .02 {
			postCount++
		}
		if (name == specified || name == "ascii" || name == "utf_8") && mean < .1 {
			if mean == 0 {
				return current
			}
			early.append(current)
		}
		if len(early.items) > 0 && (specified == "" || tested[specified]) && tested["ascii"] && tested["utf_8"] {
			return early.best()
		}
		if !definitive && !multi {
			best := 0.
			for _, r := range merged {
				best = max(best, r.Ratio)
			}
			if best >= .5 && tested["ascii"] && tested["utf_8"] {
				definitive = true
				for _, t := range targets {
					targetSet[t] = true
				}
			}
		}
		if !mbDefinitive && multi && decoded != nil && PythonRuneCount(*decoded) < length && float64(PythonRuneCount(*decoded)) < float64(length)*.98 && !strings.HasPrefix(name, "utf_") && tested["ascii"] && tested["utf_8"] {
			mbDefinitive = true
		}
		if name == sig {
			for _, m := range results.items {
				if m.encoding == name {
					return m
				}
				for _, leaf := range m.leaves {
					if leaf.encoding == name {
						return m
					}
				}
			}
		}
	}
	if len(results.items) == 0 {
		if fallbackSpecified != nil {
			results.append(fallbackSpecified)
		} else if fallbackUTF != nil {
			results.append(fallbackUTF)
		} else if fallbackASCII != nil {
			results.append(fallbackASCII)
		}
	}
	return results.best()
}
