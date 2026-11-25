package strm

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path/filepath"
	"testing"
)

func TestRenamePublicationCollisionPreservesOldOwnedOutput(t *testing.T) {
	svc, _ := testService(t)
	svc.strmDir = t.TempDir()
	task := &domain.StrmTask{ID: 71, AccountID: 1, Path: "/movies", OutputFolder: "owned", Recursive: true}
	root := TaskOutputDir(svc.strmDir, "owned")
	if err := saveTreeInventory(root, treeInventory{TaskID: 71, Files: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.applyLifeEvent(ctx, task, driver.LifeEvent{Kind: "create", Path: "/movies/old.mkv", Verified: true}); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(root, "old.strm"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "new.strm"), []byte("manual collision"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = svc.applyLifeEvent(ctx, task, driver.LifeEvent{Kind: "rename", OldPath: "/movies/old.mkv", Path: "/movies/new.mkv", Verified: true}); err == nil {
		t.Fatal("collision accepted")
	}
	kept, err := os.ReadFile(filepath.Join(root, "old.strm"))
	if err != nil || string(kept) != string(old) {
		t.Fatalf("old output lost: %q %v", kept, err)
	}
	inv, err := loadTreeInventory(root, 71)
	if err != nil || inv.Files["old.strm"] != treeFingerprint(old) {
		t.Fatalf("ownership lost: %v %v", inv, err)
	}
	manual, err := os.ReadFile(filepath.Join(root, "new.strm"))
	if err != nil || string(manual) != "manual collision" {
		t.Fatal("manual destination changed")
	}
}
