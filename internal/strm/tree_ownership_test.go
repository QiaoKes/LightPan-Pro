package strm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeInventoryRejectsUnownedAndSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manual.nfo"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTreeInventory(root, 1); err == nil {
		t.Fatal("adopted manual directory")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := safeOwnedPath(link, "manual.nfo"); err == nil {
		t.Fatal("followed symlink")
	}
	if _, err := safeOwnedPath(root, "../escape"); err == nil {
		t.Fatal("allowed traversal")
	}
}
func TestTreeInventoryRoundTripAndTaskOwnership(t *testing.T) {
	root := t.TempDir()
	inv := treeInventory{TaskID: 7, Files: map[string]string{"media.strm": "hash"}}
	if err := saveTreeInventory(root, inv); err != nil {
		t.Fatal(err)
	}
	got, err := loadTreeInventory(root, 7)
	if err != nil || got.Files["media.strm"] != "hash" {
		t.Fatal(got, err)
	}
	if _, err = loadTreeInventory(root, 8); err == nil {
		t.Fatal("shared output ownership")
	}
}

func TestTaskOutputOverlapAndOwnedDelete(t *testing.T) {
	root := t.TempDir()
	for _, candidate := range []string{"one", "one/sub"} {
		if err := ValidateTaskOutput(root, 0, candidate, map[int64]string{1: "one"}); err == nil {
			t.Fatal("overlap accepted", candidate)
		}
	}
	if err := ValidateTaskOutput(root, 0, "one", map[int64]string{1: "one/sub"}); err == nil {
		t.Fatal("ancestor accepted")
	}
	if err := ValidateTaskOutput(root, 0, "two", map[int64]string{1: "one"}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "owned")
	os.MkdirAll(out, 0755)
	b := []byte("generated")
	os.WriteFile(filepath.Join(out, "a.strm"), b, 0644)
	os.WriteFile(filepath.Join(out, "manual.nfo"), b, 0644)
	if err := saveTreeInventory(out, treeInventory{TaskID: 1, Files: map[string]string{"a.strm": treeFingerprint(b)}}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTaskOutput(root, "owned"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "a.strm")); !os.IsNotExist(err) {
		t.Fatal("owned file retained", err)
	}
	if _, err := os.Stat(filepath.Join(out, "manual.nfo")); err != nil {
		t.Fatal("manual file removed", err)
	}
}
