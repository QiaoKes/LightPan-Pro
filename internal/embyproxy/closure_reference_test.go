package embyproxy

import (
	"context"
	"fmt"
	"litepan/internal/cache"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/playback"
	"litepan/internal/strm"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClosureColdMetadataWarmReuseExpiryAndOriginalBypass(t *testing.T) {
	var metadata, original, resolves int
	playURL := strm.BuildPathPlayURL("http://litepan.test", 12, "root", "movie.mkv", "movie.mkv", "token", false, nil)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/Items") {
			metadata++
			fmt.Fprintf(w, `{"Items":[{"Id":"123","MediaSources":[{"Id":"ms1","Path":%q}]}]}`, playURL)
			return
		}
		original++
		w.Header().Set("X-Original-Emby", "yes")
		w.WriteHeader(206)
	}))
	defer up.Close()
	svc := testEmbyProxyService(t, up.URL)
	svc.resolvePath = func(context.Context, int64, string, string) (string, error) { return "file", nil }
	fail := false
	svc.servePlayback = func(w http.ResponseWriter, r *http.Request, _ playback.Request, _ playback.Intent) error {
		resolves++
		if fail {
			return fmt.Errorf("Open fixture failure")
		}
		http.Redirect(w, r, "https://cdn.example/movie", 302)
		return nil
	}
	cfg := Config{ID: "closure", EmbyURL: up.URL}
	request := func(method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/Videos/123/stream?MediaSourceId=ms1&DeviceId=tv", nil)
		r.Header.Set("User-Agent", "player")
		r.Header.Set("Range", "bytes=100-")
		w := httptest.NewRecorder()
		svc.redirectSTRMStream(w, r, cfg, "Videos/123/stream")
		return w
	}
	before := time.Now()
	if w := request("GET"); w.Code != 302 {
		t.Fatalf("cold %d", w.Code)
	}
	for _, m := range []string{"GET", "HEAD"} {
		if w := request(m); w.Code != 302 {
			t.Fatal(w.Code)
		}
	}
	if metadata != 1 || resolves != 1 || original != 0 {
		t.Fatalf("cold/warm counts %d %d %d", metadata, resolves, original)
	}
	for _, slot := range svc.directSlots {
		if slot.entry.expires.Before(before.Add(1200*time.Second)) || slot.entry.expires.After(time.Now().Add(1200*time.Second)) {
			t.Fatal("TTL differs from 1200s")
		}
		slot.entry.expires = time.Now().Add(-time.Nanosecond)
	}
	fail = true
	w := request("GET")
	if w.Code != 206 || w.Header().Get("X-Original-Emby") != "yes" || original != 1 || resolves != 2 {
		t.Fatalf("bypass code=%d original=%d resolve=%d", w.Code, original, resolves)
	}
}

// Embedding supplies unused lifecycle methods; any unexpected call panics.
type closureProvider struct {
	driver.Driver
	trace *[]string
}

func (d *closureProvider) Get(context.Context, int64) (driver.Driver, error) { return d, nil }
func (d *closureProvider) Config() driver.Config                             { return driver.Config{Name: "fixture"} }
func (d *closureProvider) ResolveDownload(_ context.Context, r driver.DownloadRequest) (*domain.DownloadInfo, error) {
	*d.trace = append(*d.trace, "downurl:"+r.FileID)
	return &domain.DownloadInfo{URL: "https://cdn.example/movie", Mode: domain.DownloadRedirect, Expiration: 1200 * time.Second}, nil
}
func (d *closureProvider) CreateTemporaryPlaybackCopy(context.Context, string, string) (driver.TemporaryPlaybackCopy, error) {
	*d.trace = append(*d.trace, "copy")
	return driver.TemporaryPlaybackCopy{FileID: "copied"}, nil
}
func (d *closureProvider) DeleteTemporaryPlaybackCopy(context.Context, driver.TemporaryPlaybackCopy) error {
	return nil
}

type closureLedger struct{}

func (closureLedger) Save(context.Context, domain.TemporaryPlaybackRecord) error { return nil }
func (closureLedger) Pending(context.Context) ([]domain.TemporaryPlaybackRecord, error) {
	return nil, nil
}
func (closureLedger) Delete(context.Context, string) error { return nil }
func TestClosureColdSessionsAndRealPlaybackBranches(t *testing.T) {
	for _, device := range []string{"tv", "other"} {
		t.Run(device, func(t *testing.T) {
			var trace []string
			playURL := strm.BuildPathPlayURL("http://litepan.test", 12, "root", "movie.mkv", "movie.mkv", "token", false, nil)
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/Items") {
					trace = append(trace, "Items")
					fmt.Fprintf(w, `{"Items":[{"Id":"123","MediaSources":[{"Id":"ms1","Path":%q}]}]}`, playURL)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/Sessions") {
					trace = append(trace, "Sessions")
					fmt.Fprintf(w, `[{"DeviceId":%q,"PlayState":{"MediaSourceId":"ms1"}}]`, device)
					return
				}
				t.Errorf("unexpected upstream %s", r.URL)
				w.WriteHeader(500)
			}))
			defer up.Close()
			svc := testEmbyProxyService(t, up.URL)
			d := &closureProvider{trace: &trace}
			c := cache.NewService(cache.Options{})
			defer c.Close()
			ps := playback.NewService(driverexec.New(d, nil), c)
			ps.SetTemporaryPlaybackRepository(closureLedger{})
			svc.playback = ps
			svc.servePlayback = ps.ServeHTTP
			svc.resolvePath = func(context.Context, int64, string, string) (string, error) {
				trace = append(trace, "path")
				return "source", nil
			}
			cfg := Config{ID: "sessions", EmbyURL: up.URL}
			for _, method := range []string{"GET", "HEAD", "GET"} {
				r := httptest.NewRequest(method, "/Videos/123/stream?MediaSourceId=ms1&DeviceId=tv", nil)
				r.Header.Set("User-Agent", "player")
				r.Header.Set("Range", "bytes=100-")
				w := httptest.NewRecorder()
				svc.redirectSTRMStream(w, r, cfg, "Videos/123/stream")
				if w.Code != 302 {
					t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
				}
			}
			want := []string{"Items", "path", "Sessions", "downurl:source"}
			if device == "other" {
				want = []string{"Items", "path", "Sessions", "copy", "downurl:copied"}
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("trace=%v want=%v", trace, want)
			}
		})
	}
}
