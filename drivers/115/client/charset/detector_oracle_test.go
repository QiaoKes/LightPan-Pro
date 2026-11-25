package charset

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestDetectionOracle(t *testing.T) { testDetectionOracle(t, "testdata/detection-oracle.json.gz") }
func TestDetectionStressOracle(t *testing.T) {
	testDetectionOracle(t, "testdata/detection-stress-oracle.json.gz")
}
func TestDetectionLargeOracle(t *testing.T) {
	testDetectionOracle(t, "testdata/detection-large-oracle.json.gz")
}
func testDetectionOracle(t *testing.T, path string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var corpus struct {
		Cases []struct {
			ID         string          `json:"id"`
			Raw        string          `json:"raw_base64"`
			Encoding   *string         `json:"encoding"`
			Language   string          `json:"language"`
			Confidence *float64        `json:"confidence"`
			Text       *string         `json:"decoded_text"`
			Exception  json.RawMessage `json:"exception"`
		}
	}
	if err = json.NewDecoder(z).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	expectedCount := 335
	if path == "testdata/detection-large-oracle.json.gz" {
		expectedCount = 5
	}
	if path == "testdata/detection-stress-oracle.json.gz" {
		expectedCount = 2350
	}
	if len(corpus.Cases) != expectedCount {
		t.Fatalf("oracle cases got%d want%d", len(corpus.Cases), expectedCount)
	}
	for _, c := range corpus.Cases {
		t.Run(c.ID, func(t *testing.T) {
			if string(c.Exception) != "null" {
				t.Fatalf("unexpected oracle exception %s", c.Exception)
			}
			raw, err := base64.StdEncoding.DecodeString(c.Raw)
			if err != nil {
				t.Fatal(err)
			}
			wantEncoding := "<none>"
			if c.Encoding != nil {
				wantEncoding = *c.Encoding
			}
			got := Detect(raw)
			if (c.Encoding != nil) != got.Found || c.Encoding != nil && *c.Encoding != got.Encoding {
				t.Errorf("encoding got %q (%v), want %v", got.Encoding, got.Found, wantEncoding)
			}
			if got.Language != c.Language {
				t.Errorf("language got %q want %q", got.Language, c.Language)
			}
			if c.Confidence != nil && math.Abs(got.Confidence-*c.Confidence) > 1e-12 {
				t.Errorf("confidence got %.15g want %.15g", got.Confidence, *c.Confidence)
			}
			if c.Text != nil {
				text, err := Decode(raw)
				if err != nil {
					t.Fatal(err)
				}
				if text != *c.Text {
					t.Errorf("decoded text differs (got %d bytes want %d)", len(text), len(*c.Text))
				}
			}
		})
	}
}
