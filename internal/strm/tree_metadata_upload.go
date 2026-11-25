package strm

import (
	"context"
	"io/fs"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path"
	"strings"
)

type treeMetadataFiles interface {
	metadataFileService
	ResolveDirectoryID(context.Context, int64, string) (string, error)
}

func syncTreeLocalMetadata(ctx context.Context, task *domain.StrmTask, deps ScanDeps, entries []driver.TreeEntry, output string) error {
	return uploadTreeLocalMetadata(ctx, task, deps.Settings, entries, output, deps.Files, deps.Failures)
}

// Local-only files remain user-owned: uploading never adopts or removes them.
func uploadTreeLocalMetadata(ctx context.Context, task *domain.StrmTask, settings ScanSettings, entries []driver.TreeEntry, output string, files treeMetadataFiles, failures *FailureCollector) error {
	if !task.SyncMetadata || normalizeMetadataSyncMode(settings.MetadataSyncMode) != MetadataSyncBidirectional {
		return nil
	}
	rules := newScanRules(task, settings)
	base := strings.TrimRight(task.Path, "/")
	dirs := map[string]bool{}
	remote := map[string]bool{}
	excluded := func(rel string) bool {
		if !task.Recursive && strings.Contains(rel, "/") {
			return true
		}
		for _, kw := range rules.excludeFiles {
			if strings.Contains(path.Base(rel), kw) {
				return true
			}
		}
		for _, kw := range rules.excludeDirs {
			if strings.Contains(path.Dir(rel), kw) {
				return true
			}
		}
		return false
	}
	for _, entry := range entries {
		if entry.IsDir || (base != "" && !strings.HasPrefix(entry.Path, base+"/")) {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(entry.Path, base), "/")
		if excluded(rel) {
			continue
		}
		remote[metadataPathKey(rel)] = true
		if _, ok := rules.mediaExts[strings.TrimPrefix(strings.ToLower(path.Ext(rel)), ".")]; !ok {
			continue
		}
		dir := path.Dir(rel)
		dirs[dir] = true
		if settings.MetadataParentEnabled {
			for dir != "." {
				dir = path.Dir(dir)
				dirs[dir] = true
			}
		}
	}
	if _, err := safeOwnedPath(output, treeInventoryName); err != nil {
		return err
	}
	root, err := os.OpenRoot(output)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	var uploads []localMetadataItem
	for dir := range dirs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if dir != "." {
			if _, err := safeOwnedPath(output, dir); err != nil {
				return err
			}
		}
		listing, err := fs.ReadDir(root.FS(), dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		parentID := ""
		for _, entry := range listing {
			rel := path.Join(dir, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || excluded(rel) || remote[metadataPathKey(rel)] || !isMetadataExtension(entry.Name(), rules.metadataExts) {
				continue
			}
			full, err := safeOwnedPath(output, rel)
			if err != nil {
				return err
			}
			info, err := root.Stat(rel)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || (settings.MetadataMaxSizeMB > 0 && info.Size() > int64(settings.MetadataMaxSizeMB)*1024*1024) {
				continue
			}
			if parentID == "" {
				parentID, err = files.ResolveDirectoryID(ctx, task.AccountID, path.Join("/", base, dir))
				if err != nil {
					return err
				}
			}
			uploads = append(uploads, localMetadataItem{parentID: parentID, fileName: entry.Name(), relPath: rel, absPath: full, size: info.Size(), modTime: info.ModTime()})
		}
	}
	_, err = uploadLocalMetadata(ctx, task.AccountID, uploads, files, failures, nil)
	return err
}
