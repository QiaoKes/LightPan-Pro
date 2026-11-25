package pan115

import (
	"context"
	"io"
	"litepan/internal/driver"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func referenceWireDriver(t *testing.T, trace *[]string, reply func(*http.Request) (int, string)) *Driver {
	t.Helper()
	d := &Driver{token: "fixture-open", refresh: "fixture-refresh", cookie: "UID=fixture"}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "proapi.115.com" {
			t.Errorf("unexpected identity endpoint %s", r.URL)
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("Open request leaked Cookie")
		}
		if r.Header.Get("Authorization") != "Bearer fixture-open" {
			t.Errorf("authorization=%s", r.Header.Get("Authorization"))
		}
		_ = r.ParseForm()
		*trace = append(*trace, r.Method+" "+r.URL.Path+" "+r.Form.Encode())
		status, body := reply(r)
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	return d
}

const referenceEntry = `{"state":true,"data":{"file_id":"10","file_name":"movie.mkv","pick_code":"source-pick","size":"100"}}`
const referenceCopiedEntry = `{"state":true,"data":{"file_id":"20","file_name":"movie.mkv","pick_code":"copy-pick","size":"100"}}`
const referenceLink = `{"state":true,"data":{"url":"https://cdn.example/movie","file_name":"movie.mkv","file_size":100}}`

func TestReferenceWirePathDoubleLookupThenDownurl(t *testing.T) {
	var trace []string
	d := referenceWireDriver(t, &trace, func(r *http.Request) (int, string) {
		if r.URL.Path == pathDownload {
			if r.UserAgent() != "exact-player-UA" {
				t.Errorf("UA=%q", r.UserAgent())
			}
			return 200, referenceLink
		}
		return 200, referenceEntry
	})
	_, err := d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: "/movie.mkv", UA: "exact-player-UA"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /open/folder/get_info path=%2Fmovie.mkv", "POST /open/folder/get_info path=%2Fmovie.mkv", "POST /open/ufile/downurl pick_code=source-pick"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace=%v", trace)
	}
	t.Log(trace)
}
func TestReferenceWire401DoesNotRefreshOrUseCookie(t *testing.T) {
	var trace []string
	d := referenceWireDriver(t, &trace, func(*http.Request) (int, string) { return 401, `{}` })
	d.rememberPickCode(fileEntry{FileID: "10", PickCode: "source-pick"})
	_, err := d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: "10", UA: "player"})
	if err == nil {
		t.Fatal("401 succeeded")
	}
	if len(trace) != 1 || trace[0] != "POST /open/ufile/downurl pick_code=source-pick" {
		t.Fatalf("refresh/fallback trace=%v", trace)
	}
	t.Log(trace)
}
func TestReferenceWireTemporaryCopyOrderAndExactTrash(t *testing.T) {
	for _, source := range []string{"/movie.mkv", "10"} {
		t.Run(source, func(t *testing.T) {
			var trace []string
			copied := false
			d := referenceWireDriver(t, &trace, func(r *http.Request) (int, string) {
				switch r.URL.Path {
				case pathCopy:
					copied = true
					return 200, `{"state":true}`
				case pathFileInfo:
					if copied && r.Method == "POST" {
						return 200, referenceCopiedEntry
					}
					return 200, referenceEntry
				case pathDownload:
					return 200, referenceLink
				default:
					return 200, `{"state":true}`
				}
			})
			cp, err := d.CreateTemporaryPlaybackCopy(context.Background(), source, "timestamp_movie.mkv")
			if err != nil {
				t.Fatal(err)
			}
			if cp.FileID != "20" {
				t.Fatalf("copy=%+v", cp)
			}
			if _, err = d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: cp.FileID, UA: "player"}); err != nil {
				t.Fatal(err)
			}
			if err = d.DeleteTemporaryPlaybackCopy(context.Background(), cp); err != nil {
				t.Fatal(err)
			}
			first := "POST /open/folder/get_info path=%2Fmovie.mkv"
			second := "POST /open/ufile/copy file_id=10&pid=0"
			if source == "10" {
				first, second = second, "GET /open/folder/get_info file_id=10"
			}
			want := []string{first, second, "POST /open/folder/get_info path=%2Fmovie.mkv", "POST /open/ufile/update file_id=20&file_name=timestamp_movie.mkv", "POST /open/ufile/downurl pick_code=copy-pick", "POST /open/ufile/delete file_ids=20"}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("trace=%v want=%v", trace, want)
			}
			t.Log(trace)
		})
	}
}
