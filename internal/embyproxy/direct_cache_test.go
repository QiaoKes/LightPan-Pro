package embyproxy

import (
	"context"
	"fmt"
	"litepan/internal/playback"
	"litepan/internal/strm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlaybackDeviceUsesExplicitIdentityThenIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/Videos/1/stream?DeviceId=television", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	if got := playbackDevice(r); got != "television" {
		t.Fatalf("device=%q", got)
	}
	r.URL.RawQuery = ""
	if got := playbackDevice(r); got != "192.0.2.1" {
		t.Fatalf("fallback=%q", got)
	}
	r.Header.Set("User-Agent", "another player")
	if got := playbackDevice(r); got != "192.0.2.1" {
		t.Fatalf("UA changed identity: %q", got)
	}
}

func TestSessionPredicatePreservesDuplicates(t *testing.T) {
	for _, tc := range []struct {
		devices []string
		want    bool
	}{
		{nil, false}, {[]string{"device"}, false}, {[]string{"other"}, true},
		{[]string{"device", "device"}, true}, {[]string{"device", "other"}, true},
	} {
		if got := differentPlaybackDevice(tc.devices, "device"); got != tc.want {
			t.Fatalf("devices=%v got=%v", tc.devices, got)
		}
	}
}

func TestDirectCacheTwentySeeksAvoidUpstreamAndPathResolution(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		http.Error(w, "unexpected upstream", 500)
	}))
	defer upstream.Close()
	svc := testEmbyProxyService(t, upstream.URL)
	cfg := Config{ID: "seek-test", EmbyURL: upstream.URL}
	pathCalls, links := 0, 0
	svc.resolvePath = func(context.Context, int64, string, string) (string, error) { pathCalls++; return "file-id", nil }
	svc.servePlayback = func(w http.ResponseWriter, r *http.Request, _ playback.Request, _ playback.Intent) error {
		links++
		http.Redirect(w, r, "https://cdn.example/media", 302)
		return nil
	}
	playURL := strm.BuildPathPlayURL("http://litepan.test", 12, "root", "movie.mkv", "movie.mkv", "token", false, nil)
	svc.rememberMediaSource(cfg, "123", "ms1", playURL)
	for i := 0; i < 21; i++ {
		r := httptest.NewRequest("GET", "/Videos/123/stream?MediaSourceId=ms1&DeviceId=tv", nil)
		r.Header.Set("User-Agent", "player")
		r.Header.Set("Range", fmt.Sprintf("bytes=%d-", i*1024))
		w := httptest.NewRecorder()
		svc.redirectSTRMStream(w, r, cfg, "Videos/123/stream")
		if w.Code != 302 || w.Header().Get("Location") != "https://cdn.example/media" {
			t.Fatalf("seek %d status=%d", i, w.Code)
		}
	}
	if upstreamCalls != 0 || pathCalls != 1 || links != 1 {
		t.Fatalf("upstream=%d paths=%d links=%d", upstreamCalls, pathCalls, links)
	}
	r := httptest.NewRequest("GET", "/Videos/123/stream?MediaSourceId=ms1&DeviceId=other", nil)
	r.Header.Set("User-Agent", "player")
	svc.redirectSTRMStream(httptest.NewRecorder(), r, cfg, "Videos/123/stream")
	if links != 2 {
		t.Fatalf("different device reused link: %d", links)
	}
}
