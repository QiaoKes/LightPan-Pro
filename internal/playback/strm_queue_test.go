package playback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"litepan/internal/cache"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
)

type queuedSTRMTraceDriver struct {
	transferRefreshDriver
	ordinary, queued atomic.Int32
	started          chan struct{}
	release          chan struct{}
}

func (d *queuedSTRMTraceDriver) ResolveDownload(context.Context, driver.DownloadRequest) (*domain.DownloadInfo, error) {
	d.ordinary.Add(1)
	return &domain.DownloadInfo{URL: "https://cdn.example/file", FileName: "movie.mkv", Size: 100, Mode: domain.DownloadRedirect, Expiration: 1200 * time.Second}, nil
}
func (d *queuedSTRMTraceDriver) ResolveSTRMDownload(ctx context.Context, req driver.DownloadRequest) (*domain.DownloadInfo, error) {
	d.queued.Add(1)
	if d.started != nil {
		select {
		case d.started <- struct{}{}:
		default:
		}
	}
	if d.release != nil {
		select {
		case <-d.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &domain.DownloadInfo{URL: "https://cdn.example/strm", FileName: "movie.mkv", Size: 100, Mode: domain.DownloadRedirect, Expiration: 1200 * time.Second}, nil
}
func TestSTRMQueueCacheFirstCoalescingAndEmbyIsolation(t *testing.T) {
	d := &queuedSTRMTraceDriver{started: make(chan struct{}, 1), release: make(chan struct{})}
	c := cache.NewService(cache.Options{})
	defer c.Close()
	s := NewService(driverexec.New(referenceTraceProvider{d}, nil), c)
	play := func(strm bool) {
		t.Helper()
		r := httptest.NewRequest("GET", "/play", nil)
		r.Header.Set("User-Agent", "Lavf/61")
		w := httptest.NewRecorder()
		if err := s.ServeHTTP(w, r, Request{AccountID: 1, FileID: "file"}, Intent{STRM: strm}); err != nil {
			t.Error(err)
		}
		if w.Code != http.StatusFound {
			t.Errorf("status=%d", w.Code)
		}
	}
	// Ordinary/Emby resolution neither joins nor warms the STRM queue cache.
	play(false)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); play(true) }()
	}
	select {
	case <-d.started:
	case <-time.After(time.Second):
		t.Fatal("queue not used")
	}
	close(d.release)
	wg.Wait()
	for i := 0; i < 20; i++ {
		play(true)
	}
	play(false)
	if d.queued.Load() != 1 || d.ordinary.Load() != 1 {
		t.Fatalf("queued=%d ordinary=%d", d.queued.Load(), d.ordinary.Load())
	}
}
