package pan115

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAbsoluteTreeRootResolvesMountOnce(t *testing.T) {
	d := &Driver{}
	calls := 0
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("file_id") != "42" {
			t.Fatal("incorrect mount lookup")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"file_id":"42","file_name":"挂载","paths":[{"file_id":"0","file_name":"根目录"},{"file_id":"7","file_name":"媒体"}]}}`))}, nil
	})}
	root, err := d.absoluteTreeRoot(context.Background(), "剧集")
	if err != nil || root != "/剧集" || calls != 0 {
		t.Fatalf("root=%q err=%v calls=%d", root, err, calls)
	}
	d.add.RootFolderID = "42"
	root, err = d.absoluteTreeRoot(context.Background(), "剧集")
	if err != nil || root != "/媒体/挂载/剧集" || calls != 1 {
		t.Fatalf("root=%q err=%v calls=%d", root, err, calls)
	}
}
