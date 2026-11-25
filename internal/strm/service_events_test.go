package strm

import (
	"context"
	"errors"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/file"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceSharesAccountFetchAndIsolatesOutputFailure(t *testing.T) {
	ctx := context.Background()
	svc, st := testService(t)
	svc.strmDir = t.TempDir()
	if err := svc.settings.Update(context.Background(), map[string]string{"strm_token": "fixture-token"}); err != nil {
		t.Fatal(err)
	}
	svc.eventStore = st.DB
	aid, err := st.Accounts.Create(ctx, &domain.Account{Name: "event account", DriverType: "fixture", Config: "{}", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	var tasks []*domain.StrmTask
	for _, name := range []string{"conflict", "healthy"} {
		task := &domain.StrmTask{Name: name, AccountID: aid, ParentID: "0", Path: "/mounted", OutputFolder: name, Recursive: true, MonitoringEnabled: true, Status: domain.StrmStatusActive}
		id, e := st.StrmTasks.Create(ctx, task)
		if e != nil {
			t.Fatal(e)
		}
		task.ID = id
		tasks = append(tasks, task)
		root := TaskOutputDir(svc.strmDir, name)
		if e = saveTreeInventory(root, treeInventory{TaskID: id, Files: map[string]string{}}); e != nil {
			t.Fatal(e)
		}
		if e = st.DB.SaveEventSync(ctx, id, EventSyncState{RootPath: "/absolute/mounted", SnapshotComplete: true, SinceUnix: 100}); e != nil {
			t.Fatal(e)
		}
	}
	manual := filepath.Join(svc.strmDir, "conflict", "movie.strm")
	os.WriteFile(manual, []byte("manual"), 0644)
	calls := 0
	read := func(_ context.Context, account int64, r driver.LifeRequest) (driver.LifeBatch, error) {
		calls++
		if account != aid || r.SinceUnix != 70 {
			t.Fatal(account, r)
		}
		return driver.LifeBatch{Complete: true, ThroughUnix: 120, NextCursor: "next", Events: []driver.LifeEvent{{ID: "event", Kind: "create", Path: "/absolute/mounted/movie.mkv", Verified: true, UpdateUnix: 110}}}, nil
	}
	svc.pollEventTasks(ctx, tasks, read)
	if calls != 1 {
		t.Fatal("amplified account fetch", calls)
	}
	failed, _ := st.DB.LoadEventSync(ctx, tasks[0].ID)
	good, _ := st.DB.LoadEventSync(ctx, tasks[1].ID)
	if !failed.NeedsReconcile || failed.Cursor != "" || len(failed.Receipts) != 0 {
		t.Fatal(failed)
	}
	if good.Cursor != "next" || good.SinceUnix != 120 {
		t.Fatal(good)
	}
	body, e := os.ReadFile(filepath.Join(svc.strmDir, "healthy", "movie.strm"))
	if e != nil {
		t.Fatal(e)
	}
	ref, ok := ParsePlayReference(string(body))
	if !ok || ref.RootID != "/absolute/mounted" || ref.RelativePath != "movie.mkv" {
		t.Fatal(ref)
	}
	raw, _ := os.ReadFile(manual)
	if string(raw) != "manual" {
		t.Fatal("overwrote manual output")
	}
}

func TestEventRenameSameTargetUpdatesReferenceWithoutDeleting(t *testing.T) {
	svc, _ := testService(t)
	svc.strmDir = t.TempDir()
	if err := svc.settings.Update(context.Background(), map[string]string{"strm_token": "fixture-token"}); err != nil {
		t.Fatal(err)
	}
	task := &domain.StrmTask{ID: 7, AccountID: 1, Path: "/movies", OutputFolder: "owned", Recursive: true}
	root := TaskOutputDir(svc.strmDir, "owned")
	if err := saveTreeInventory(root, treeInventory{TaskID: 7, Files: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.applyLifeEvent(ctx, task, driver.LifeEvent{Kind: "create", Path: "/movies/movie.mkv", Verified: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.applyLifeEvent(ctx, task, driver.LifeEvent{Kind: "rename", OldPath: "/movies/movie.mkv", Path: "/movies/movie.mp4", Verified: true}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "movie.strm"))
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := ParsePlayReference(string(body))
	if !ok || ref.RelativePath != "movie.mp4" {
		t.Fatal(ref, string(body))
	}
	inv, err := loadTreeInventory(root, 7)
	if err != nil || inv.Files["movie.strm"] != treeFingerprint(body) || len(inv.Pending) != 0 {
		t.Fatal(inv, err)
	}
}

func TestPublicationRecoversCommittedPendingFingerprint(t *testing.T) {
	root := t.TempDir()
	body := []byte("published before ledger commit")
	os.WriteFile(filepath.Join(root, "a.strm"), body, 0644)
	inv := treeInventory{TaskID: 1, Files: map[string]string{}, Pending: map[string]string{"a.strm": treeFingerprint(body)}}
	if err := saveTreeInventory(root, inv); err != nil {
		t.Fatal(err)
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer confined.Close()
	if err = publishTreeOutput(root, confined, &inv, "a.strm", body); err != nil {
		t.Fatal(err)
	}
	if len(inv.Pending) != 0 || inv.Files["a.strm"] == "" {
		t.Fatal(inv)
	}
	if err = publishTreeOutput(root, confined, &inv, "a.strm", []byte(strings.Repeat("new", 5))); err != nil {
		t.Fatal(err)
	}
}

func TestTaskRootEditInvalidatesDurableEventBaseline(t *testing.T) {
	ctx := context.Background()
	svc, st := testService(t)
	svc.strmDir = t.TempDir()
	svc.eventStore = st.DB
	aid, err := st.Accounts.Create(ctx, &domain.Account{Name: "account", DriverType: "fixture", Config: "{}", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	task := &domain.StrmTask{Name: "task", AccountID: aid, ParentID: "one", Path: "/old", OutputFolder: "owned", Recursive: true, Status: domain.StrmStatusActive}
	id, err := st.StrmTasks.Create(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.DB.SaveEventSync(ctx, id, EventSyncState{RootPath: "/absolute/old", SnapshotComplete: true, SinceUnix: 50, Cursor: "old", Receipts: map[string]int64{"old": 49}}); err != nil {
		t.Fatal(err)
	}
	task.ID = id
	task.Path = "/new"
	task.ParentID = "two"
	if _, err = svc.UpdateTask(ctx, id, task); err != nil {
		t.Fatal(err)
	}
	state, err := st.DB.LoadEventSync(ctx, id)
	if err != nil || state.SnapshotComplete || state.Cursor != "" || state.RootPath != "" || state.SinceUnix != 0 {
		t.Fatal(state, err)
	}
}

func TestBaselinePreservesEarlierDurableWatermark(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	memory := &memoryEventStore{state: EventSyncState{SinceUnix: 100}}
	svc.eventStore = memory
	if err := svc.treeBaselineComplete(ctx, 1, "/absolute", 200); err != nil {
		t.Fatal(err)
	}
	if memory.state.SinceUnix != 100 || memory.state.RootPath != "/absolute" || !memory.state.SnapshotComplete {
		t.Fatal(memory.state)
	}
}

type directoryFixtureDriver struct {
	standardScanTestDriver
	calls   int
	failure error
}

func (d *directoryFixtureDriver) ExpandDirectoryEvents(context.Context, string) ([]driver.LifeEvent, error) {
	d.calls++
	if d.failure != nil {
		return nil, d.failure
	}
	return []driver.LifeEvent{{Kind: "create", Path: "/movies/new/child.mkv", Verified: true}}, nil
}

type directoryFixtureProvider struct{ d *directoryFixtureDriver }

func (p directoryFixtureProvider) Get(context.Context, int64) (driver.Driver, error) { return p.d, nil }
func TestServiceDirectoryThenFileCommitsBothEvents(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	svc.strmDir = t.TempDir()
	memory := &memoryEventStore{state: EventSyncState{SnapshotComplete: true, RootPath: "/movies", SinceUnix: 100}}
	svc.eventStore = memory
	fixture := &directoryFixtureDriver{}
	svc.files = file.NewService(driverexec.New(directoryFixtureProvider{fixture}, nil), nil, nil, nil, nil, nil)
	task := &domain.StrmTask{ID: 9, AccountID: 1, Path: "/movies", OutputFolder: "owned", Recursive: true, MonitoringEnabled: true, Status: domain.StrmStatusActive}
	if err := saveTreeInventory(TaskOutputDir(svc.strmDir, "owned"), treeInventory{TaskID: 9, Files: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	svc.pollEventTasks(ctx, []*domain.StrmTask{task}, func(context.Context, int64, driver.LifeRequest) (driver.LifeBatch, error) {
		return driver.LifeBatch{Complete: true, ThroughUnix: 120, NextCursor: "done", Events: []driver.LifeEvent{{ID: "dir", Kind: "create", Path: "/movies/new", FileID: "directory", IsDir: true, Verified: true, UpdateUnix: 110}, {ID: "file", Kind: "create", Path: "/movies/other.mkv", Verified: true, UpdateUnix: 111}}}, nil
	})
	if fixture.calls != 1 || memory.state.Cursor != "done" || len(memory.state.Receipts) != 2 {
		t.Fatal(fixture.calls, memory.state)
	}
	for _, rel := range []string{"new/child.strm", "other.strm"} {
		if _, err := os.Stat(filepath.Join(svc.strmDir, "owned", rel)); err != nil {
			t.Fatal(rel, err)
		}
	}
}

func TestDirectoryExpansionSharedOnlyWithinAccountPollRound(t *testing.T) {
	ctx := context.Background()
	svc, st := testService(t)
	svc.strmDir = t.TempDir()
	svc.eventStore = st.DB
	fixture := &directoryFixtureDriver{}
	svc.files = file.NewService(driverexec.New(directoryFixtureProvider{fixture}, nil), nil, nil, nil, nil, nil)
	invalidations := 0
	svc.files.SetPlaybackInvalidator(func(int64) { invalidations++ })
	aid, err := st.Accounts.Create(ctx, &domain.Account{Name: "account", DriverType: "fixture", Config: "{}", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	var tasks []*domain.StrmTask
	for _, name := range []string{"first", "second"} {
		task := &domain.StrmTask{Name: name, AccountID: aid, OutputFolder: name, Path: "/movies", Recursive: true, MonitoringEnabled: true, Status: domain.StrmStatusActive}
		id, e := st.StrmTasks.Create(ctx, task)
		if e != nil {
			t.Fatal(e)
		}
		task.ID = id
		tasks = append(tasks, task)
		if e = saveTreeInventory(TaskOutputDir(svc.strmDir, name), treeInventory{TaskID: id, Files: map[string]string{}}); e != nil {
			t.Fatal(e)
		}
		if e = st.DB.SaveEventSync(ctx, id, EventSyncState{RootPath: "/movies", SnapshotComplete: true, SinceUnix: 100}); e != nil {
			t.Fatal(e)
		}
	}
	for round, eventID := range []string{"one", "two"} {
		fetches := 0
		svc.pollEventTasks(ctx, tasks, func(context.Context, int64, driver.LifeRequest) (driver.LifeBatch, error) {
			fetches++
			return driver.LifeBatch{Complete: true, ThroughUnix: 120, Events: []driver.LifeEvent{{ID: eventID, FileID: "directory", Kind: "create", Path: "/movies/new", IsDir: true, Verified: true, UpdateUnix: 110}}}, nil
		})
		if fetches != 1 || fixture.calls != round+1 {
			t.Fatal("directory requests amplified or cached across ticks", fetches, fixture.calls)
		}
		if invalidations != 1 {
			t.Fatal("invalidations amplified by tasks or no-op", invalidations)
		}
		for _, task := range tasks {
			state, e := st.DB.LoadEventSync(ctx, task.ID)
			if e != nil || state.NeedsReconcile || state.Receipts[eventID] != 110 {
				t.Fatal(task.ID, state, e)
			}
			if _, e = os.Stat(filepath.Join(svc.strmDir, task.OutputFolder, "new/child.strm")); e != nil {
				t.Fatal(e)
			}
		}
	}
	svc.pollEventTasks(ctx, tasks, func(context.Context, int64, driver.LifeRequest) (driver.LifeBatch, error) {
		return driver.LifeBatch{Complete: true, ThroughUnix: 130, Events: []driver.LifeEvent{{ID: "outside", Kind: "create", FileID: "unrelated", Path: "/outside/directory", IsDir: true, Verified: true, UpdateUnix: 125}}}, nil
	})
	if invalidations != 1 || fixture.calls != 2 {
		t.Fatal("unrelated event evicted playback or expanded", invalidations, fixture.calls)
	}

	fixture.failure = errors.New("directory lookup failed")
	svc.pollEventTasks(ctx, tasks, func(context.Context, int64, driver.LifeRequest) (driver.LifeBatch, error) {
		return driver.LifeBatch{Complete: true, ThroughUnix: 140, Events: []driver.LifeEvent{{ID: "failed-directory", Kind: "create", FileID: "directory", Path: "/movies/new", IsDir: true, Verified: true, UpdateUnix: 135}}}, nil
	})
	if fixture.calls != 3 || invalidations != 1 {
		t.Fatal("expansion error not shared", fixture.calls, invalidations)
	}
	for _, task := range tasks {
		state, err := st.DB.LoadEventSync(ctx, task.ID)
		if err != nil || !state.NeedsReconcile || state.Receipts["failed-directory"] != 0 {
			t.Fatal(state, err)
		}
	}

}

type pickcodeAttachmentFixture struct {
	standardScanTestDriver
	pcCalls, idCalls int
}

func (d *pickcodeAttachmentFixture) DownloadAttachmentPickcode(_ context.Context, pc string) ([]byte, error) {
	d.pcCalls++
	if pc != "known-pc" {
		return nil, errors.New("wrong pickcode")
	}
	return []byte("subtitle"), nil
}
func (d *pickcodeAttachmentFixture) DownloadAttachment(context.Context, string) ([]byte, error) {
	d.idCalls++
	return nil, errors.New("unexpected skim route")
}

type pickcodeAttachmentProvider struct{ d *pickcodeAttachmentFixture }

func (p pickcodeAttachmentProvider) Get(context.Context, int64) (driver.Driver, error) {
	return p.d, nil
}
func TestDirectoryChildAttachmentUsesKnownPickcode(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	svc.strmDir = t.TempDir()
	fixture := &pickcodeAttachmentFixture{}
	svc.files = file.NewService(driverexec.New(pickcodeAttachmentProvider{fixture}, nil), nil, nil, nil, nil, nil)
	task := &domain.StrmTask{ID: 5, AccountID: 1, OutputFolder: "owned", Path: "/movies", SyncMetadata: true, Recursive: true}
	root := TaskOutputDir(svc.strmDir, "owned")
	if err := saveTreeInventory(root, treeInventory{TaskID: 5, Files: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.applyLifeEvent(ctx, task, driver.LifeEvent{Kind: "create", Path: "/movies/movie.srt", FileID: "file", Pickcode: "known-pc", Verified: true}); err != nil {
		t.Fatal(err)
	}
	if fixture.pcCalls != 1 || fixture.idCalls != 0 {
		t.Fatal(fixture.pcCalls, fixture.idCalls)
	}
	if body, err := os.ReadFile(filepath.Join(root, "movie.srt")); err != nil || string(body) != "subtitle" {
		t.Fatal(string(body), err)
	}
}
