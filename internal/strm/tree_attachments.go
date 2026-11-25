package strm

import (
	"context"
	"fmt"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path"
	"strings"
)

func syncTreeAttachments(ctx context.Context, task *domain.StrmTask, deps ScanDeps, entries []driver.TreeEntry, output string, inv *treeInventory, seen map[string]bool) error {
	if !task.SyncMetadata {
		return nil
	}
	confined, err := os.OpenRoot(output)
	if err != nil {
		return err
	}
	defer confined.Close()
	rules := newScanRules(task, deps.Settings)
	base := strings.TrimRight(task.Path, "/")
	attachmentSeen := map[string]bool{}
	relative := func(remote string) (string, error) {
		if base != "" && !strings.HasPrefix(remote, base+"/") {
			return "", fmt.Errorf("attachment outside snapshot root")
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(remote, base), "/")
		if _, e := safeOwnedPath(output, rel); e != nil {
			return "", e
		}
		return rel, nil
	}
	eligible := func(rel string) bool {
		if !task.Recursive && strings.Contains(rel, "/") {
			return false
		}
		for _, keyword := range rules.excludeFiles {
			if strings.Contains(path.Base(rel), keyword) {
				return false
			}
		}
		for _, keyword := range rules.excludeDirs {
			if strings.Contains(path.Dir(rel), keyword) {
				return false
			}
		}
		_, ok := rules.metadataExts[strings.TrimPrefix(strings.ToLower(path.Ext(rel)), ".")]
		return ok
	}
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		rel, e := relative(entry.Path)
		if e != nil {
			return e
		}
		if !eligible(rel) {
			continue
		}
		attachmentSeen[rel] = true
		seen[rel] = true
		ext := strings.ToLower(path.Ext(rel))
		if ext == ".srt" || ext == ".ass" || ext == ".ssa" {
			continue
		}
		if _, e = confined.Stat(rel); e == nil {
			continue
		} else if !os.IsNotExist(e) {
			return e
		}
		var body []byte
		if entry.StableID == "" {
			body, e = deps.Files.DownloadAttachmentPath(ctx, task.AccountID, entry.Path)
		} else {
			body, e = deps.Files.DownloadAttachment(ctx, task.AccountID, entry.StableID)
		}
		if e != nil {
			return e
		}
		if e = publishTreeOutput(output, confined, inv, rel, body); e != nil {
			return e
		}
	}
	specialEnabled := false
	for _, suffix := range []string{"srt", "ass", "ssa"} {
		if _, ok := rules.metadataExts[suffix]; ok {
			specialEnabled = true
		}
	}
	if specialEnabled {
		skip := func(remote string) bool {
			rel, e := relative(remote)
			if e != nil {
				return false
			}
			if !eligible(rel) {
				return true
			}
			_, e = confined.Stat(rel)
			if e == nil {
				attachmentSeen[rel] = true
				seen[rel] = true
			}
			return e == nil
		}
		e := deps.Files.SyncSubtitleAttachments(ctx, task.AccountID, task.Path, skip, func(item driver.SubtitleAttachment) error {
			rel, e := relative(item.Path)
			if e != nil {
				return e
			}
			if !eligible(rel) {
				return nil
			}
			if e = publishTreeOutput(output, confined, inv, rel, item.Data); e != nil {
				return e
			}
			attachmentSeen[rel] = true
			seen[rel] = true
			return nil
		})
		if e != nil {
			return e
		}
	}
	if e := syncTreeLocalMetadata(ctx, task, deps, entries, output); e != nil {
		return e
	}
	if normalizeMetadataSyncMode(deps.Settings.MetadataSyncMode) != "cloud_primary" {
		return nil
	}
	for rel, hash := range inv.Files {
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(rel)), ".")
		if _, ok := rules.metadataExts[ext]; !ok || attachmentSeen[rel] {
			continue
		}
		if _, e := safeOwnedPath(output, rel); e != nil {
			return e
		}
		body, e := confined.ReadFile(rel)
		if os.IsNotExist(e) {
			delete(inv.Files, rel)
			continue
		}
		if e != nil {
			return e
		}
		if treeFingerprint(body) != hash {
			continue
		}
		if e = confined.Remove(rel); e != nil {
			return e
		}
		delete(inv.Files, rel)
	}
	return saveTreeInventory(output, *inv)
}
