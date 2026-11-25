package pan115

import (
	"context"
	"path"
	"strings"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

// ResolvePath preserves a tree-export path reference until ResolveDownload.
// No metadata request belongs here: reference download_url_path performs its
// two get_item calls immediately before requesting the Open download URL.
func (d *Driver) ResolvePath(ctx context.Context, rootID, relativePath string) (*domain.FileItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := strings.TrimSpace(rootID)
	if !strings.HasPrefix(root, "/") {
		return nil, domain.Errorf(domain.CodeValidation, "115 路径解析需要绝对云路径作为根目录")
	}
	fullPath := path.Join(root, relativePath)
	return &domain.FileItem{ID: fullPath, Name: path.Base(fullPath), IDKind: domain.IDPath, IsDir: relativePath == "" || relativePath == "."}, nil
}

var _ driver.PathResolver = (*Driver)(nil)
