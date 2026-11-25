package strm

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path/filepath"
	"testing"
)

func TestCrosslayerMonitorEscapedPathRetainsPlaybackIdentity(t *testing.T) {
	for _, rel := range []string{`a\\b/\../movie.mkv`, `a\/b/movie.mkv`, `\./电影\\name.mkv`} {
		t.Run(rel, func(t *testing.T) {
			s, _ := testService(t)
			s.strmDir = t.TempDir()
			if err := s.settings.Update(context.Background(), map[string]string{"strm_token": "fixture-token"}); err != nil {
				t.Fatal(err)
			}
			task := &domain.StrmTask{ID: 91, AccountID: 1, Path: "/Movies", OutputFolder: "owned", Recursive: true}
			root := TaskOutputDir(s.strmDir, "owned")
			if err := saveTreeInventory(root, treeInventory{TaskID: 91, Files: map[string]string{}}); err != nil {
				t.Fatal(err)
			}
			// Deliberately no file/remote service: generic media event publication must
			// preserve provider strings without an extra identity lookup.
			if err := s.applyLifeEvent(context.Background(), task, driver.LifeEvent{Kind: "create", Path: "/Movies/" + rel, Verified: true}); err != nil {
				t.Fatal(err)
			}
			inv, err := loadTreeInventory(root, 91)
			if err != nil || len(inv.Files) != 1 {
				t.Fatalf("inventory=%v err=%v", inv, err)
			}
			for local := range inv.Files {
				body, err := os.ReadFile(filepath.Join(root, local))
				if err != nil {
					t.Fatal(err)
				}
				ref, ok := ParsePlayReference(string(body))
				if !ok || ref.RootID != "/Movies" || ref.RelativePath != rel {
					t.Fatalf("path lost escaping: ref=%+v body=%s", ref, body)
				}
				if _, err := safeOwnedPath(root, local); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.applyLifeEvent(context.Background(), task, driver.LifeEvent{Kind: "delete", Path: "/Movies/" + rel, Verified: true}); err != nil {
				t.Fatal(err)
			}
			inv, err = loadTreeInventory(root, 91)
			if err != nil || len(inv.Files) != 0 {
				t.Fatalf("owned delete mismatch=%v %v", inv, err)
			}
		})
	}
}
