package strm

import (
	"context"
	"fmt"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func scanTreeTask(ctx context.Context, task *domain.StrmTask, deps ScanDeps, root string, rules scanRules) (ScanResult, error) {
	var result ScanResult
	err := WithTaskSync(ctx, task.ID, func() error {
		output := TaskOutputDir(root, TaskRelDir(task.GroupDir, task.OutputFolder))
		inv, err := loadTreeInventory(output, task.ID)
		if err != nil {
			return err
		}
		var entries []driver.TreeEntry
		startedUnix := time.Now().Unix()
		snap, err := deps.Files.ExportTree(ctx, task.AccountID, driver.TreeRequest{OperationID: fmt.Sprint(time.Now().UnixNano()), RootID: task.ParentID, RootPath: task.Path}, func(e driver.TreeEntry) error { entries = append(entries, e); return nil })
		if err != nil {
			return err
		}
		if !snap.Complete {
			return fmt.Errorf("incomplete tree snapshot; outputs retained")
		}
		if err = os.MkdirAll(root, 0755); err != nil {
			return err
		}
		sandbox, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		defer sandbox.Close()
		outputRel := TaskRelDir(task.GroupDir, task.OutputFolder)
		if err = sandbox.MkdirAll(outputRel, 0755); err != nil {
			return err
		}
		ownedRoot, err := sandbox.OpenRoot(outputRel)
		if err != nil {
			return err
		}
		defer ownedRoot.Close()
		seen := map[string]bool{}
		remoteScope := task.Path
		if snap.RootPath != "" {
			remoteScope = snap.RootPath
		}
		base := strings.TrimRight(remoteScope, "/")
		for _, entry := range entries {
			if err = ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir {
				continue
			}
			if base != "" && !strings.HasPrefix(entry.Path, base+"/") {
				return fmt.Errorf("tree leaf outside task root")
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(entry.Path, base), "/")
			if rel == "" {
				continue
			}
			for _, seg := range strings.Split(rel, "/") {
				if seg == ".." || seg == "." {
					return fmt.Errorf("invalid tree leaf path")
				}
			}
			if !task.Recursive && strings.Contains(rel, "/") {
				continue
			}
			result.ScannedCount++
			ext := strings.TrimPrefix(strings.ToLower(path.Ext(rel)), ".")
			if _, ok := rules.mediaExts[ext]; !ok {
				continue
			}
			skip := false
			for _, kw := range rules.excludeFiles {
				if strings.Contains(path.Base(rel), kw) {
					skip = true
				}
			}
			for _, kw := range rules.excludeDirs {
				if strings.Contains(path.Dir(rel), kw) {
					skip = true
				}
			}
			if skip {
				continue
			}
			dirs := []string{}
			if path.Dir(rel) != "." {
				dirs = strings.Split(path.Dir(rel), "/")
			}
			local := LocalRelPath("", dirs, path.Base(rel), deps.Settings.ISOFilenameEnabled)
			_, e := safeOwnedPath(output, local)
			if e != nil {
				return e
			}
			remoteRoot := remoteScope
			if remoteRoot == "" {
				remoteRoot = "/"
			}
			content := []byte(BuildPathPlayURL(deps.BaseURL, task.AccountID, remoteRoot, rel, path.Base(rel), deps.Token, deps.SignEnabled, deps.Secret))
			old, e := ownedRoot.ReadFile(local)
			if e == nil {
				fingerprint, owned := inv.Files[local]
				if !owned || treeFingerprint(old) != fingerprint {
					return fmt.Errorf("unmanaged or modified output conflict: %s", local)
				}
				seen[local] = true
				continue
			}
			if !os.IsNotExist(e) {
				return e
			}
			if e = ownedRoot.MkdirAll(filepath.Dir(local), 0755); e != nil {
				return e
			}
			// Record the intended fingerprint before publication. If interrupted,
			// retry can recognize only this exact task-owned content.
			inv.Files[local] = treeFingerprint(content)
			if e = saveTreeInventory(output, inv); e != nil {
				return e
			}
			f, e := ownedRoot.OpenFile(local, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if e != nil {
				return e
			}
			_, e = f.Write(content)
			closeErr := f.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
			inv.Files[local] = treeFingerprint(content)
			seen[local] = true
			result.GeneratedCount++
			// Persist ownership after each publication so later failures retain it.
			if e = saveTreeInventory(output, inv); e != nil {
				return e
			}
		}
		attachmentTask := *task
		attachmentTask.Path = remoteScope
		if err = syncTreeAttachments(ctx, &attachmentTask, deps, entries, output, &inv, seen); err != nil {
			return err
		}
		for rel, fingerprint := range inv.Files {
			if !strings.EqualFold(filepath.Ext(rel), ".strm") {
				continue
			}
			if seen[rel] {
				continue
			}
			_, e := safeOwnedPath(output, rel)
			if e != nil {
				return e
			}
			b, e := ownedRoot.ReadFile(rel)
			if os.IsNotExist(e) {
				delete(inv.Files, rel)
				continue
			}
			if e != nil {
				return e
			}
			if treeFingerprint(b) != fingerprint {
				continue
			}
			if e = ownedRoot.Remove(rel); e != nil {
				return e
			}
			delete(inv.Files, rel)
			result.RemovedCount++
		}
		if err = saveTreeInventory(output, inv); err != nil {
			return err
		}
		if deps.OnTreeComplete != nil {
			return deps.OnTreeComplete(ctx, task.ID, remoteScope, startedUnix)
		}
		return nil
	})
	return result, err
}
