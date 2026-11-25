package playback

import (
	"context"
	"fmt"
	"litepan/internal/cache"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type referenceTraceDriver struct {
	transferRefreshDriver
	endpoint string
	client   *http.Client
}

func (d *referenceTraceDriver) ResolveDownload(ctx context.Context, req driver.DownloadRequest) (*domain.DownloadInfo, error) {
	r, _ := http.NewRequestWithContext(ctx, "POST", d.endpoint+"/open/ufile/downurl", nil)
	r.Header.Set("User-Agent", req.UA)
	resp, err := d.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, domain.Errorf(domain.CodeDriverError, "Open failure %d", resp.StatusCode)
	}
	return &domain.DownloadInfo{URL: d.endpoint + "/cdn/movie", FileName: "movie.mkv", Size: 100000, Mode: domain.DownloadRedirect, Expiration: 1200 * time.Second}, nil
}

type referenceTraceProvider struct{ d driver.Driver }

func (p referenceTraceProvider) Get(context.Context, int64) (driver.Driver, error) { return p.d, nil }

func TestReferenceTraceTwentyRangeRedirects(t *testing.T) {
	var trace []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace = append(trace, r.Method+" "+r.URL.Path+" UA="+r.UserAgent())
		if r.URL.Path != "/open/ufile/downurl" {
			t.Errorf("server fetched CDN: %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	c := cache.NewService(cache.Options{})
	defer c.Close()
	d := &referenceTraceDriver{endpoint: upstream.URL, client: upstream.Client()}
	s := NewService(driverexec.New(referenceTraceProvider{d}, nil), c)
	play := func(ua string) {
		for i := 0; i < 20; i++ {
			method := "GET"
			if i%2 == 1 {
				method = "HEAD"
			}
			r := httptest.NewRequest(method, "/play", nil)
			r.Header.Set("User-Agent", ua)
			r.Header.Set("Range", fmt.Sprintf("bytes=%d-", i*100))
			w := httptest.NewRecorder()
			if err := s.ServeHTTP(w, r, Request{AccountID: 1, FileID: "/movie.mkv"}, Intent{}); err != nil {
				t.Fatal(err)
			}
			if w.Code != 302 || w.Header().Get("Location") != upstream.URL+"/cdn/movie" {
				t.Fatalf("response %d %s", w.Code, w.Body.String())
			}
		}
	}
	play("player-a")
	if len(trace) != 1 {
		t.Fatalf("20 seeks: %v", trace)
	}
	play("player-b")
	if len(trace) != 2 {
		t.Fatalf("UA isolation: %v", trace)
	}
	s.InvalidateAccount(1)
	play("player-a")
	if len(trace) != 3 {
		t.Fatalf("credential invalidation: %v", trace)
	}
	t.Logf("HTTP trace: %v", trace)
}

type traceLedger struct {
	rows map[string]domain.TemporaryPlaybackRecord
}

func (l *traceLedger) Save(_ context.Context, r domain.TemporaryPlaybackRecord) error {
	l.rows[r.ID] = r
	return nil
}
func (l *traceLedger) Pending(context.Context) ([]domain.TemporaryPlaybackRecord, error) {
	var out []domain.TemporaryPlaybackRecord
	for _, r := range l.rows {
		out = append(out, r)
	}
	return out, nil
}
func (l *traceLedger) Delete(_ context.Context, id string) error { delete(l.rows, id); return nil }

type traceCleanupDriver struct {
	referenceTraceDriver
	deleted []string
	fail    bool
}

func (d *traceCleanupDriver) CreateTemporaryPlaybackCopy(context.Context, string, string) (driver.TemporaryPlaybackCopy, error) {
	panic("unexpected copy during recovery")
}
func (d *traceCleanupDriver) DeleteTemporaryPlaybackCopy(_ context.Context, c driver.TemporaryPlaybackCopy) error {
	d.deleted = append(d.deleted, c.FileID)
	if d.fail {
		return fmt.Errorf("retry fixture")
	}
	return nil
}
func TestReferenceTraceCleanupRecovery(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			d := &traceCleanupDriver{fail: fail}
			l := &traceLedger{rows: map[string]domain.TemporaryPlaybackRecord{"ownership": {ID: "ownership", AccountID: 7, FileID: "copy-only", Due: time.Now().Add(-time.Minute)}}}
			// Recreate Service with durable pending ownership as startup does.
			s := NewService(driverexec.New(referenceTraceProvider{d}, nil), nil)
			s.SetTemporaryPlaybackRepository(l)
			s.cleanupTemporary(context.Background())
			if len(d.deleted) != 1 || d.deleted[0] != "copy-only" {
				t.Fatalf("delete trace=%v", d.deleted)
			}
			if fail {
				r, ok := l.rows["ownership"]
				if !ok || r.LastError == "" || !r.Due.After(time.Now()) {
					t.Fatalf("lost failed ownership: %+v", l.rows)
				}
			} else if len(l.rows) != 0 {
				t.Fatalf("successful cleanup retained: %v", l.rows)
			}
		})
	}
}
