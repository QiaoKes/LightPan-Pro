package strm

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path/filepath"
	"testing"
)

type treeUploadFixture struct {
	uploaded []driver.LocalUploadRequest
	resolved []string
	existing []domain.FileItem
}

func (f *treeUploadFixture) ResolveDirectoryID(_ context.Context, _ int64, p string) (string, error) {
	f.resolved = append(f.resolved, p)
	return "actual-parent", nil
}
func (f *treeUploadFixture) List(context.Context, int64, string, bool) ([]domain.FileItem, error) {
	return f.existing, nil
}
func (f *treeUploadFixture) UploadLocal(ctx context.Context, _ int64, r driver.LocalUploadRequest) (*driver.LocalUploadResult, error) {
	if !isMetadataSyncMutation(ctx) {
		panic("missing mutation marker")
	}
	f.uploaded = append(f.uploaded, r)
	return &driver.LocalUploadResult{}, nil
}
func TestTreeMetadataLocalModes(t *testing.T) {
	for _, mode := range []string{MetadataSyncCloudPrimary, MetadataSyncLocalPrimary, MetadataSyncBidirectional} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "season"), 0755); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(root, "season", "movie.nfo")
			if err := os.WriteFile(local, []byte("manual"), 0644); err != nil {
				t.Fatal(err)
			}
			f := &treeUploadFixture{}
			task := &domain.StrmTask{AccountID: 1, Path: "/mount/media", Recursive: true, SyncMetadata: true}
			err := uploadTreeLocalMetadata(context.Background(), task, ScanSettings{MetadataSyncMode: mode, MetadataExtensions: "nfo"}, []driver.TreeEntry{{Path: "/mount/media/season/movie.mkv"}}, root, f, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == MetadataSyncBidirectional {
				want = 1
			}
			if len(f.uploaded) != want {
				t.Fatalf("uploads=%d", len(f.uploaded))
			}
			if want == 1 {
				if f.resolved[0] != "/mount/media/season" || f.uploaded[0].ParentID != "actual-parent" || f.uploaded[0].ConflictPolicy != "skip" {
					t.Fatalf("wrong upload %#v paths%v", f.uploaded, f.resolved)
				}
			}
			b, err := os.ReadFile(local)
			if err != nil || string(b) != "manual" {
				t.Fatal("manual changed")
			}
		})
	}
}
func TestTreeMetadataNoOverwriteOrSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.nfo")
	os.WriteFile(outside, []byte("secret"), 0644)
	os.WriteFile(filepath.Join(root, "existing.nfo"), []byte("manual"), 0644)
	os.Symlink(outside, filepath.Join(root, "linked.nfo"))
	f := &treeUploadFixture{existing: []domain.FileItem{{Name: "existing.nfo"}}}
	task := &domain.StrmTask{AccountID: 1, Path: "/media", Recursive: true, SyncMetadata: true}
	err := uploadTreeLocalMetadata(context.Background(), task, ScanSettings{MetadataSyncMode: MetadataSyncBidirectional, MetadataExtensions: "nfo"}, []driver.TreeEntry{{Path: "/media/movie.mkv"}}, root, f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.uploaded) != 0 {
		t.Fatal("overwrote remote or followed symlink")
	}
}
