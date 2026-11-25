package strm

import (
	"context"
	"errors"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/settings"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memoryEventStore struct {
	state EventSyncState
	saves int
}

func (s *memoryEventStore) LoadEventSync(context.Context, int64) (EventSyncState, error) {
	return s.state, nil
}
func (s *memoryEventStore) SaveEventSync(_ context.Context, _ int64, v EventSyncState) error {
	s.state = v
	s.saves++
	return nil
}

type fixtureEvents struct {
	batch   driver.LifeBatch
	request driver.LifeRequest
}

func (p *fixtureEvents) ReadLifeEvents(_ context.Context, r driver.LifeRequest) (driver.LifeBatch, error) {
	p.request = r
	return p.batch, nil
}
func TestEventSyncCheckpointAfterOutput(t *testing.T) {
	store := &memoryEventStore{state: EventSyncState{SnapshotComplete: true, SinceUnix: 100}}
	provider := &fixtureEvents{batch: driver.LifeBatch{Complete: true, NextCursor: "next", Events: []driver.LifeEvent{{ID: "event", Verified: true, UpdateUnix: 110}}}}
	failure := errors.New("publish failed")
	syncer := EventSync{Store: store, Provider: provider, Apply: func(context.Context, driver.LifeEvent) error { return failure }}
	if !errors.Is(syncer.Poll(context.Background(), 1), failure) || store.state.Cursor != "" || len(store.state.Receipts) != 0 {
		t.Fatal("failed output committed checkpoint")
	}
	store.state.NeedsReconcile = false // a successful external baseline releases the failure gate
	calls := 0
	syncer.Apply = func(context.Context, driver.LifeEvent) error { calls++; return nil }
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if provider.request.SinceUnix != 70 || store.state.Cursor != "next" {
		t.Fatal("checkpoint/overlap mismatch")
	}
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("event replay was not deduplicated")
	}
}
func TestEventSyncWaitsForSnapshot(t *testing.T) {
	store := &memoryEventStore{}
	syncer := EventSync{Store: store}
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if store.saves != 0 {
		t.Fatal("uninitialized task advanced")
	}
}

func TestLifeDeleteHonorsMetadataModeAndOwnership(t *testing.T) {
	for _, mode := range []string{"cloud_primary", "local_primary", "bidirectional"} {
		t.Run(mode, func(t *testing.T) {
			svc, _ := testService(t)
			svc.strmDir = t.TempDir()
			if err := svc.settings.Update(context.Background(), map[string]string{settings.KeyStrmMetadataSyncMode: mode}); err != nil {
				t.Fatal(err)
			}
			task := &domain.StrmTask{ID: 9, Path: "/Movies", OutputFolder: "managed", Recursive: true}
			root := TaskOutputDir(svc.strmDir, TaskRelDir(task.GroupDir, task.OutputFolder))
			if err := os.MkdirAll(filepath.Join(root, "A"), 0755); err != nil {
				t.Fatal(err)
			}
			inv := treeInventory{TaskID: 9, Files: map[string]string{}}
			for _, name := range []string{"A/a.strm", "A/poster.jpg", "A/manual.txt"} {
				body := []byte(name)
				if err := os.WriteFile(filepath.Join(root, name), body, 0644); err != nil {
					t.Fatal(err)
				}
				if !strings.HasSuffix(name, "manual.txt") {
					inv.Files[name] = treeFingerprint(body)
				}
			}
			if err := saveTreeInventory(root, inv); err != nil {
				t.Fatal(err)
			}
			err := svc.applyLifeEvent(context.Background(), task, driver.LifeEvent{Kind: "delete", Path: "/Movies/A", IsDir: true, Verified: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(root, "A/a.strm")); !os.IsNotExist(err) {
				t.Fatal("owned STRM retained")
			}
			_, err = os.Stat(filepath.Join(root, "A/poster.jpg"))
			if (mode == "cloud_primary") != os.IsNotExist(err) {
				t.Fatal("metadata policy bypassed")
			}
			if _, err = os.Stat(filepath.Join(root, "A/manual.txt")); err != nil {
				t.Fatal("manual file removed")
			}
		})
	}
}

func TestEmptyCompleteBatchesAdvanceStartWatermarkAndBoundReceipts(t *testing.T) {
	store := &memoryEventStore{state: EventSyncState{SnapshotComplete: true, SinceUnix: 100, Receipts: map[string]int64{"old": 20, "overlap": 95}}}
	provider := &fixtureEvents{batch: driver.LifeBatch{Complete: true, ThroughUnix: 120}}
	syncer := EventSync{Store: store, Provider: provider}
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if store.state.SinceUnix != 120 || len(store.state.Receipts) != 1 {
		t.Fatal(store.state)
	}
	provider.batch.ThroughUnix = 150
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if provider.request.SinceUnix != 90 || store.state.SinceUnix != 150 || len(store.state.Receipts) != 0 {
		t.Fatal(provider.request, store.state)
	}
}

func TestEventSyncUsesKnownPriorIdentityForMove(t *testing.T) {
	store := &memoryEventStore{state: EventSyncState{SnapshotComplete: true, SinceUnix: 100, FilePaths: map[string]string{"file": "/old/movie.mkv"}}}
	p := &fixtureEvents{batch: driver.LifeBatch{Complete: true, ThroughUnix: 120, Events: []driver.LifeEvent{{ID: "move", FileID: "file", Kind: "move", OldPath: "/new/guessed.mkv", Path: "/new/movie.mkv", Verified: true, UpdateUnix: 110}}}}
	syncer := EventSync{Store: store, Provider: p, Apply: func(_ context.Context, event driver.LifeEvent) error {
		if event.OldPath != "/old/movie.mkv" {
			t.Fatal(event)
		}
		return nil
	}}
	if err := syncer.Poll(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if store.state.FilePaths["file"] != "/new/movie.mkv" {
		t.Fatal(store.state)
	}
}
