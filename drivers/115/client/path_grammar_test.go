package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestMonitorPathEscapingReferenceVectors(t *testing.T) {
	for _, test := range []struct{ raw, escaped string }{
		{"movie.mkv", "movie.mkv"}, {"a/b", `a\/b`}, {`a\b`, `a\\b`}, {".", `\.`}, {"..", `\..`}, {`a\/b`, `a\\\/b`},
	} {
		if got := escapeMonitorName(test.raw); got != test.escaped {
			t.Errorf("escape %q: %q", test.raw, got)
		}
		if got := unescapeMonitorName(test.escaped); got != test.raw {
			t.Errorf("unescape %q: %q", test.escaped, got)
		}
	}
	for _, test := range []struct {
		input string
		parts []string
	}{
		{`/base/a\/b/file.nfo`, []string{"base", "a/b", "file.nfo"}},
		{`/base/a\\/b/file.nfo`, []string{"base", `a\`, "b", "file.nfo"}},
		{`/base/a\\\/b/file.nfo`, []string{"base", `a\/b`, "file.nfo"}},
		{`/base/\./file.nfo`, []string{"base", ".", "file.nfo"}},
		{`/base/\../file.nfo`, []string{"base", "..", "file.nfo"}},
		{`/base/a\\b/file.nfo`, []string{"base", `a\b`, "file.nfo"}},
		{`/base/../file.nfo`, []string{"file.nfo"}},
		{`/base/a\q/file.nfo`, []string{"base", `a\q`, "file.nfo"}},
	} {
		if got := splitMonitorPath(test.input); !reflect.DeepEqual(got, test.parts) {
			t.Errorf("split %q: %q want %q", test.input, got, test.parts)
		}
	}
}

func TestEscapedAttachmentPathExactRequestTraces(t *testing.T) {
	for _, test := range []struct {
		name, remote string
		probes       []string
		directory    bool
	}{
		{"slash-parent", `/base/a\/b/file.nfo`, []string{"base"}, true},
		{"slash-root", `/a\/b/file.nfo`, nil, true},
		{"literal-dot", `/base/\./file.nfo`, []string{"base/./file.nfo", "base/."}, false},
		{"literal-backslash", `/base/a\\b/file.nfo`, []string{`base/a\b/file.nfo`, `base/a\b`}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			attachmentOriginSequence.Store(0)
			var probes []string
			lists := 0
			c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				q := r.URL.Query()
				body := ""
				switch r.URL.Path {
				case "/files/getid":
					probes = append(probes, q.Get("path"))
					id := "10"
					if len(test.probes) > 1 && len(probes) == 1 {
						id = "0"
					}
					body = `{"state":true,"id":"` + id + `"}`
				case "/files":
					lists++
					if test.directory && lists == 1 {
						cid := "10"
						if len(test.probes) == 0 {
							cid = "0"
						}
						if q.Get("nf") != "1" || q.Get("cid") != cid {
							return nil, &HTTPError{StatusCode: 400}
						}
						body = `{"state":true,"count":1,"data":[{"cid":"20","n":"a/b"}]}`
					} else {
						cid := "10"
						if test.directory {
							cid = "20"
						}
						if q.Has("nf") || q.Get("cid") != cid {
							return nil, &HTTPError{StatusCode: 400}
						}
						body = `{"state":true,"count":1,"data":[{"fid":"30","n":"file.nfo"}]}`
					}
				default:
					return nil, &HTTPError{StatusCode: 400}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}, "UID=1")
			id, err := c.AttachmentID(context.Background(), test.remote)
			expectedLists := 1
			if test.directory {
				expectedLists++
			}
			if err != nil || id != "30" || !reflect.DeepEqual(probes, test.probes) || lists != expectedLists {
				t.Fatal(id, err, probes, lists)
			}
		})
	}
}

func TestFolderPathEscapesRawCacheSeparately(t *testing.T) {
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/android/folder/update" {
			return nil, fmt.Errorf("extra probe")
		}
		body := map[string]any{"state": true, "data": map[string]any{"file_id": "3", "file_name": `a\b`, "paths": []map[string]any{{"file_id": 0, "file_name": ""}, {"file_id": "1", "file_name": "a/b"}, {"file_id": "2", "file_name": ".."}}}}
		raw, _ := json.Marshal(body)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}, "UID=1")
	got, err := c.FolderPath(context.Background(), "3", "escape-cache-fixture")
	if err != nil || got != `/a\/b/\../a\\b` {
		t.Fatal(got, err)
	}
	if raw, ok := cachedDirectoryPath("escape-cache-fixture", "3", false); !ok || raw != `/a/b/../a\b` {
		t.Fatal(raw, ok)
	}
	if escaped, ok := cachedDirectoryPath("escape-cache-fixture", "3", true); !ok || escaped != got {
		t.Fatal(escaped, ok)
	}
}

func TestDirectorySkimEscapesParentsAndLeaf(t *testing.T) {
	const uid = "skim-escape-fixture"
	stablePoints.Store(uid, "0123")
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var raw []byte
		switch r.URL.Path {
		case "/android/folder/update":
			raw = []byte(`{"state":true,"data":{"file_id":"42","file_name":"a/b","paths":[{"file_id":0,"file_name":""}]}}`)
		case "/os_windows/2.0/ufile/downfolders":
			body := `{"state":true,"data":{"list":[],"has_next_page":false}}`
			if r.URL.Query().Get("page") == "1" {
				body = `{"state":true,"data":{"list":[{"fid":"100","pid":"42","fn":"."}],"has_next_page":false}}`
			}
			raw = encryptedFixture(body)
		case "/os_windows/2.0/ufile/downfiles":
			body := `{"state":true,"data":{"list":[],"has_next_page":false}}`
			if r.URL.Query().Get("page") == "1" {
				body = `{"state":true,"data":{"list":[{"pc":"at6h4t9","pid":"100","fs":12}],"has_next_page":false}}`
			}
			raw = encryptedFixture(body)
		case "/files/file":
			raw = []byte(`{"state":true,"data":[{"file_id":"101","file_name":"a\\b/file.nfo"}]}`)
		default:
			return nil, &HTTPError{StatusCode: 400}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})}, "UID=1")
	files, err := c.DirectoryEventFiles(context.Background(), "42", uid)
	if path, ok := cachedDirectoryPath(uid, "100", true); !ok || path != `/a\/b/\.` {
		t.Fatal(path, ok)
	}
	if err != nil || len(files) != 1 || files[0].Path != `/a\/b/\./a\\b\/file.nfo` || files[0].Name != `a\b/file.nfo` {
		t.Fatal(files, err)
	}
}
