package charset

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestPythonRoundReference(t *testing.T) {
	raw, err := os.ReadFile("rounding_vectors_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Input    string
		Digits   int
		Expected string
	}
	if err = json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for i, v := range vectors {
		input, err := strconv.ParseUint(v.Input, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := strconv.ParseUint(v.Expected, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		got := pythonRound(math.Float64frombits(input), v.Digits)
		if math.Float64bits(got) != expected {
			t.Fatalf("case %d round(%x,%d) got %x want %x", i, input, v.Digits, math.Float64bits(got), expected)
		}
	}
}
