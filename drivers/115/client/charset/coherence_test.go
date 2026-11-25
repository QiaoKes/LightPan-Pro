package charset

import (
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestCoherenceReference(t *testing.T) {
	raw, err := os.ReadFile("coherence_vectors_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text      string
		Inclusion []string
		Expected  []CoherenceMatch
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func(i int, c struct {
			Text      string
			Inclusion []string
			Expected  []CoherenceMatch
		}) {
			defer wg.Done()
			got := CoherenceRatio(c.Text, .1, c.Inclusion)
			if !reflect.DeepEqual(got, c.Expected) {
				t.Errorf("case %d got %v want %v", i, got, c.Expected)
			}
		}(i, c)
	}
	wg.Wait()
}
func TestMergeCoherenceStable(t *testing.T) {
	got := MergeCoherenceRatios([][]CoherenceMatch{{{"French", .8}, {"English", .6}}, {{"English", 1}, {"French", .8}}})
	want := []CoherenceMatch{{"French", .8}, {"English", .8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestPopularityProjectionOrderReference(t *testing.T) {
	raw, err := os.ReadFile("popularity_vectors_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Language, Characters string
		Expected             float64
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("missing arithmetic boundary cases")
	}
	for _, c := range cases {
		if got := charactersPopularityCompare(c.Language, []rune(c.Characters)); got != c.Expected {
			t.Errorf("%s got %.17g want %.17g", c.Language, got, c.Expected)
		}
	}
}
