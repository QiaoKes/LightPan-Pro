package pan115client

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Expected text comes from the pinned native Python oracle; expected leaves
// apply the reference stack parser to that text offline.
func TestTreeCharsetOracleLeaves(t *testing.T) {
	raw, err := os.ReadFile("testdata/tree-charset-leaves.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID     string   `json:"id"`
		Raw    string   `json:"raw_base64"`
		Leaves []string `json:"leaves"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			raw, err := base64.StdEncoding.DecodeString(c.Raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseDirectoryTree(raw, "/")
			if err != nil || !reflect.DeepEqual(got, c.Leaves) {
				t.Fatalf("got %q (%v), want %q", got, err, c.Leaves)
			}
		})
	}
}
