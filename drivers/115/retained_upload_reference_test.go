package pan115

import (
	"context"
	"io"
	"litepan/internal/driver"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetainedPublicUploadInstantAndSinglePUT(t *testing.T) {
	for _, instant := range []bool{true, false} {
		t.Run(map[bool]string{true: "instant", false: "single"}[instant], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hello.txt")
			if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
				t.Fatal(err)
			}
			var trace []string
			d := referenceWireDriver(t, &trace, func(r *http.Request) (int, string) {
				switch r.URL.Path {
				case pathList:
					return 200, `{"state":true,"count":0,"data":[]}`
				case pathUploadInit:
					if r.Form.Get("fileid") != "AAF4C61DDCC5E8A2DABEDE0F3B482CD9AEA9434D" || r.Form.Get("preid") != r.Form.Get("fileid") || r.Form.Get("file_size") != "5" || r.Form.Get("file_name") != "hello.txt" || r.Form.Get("target") != "U_1_9" {
						t.Errorf("init=%v", r.Form)
					}
					if instant {
						return 200, `{"state":true,"data":{"status":2,"file_id":"10"}}`
					}
					return 200, `{"state":true,"data":{"status":1,"bucket":"bucket","object":"hello.txt"}}`
				case pathUploadToken:
					return 200, `{"state":true,"data":{"access_key_id":"fixture","access_key_secret":"fixture","security_token":"fixture","endpoint":"https://oss.example.invalid"}}`
				case pathFileInfo:
					return 200, `{"state":true,"data":{"file_id":"10","file_name":"hello.txt","size":"5"}}`
				default:
					t.Fatalf("unexpected %s", r.URL)
					return 500, ""
				}
			})
			puts := 0
			d.uploadClient = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
				puts++
				b, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if r.Method != "PUT" || string(b) != "hello" || r.ContentLength != 5 || !strings.HasPrefix(r.Header.Get("Authorization"), "OSS fixture:") || r.Header.Get("x-oss-security-token") != "fixture" {
					t.Errorf("PUT=%s %s headers=%v body=%s", r.Method, r.URL, r.Header, b)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"file_id":"10"}}`))}, nil
			})}
			var state map[string]any
			result, err := d.UploadLocalFile(context.Background(), driver.LocalUploadRequest{LocalPath: path, FileName: "hello.txt", ParentID: "9", OnResumeState: func(v map[string]any) { state = v }})
			if err != nil || result == nil || result.FileID != "10" || result.Size != 5 {
				t.Fatalf("result=%+v err=%v trace=%v", result, err, trace)
			}
			if state["file_sha1"] != "AAF4C61DDCC5E8A2DABEDE0F3B482CD9AEA9434D" {
				t.Fatalf("state=%v", state)
			}
			if (instant && puts != 0) || (!instant && puts != 1) {
				t.Fatalf("puts=%d", puts)
			}
		})
	}
}
