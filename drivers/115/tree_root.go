package pan115

import (
	"context"
	"net/http"
	"path"
	"strings"

	"litepan/internal/domain"
)

// absoluteTreeRoot resolves the mount once per scan. Exported path references
// retain this cloud-absolute root so playback never repeats the mount lookup.
func (d *Driver) absoluteTreeRoot(ctx context.Context, mountRelative string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root := "/"
	if id := d.rootID(); id != "0" {
		var info fileEntry
		if err := d.apiCall(ctx, http.MethodGet, pathFileInfo, urlValues(map[string]string{"file_id": id}), nil, &info); err != nil {
			return "", err
		}
		if info.entryID() == "" {
			return "", domain.Errf(domain.CodeNotFound)
		}
		if len(info.Paths) == 0 {
			return "", domain.Errorf(domain.CodeDriverError, "115 挂载根缺少真实云路径")
		}
		relative, ok := buildDirPath(info.Paths, info.entryName(), "0")
		if !ok || relative == "" {
			return "", domain.Errorf(domain.CodeDriverError, "115 挂载根云路径无效")
		}
		root = path.Join("/", relative)
	}
	return path.Join(root, strings.TrimPrefix(mountRelative, "/")), nil
}
