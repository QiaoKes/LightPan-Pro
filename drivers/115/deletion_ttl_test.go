package pan115

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"litepan/internal/driver"
)

func TestMediaLinkUsesReference1200SecondTTL(t *testing.T) {
	d := &Driver{token: "access", pickBy: map[string]string{"123": "pick"}}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != pathDownload {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"123":{"file_name":"movie.mkv","file_size":100,"url":{"url":"https://cdn.example/movie"}}}}`))}, nil
	})}
	link, err := d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: "123", UA: "player"})
	if err != nil {
		t.Fatal(err)
	}
	if link.Expiration != 1200*time.Second {
		t.Fatalf("TTL=%v", link.Expiration)
	}
}
func TestDeleteFilesTrashesOnlyRequestedIDs(t *testing.T) {
	d := &Driver{token: "access"}
	calls := 0
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != pathDelete {
			t.Fatalf("unsafe recycle request %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("file_ids") != "123,456" {
			t.Fatal("wrong source IDs")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{}}`))}, nil
	})}
	err := d.DeleteFiles(context.Background(), []string{"123", "456"})
	if err != nil {
		t.Fatalf("trash failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestAdditionExposesNoDeleteMode(t *testing.T) {
	typ := reflect.TypeOf(Addition{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Tag.Get("json") == "delete_mode" {
			t.Fatal("obsolete delete mode exposed")
		}
	}
}
