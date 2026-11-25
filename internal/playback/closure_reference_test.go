package playback

import (
	"context"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"testing"
	"time"
)

type closureTemporaryDriver struct {
	traceCleanupDriver
	linkCompleted time.Time
}

func (d *closureTemporaryDriver) CreateTemporaryPlaybackCopy(context.Context, string, string) (driver.TemporaryPlaybackCopy, error) {
	return driver.TemporaryPlaybackCopy{FileID: "owned"}, nil
}
func (d *closureTemporaryDriver) ResolveDownload(context.Context, driver.DownloadRequest) (*domain.DownloadInfo, error) {
	d.linkCompleted = time.Now()
	return &domain.DownloadInfo{URL: "https://cdn.example/movie"}, nil
}
func TestClosureTemporaryDueAfterLinkAndNoEarlyDeletion(t *testing.T) {
	d := &closureTemporaryDriver{}
	l := &traceLedger{rows: map[string]domain.TemporaryPlaybackRecord{}}
	s := NewService(driverexec.New(referenceTraceProvider{d}, nil), nil)
	s.SetTemporaryPlaybackRepository(l)
	if _, err := s.ResolveTemporaryPlayback(context.Background(), 1, "source", "player"); err != nil {
		t.Fatal(err)
	}
	if len(l.rows) != 1 {
		t.Fatalf("ownership=%v", l.rows)
	}
	for id, r := range l.rows {
		if r.Due.Before(d.linkCompleted.Add(10*time.Second)) || r.Due.After(time.Now().Add(10*time.Second)) {
			t.Fatal("cleanup due must follow link attempt by 10s")
		}
		s.cleanupTemporary(context.Background())
		if len(d.deleted) != 0 {
			t.Fatal("early deletion")
		}
		r.Due = time.Now().Add(-time.Nanosecond)
		l.rows[id] = r
	}
	s.cleanupTemporary(context.Background())
	if len(d.deleted) != 1 || d.deleted[0] != "owned" || len(l.rows) != 0 {
		t.Fatalf("delete=%v ledger=%v", d.deleted, l.rows)
	}
}
