package file

import (
	"context"
	"path"
	"testing"

	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
)

type nativePathDriver struct {
	uploadRootDriver
	resolved int
	listed   int
}

func (d *nativePathDriver) ResolvePath(_ context.Context, root, relative string) (*domain.FileItem, error) {
	d.resolved++
	return &domain.FileItem{ID: path.Join(root, relative), Name: path.Base(relative), IDKind: domain.IDPath}, nil
}

func (d *nativePathDriver) ListFiles(context.Context, string) ([]domain.FileItem, error) {
	d.listed++
	return nil, nil
}

func TestResolveNativePathPreservesPathWithoutListing(t *testing.T) {
	d := &nativePathDriver{}
	s := NewService(driverexec.New(uploadRootProvider{drv: d}, nil), nil, nil, nil, nil, nil)
	for range 20 {
		item, err := s.ResolvePath(context.Background(), 1, "/电影", "中文 子目录/片名.mkv")
		if err != nil {
			t.Fatal(err)
		}
		if item.ID != "/电影/中文 子目录/片名.mkv" || item.IDKind != domain.IDPath {
			t.Fatalf("path identity lost: %#v", item)
		}
	}
	if d.listed != 0 || d.resolved != 20 {
		t.Fatalf("unexpected calls: listed=%d resolved=%d", d.listed, d.resolved)
	}
	for _, relative := range []string{"../片名.mkv", "a/../片名.mkv", "/片名.mkv", "片名\x00.mkv"} {
		if _, err := s.ResolvePath(context.Background(), 1, "/电影", relative); err == nil {
			t.Fatalf("accepted unsafe path %q", relative)
		}
	}
	if d.resolved != 20 {
		t.Fatal("unsafe paths reached provider")
	}
}
