package embyproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A warm direct cache must bypass even media metadata and session discovery.
func TestReferenceTraceWarmCachePrecedesAllEmbyHTTP(t *testing.T) {
	var trace []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace = append(trace, r.Method+" "+r.URL.Path)
		http.Error(w, "unexpected cold metadata fetch", 500)
	}))
	defer up.Close()
	svc := testEmbyProxyService(t, up.URL)
	cfg := Config{ID: "trace", EmbyURL: up.URL}
	r := httptest.NewRequest("GET", "/Videos/123/stream?MediaSourceId=ms1&DeviceId=tv", nil)
	r.Header.Set("User-Agent", "trace-player")
	r.Header.Set("Range", "bytes=100-")
	key := directPlaybackKey{server: embyMediaSourceCacheKey(cfg, ""), item: "123", source: "ms1", ua: r.UserAgent(), device: "tv"}
	slot := svc.acquireDirectSlot(key)
	slot.entry = directPlaybackEntry{location: "https://cdn.example/movie", expires: time.Now().Add(time.Minute)}
	for _, method := range []string{"GET", "HEAD"} {
		r.Method = method
		w := httptest.NewRecorder()
		svc.redirectSTRMStream(w, r, cfg, "Videos/123/stream")
		if w.Code != 302 || w.Header().Get("Location") != "https://cdn.example/movie" || len(trace) != 0 {
			t.Fatalf("status=%d location=%s HTTP trace=%v", w.Code, w.Header().Get("Location"), trace)
		}
	}
}
