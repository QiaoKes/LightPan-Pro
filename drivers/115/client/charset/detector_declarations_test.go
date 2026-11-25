package charset

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPinnedDeclarations(t *testing.T) {
	raw, err := os.ReadFile("testdata/declarations.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Raw       string  `json:"raw_base64"`
		Specified *string `json:"specified"`
		Detection *struct {
			Encoding   *string  `json:"encoding"`
			Language   string   `json:"language"`
			Confidence *float64 `json:"confidence"`
		} `json:"detection"`
		Exception *string `json:"exception"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		raw, _ := base64.StdEncoding.DecodeString(c.Raw)
		want := ""
		if c.Specified != nil {
			want = *c.Specified
		}
		if got := specifiedEncoding(raw); got != want {
			t.Errorf("case%d declaration got%q want%q", i, got, want)
		}
		if c.Exception != nil {
			t.Fatalf("unexpected oracle exception case%d: %s", i, *c.Exception)
		}
		got := Detect(raw)
		expected := ""
		if c.Detection.Encoding != nil {
			expected = *c.Detection.Encoding
		}
		if got.Encoding != expected || got.Language != c.Detection.Language || c.Detection.Confidence != nil && math.Abs(got.Confidence-*c.Detection.Confidence) > 1e-12 {
			t.Errorf("case%d detector got%+v want%+v", i, got, c.Detection)
		}
	}
}
func TestDefaultCandidateBound(t *testing.T) {
	set := map[string]bool{}
	for _, name := range detectorCandidates {
		set[name] = true
	}
	if len(set) != 98 {
		t.Fatal(len(set))
	}
	for _, mark := range detectorMarks {
		if !set[mark.name] {
			t.Fatalf("signature %s adds candidate", mark.name)
		}
	}
	// ascii/utf8 priorities and all signatures already belong to98 candidates;
	// a declaration can add at most one distinct name, so default bound is99.
	if !set["ascii"] || !set["utf_8"] {
		t.Fatal("priorities outside inventory")
	}
}
