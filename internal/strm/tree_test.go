package strm

import (
	"context"
	"fmt"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/file"
	"os"
	"path/filepath"
	"testing"
)

type snapshotTestDriver struct {
	standardScanTestDriver
	paths    []string
	complete bool
	failure  bool
}

func (d *snapshotTestDriver) ExportTree(_ context.Context, r driver.TreeRequest, emit func(driver.TreeEntry) error) (driver.TreeSnapshot, error) {
	for _, p := range d.paths {
		if err := emit(driver.TreeEntry{Path: p}); err != nil {
			return driver.TreeSnapshot{}, err
		}
	}
	if d.failure {
		return driver.TreeSnapshot{}, fmt.Errorf("export failed")
	}
	return driver.TreeSnapshot{Complete: d.complete}, nil
}

type snapshotProvider struct{ d *snapshotTestDriver }

func (p snapshotProvider) Get(context.Context, int64) (driver.Driver, error) { return p.d, nil }
func TestTreeScannerCompleteOwnershipAndFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	drv := &snapshotTestDriver{paths: []string{"/scope/a.mkv"}, complete: true}
	files := file.NewService(driverexec.New(snapshotProvider{drv}, nil), nil, nil, nil, nil, nil)
	task := &domain.StrmTask{ID: 17, AccountID: 2, ParentID: "42", Path: "/scope", OutputFolder: "owned", Recursive: true}
	deps := ScanDeps{Files: files, StrmDir: root, BaseURL: "https://local", Token: "token", Settings: ScanSettings{Tool115TreeEnabled: true}}
	deps.OnTreeComplete = func(_ context.Context, id int64, root string, started int64) error {
		if id != 17 || root != "/scope" || started <= 0 {
			t.Fatalf("baseline %d %s %d", id, root, started)
		}
		return nil
	}
	r, err := ScanTask(ctx, task, deps, domain.StrmRunModeFull)
	if err != nil || r.GeneratedCount != 1 {
		t.Fatal(r, err)
	}
	media := filepath.Join(root, "owned", "a.strm")
	raw, err := os.ReadFile(media)
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := ParsePlayReference(string(raw))
	if !ok || !ref.PathBased || ref.RelativePath != "a.mkv" || ref.RootID != "/scope" || ref.FileID != "" {
		t.Fatal(ref)
	}
	manual := filepath.Join(root, "owned", "manual.nfo")
	os.WriteFile(manual, []byte("manual"), 0644)
	drv.paths = nil
	drv.complete = false
	if _, err = ScanTask(ctx, task, deps, domain.StrmRunModeFull); err == nil {
		t.Fatal("incomplete accepted")
	}
	if _, err = os.Stat(media); err != nil {
		t.Fatal("incomplete deleted media", err)
	}
	drv.failure = true
	if _, err = ScanTask(ctx, task, deps, domain.StrmRunModeFull); err == nil {
		t.Fatal("failed accepted")
	}
	if _, err = os.Stat(media); err != nil {
		t.Fatal(err)
	}
	drv.failure = false
	drv.complete = true
	r, err = ScanTask(ctx, task, deps, domain.StrmRunModeFull)
	if err != nil || r.RemovedCount != 1 {
		t.Fatal(r, err)
	}
	if _, err = os.Stat(manual); err != nil {
		t.Fatal("manual removed", err)
	}
	if drv.listCalls != 0 {
		t.Fatal("recursive listing used")
	}
}
