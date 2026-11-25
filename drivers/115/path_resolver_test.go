package pan115

import (
	"context"
	"net/http"
	"testing"

	"litepan/internal/domain"
)

func TestResolveTreePathWithoutHTTP(t *testing.T) {
	d := &Driver{client: &http.Client{Transport: fixtureTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("path adapter performed metadata HTTP")
		return nil, nil
	})}}
	for _, tc := range []struct{ root, relative, want string }{
		{"/", "影片/电影.mkv", "/影片/电影.mkv"},
		{"/媒体", "剧集/第一集.mkv", "/媒体/剧集/第一集.mkv"},
		{"/媒体/", "", "/媒体"},
	} {
		item, err := d.ResolvePath(context.Background(), tc.root, tc.relative)
		if err != nil {
			t.Fatal(err)
		}
		if item.ID != tc.want || item.IDKind != domain.IDPath {
			t.Fatalf("item=%+v", item)
		}
	}
	if _, err := d.ResolvePath(context.Background(), "123", "file.mkv"); err == nil {
		t.Fatal("invented numeric root mapping")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.ResolvePath(ctx, "/", "file.mkv"); err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}
