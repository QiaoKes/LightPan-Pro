package pan115client

import (
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"reflect"
	"testing"
)

func TestParseDirectoryTreeLeaves(t *testing.T) {
	raw := []byte("|——root\n| |-电影\n| | |-中文 100%.mkv\n| |-one.mp4")
	got, err := ParseDirectoryTree(raw, "/")
	want := []string{"/", "/电影/中文 100%.mkv", "/one.mp4"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v %v", got, err)
	}
}

func TestParseDirectoryTreeInvalidDepthDoesNotPanic(t *testing.T) {
	got, err := ParseDirectoryTree([]byte("|——root"), "/scope")
	if err != nil || len(got) != 1 || got[0] != "/scope/" {
		t.Fatal(got, err)
	}
}

func TestParseTreeChineseEncoding(t *testing.T) {
	text := "|——root\n| |-中文.mkv"
	// Pinned Python 3.5.1 chooses Big5 for this tiny GB18030 sample.
	// Source encoding alone is not the reference detector's result.
	wants := [][]string{{"/", "/笢恅.mkv"}, {"/", "/中文.mkv"}}
	for i, encoder := range []interface{ Bytes([]byte) ([]byte, error) }{simplifiedchinese.GB18030.NewEncoder(), unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder()} {
		raw, err := encoder.Bytes([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseDirectoryTree(raw, "/")
		if err != nil || !reflect.DeepEqual(got, wants[i]) {
			t.Fatal(got, wants[i], err)
		}
	}
}
