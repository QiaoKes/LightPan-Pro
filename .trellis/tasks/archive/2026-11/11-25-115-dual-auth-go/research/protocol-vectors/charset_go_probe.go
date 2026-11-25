// Offline public-API oracle comparison driver; not production code.
package main

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	cs "litepan/drivers/115/client/charset"
	"math"
	"os"
)

type Case struct {
	ID         string   `json:"id"`
	Raw        string   `json:"raw_base64"`
	Encoding   *string  `json:"encoding"`
	Language   string   `json:"language"`
	Confidence *float64 `json:"confidence"`
	Text       *string  `json:"decoded_text"`
}

func main() {
	f, e := os.Open(os.Args[1])
	if e != nil {
		panic(e)
	}
	z, e := gzip.NewReader(f)
	if e != nil {
		panic(e)
	}
	var corpus struct{ Cases []Case }
	if e = json.NewDecoder(z).Decode(&corpus); e != nil {
		panic(e)
	}
	diff := []map[string]any{}
	for _, c := range corpus.Cases {
		raw, _ := base64.StdEncoding.DecodeString(c.Raw)
		got := cs.Detect(raw)
		text, err := cs.Decode(raw)
		bad := got.Found != (c.Encoding != nil) || c.Encoding != nil && *c.Encoding != got.Encoding || got.Language != c.Language || c.Confidence != nil && math.Abs(got.Confidence-*c.Confidence) > 1e-12 || c.Text != nil && text != *c.Text || err != nil
		if bad {
			diff = append(diff, map[string]any{"id": c.ID, "raw_base64": c.Raw, "want_encoding": c.Encoding, "want_language": c.Language, "want_confidence": c.Confidence, "want_text": c.Text, "got": got, "got_text": text, "error": fmt.Sprint(err)})
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"count": len(corpus.Cases), "mismatch_count": len(diff), "differences": diff})
}
