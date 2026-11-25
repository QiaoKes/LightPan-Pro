package charset

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPythonModelSortFullOrder(t *testing.T) {
	f, err := os.Open("model_sort_vectors_test.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct {
		Raw   int
		Rows  [][]float64
		Order []int
	}
	if err = json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for ci, c := range cases {
		a := make([]*charsetMatch, len(c.Rows))
		ids := map[*charsetMatch]int{}
		for i, row := range c.Rows {
			a[i] = &charsetMatch{chaos: row[0], languages: []CoherenceMatch{{Language: "English", Ratio: row[1]}}, text: strings.Repeat("x", int(row[2])), rawLength: c.Raw}
			ids[a[i]] = i
		}
		pythonMatchSort(a)
		for i, want := range c.Order {
			if ids[a[i]] != want {
				t.Fatalf("case %d count %d position %d got %d want %d", ci, len(a), i, ids[a[i]], want)
			}
		}
	}
}
