package pan115

import (
	"encoding/json"
	"testing"
)

func TestLifeSizePreservesJSONTruthiness(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want bool
	}{
		{`null`, false}, {`false`, false}, {`0`, false}, {`0.0`, false}, {`""`, false}, {`"0"`, true}, {`1`, true}, {`true`, true}, {`[]`, false}, {`{}`, false},
	} {
		if got := lifeJSONTruthy(json.RawMessage(test.raw)); got != test.want {
			t.Errorf("%s: got %v want %v", test.raw, got, test.want)
		}
	}
}
