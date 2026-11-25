package strm

import (
	"context"
	"fmt"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type taskLifeProvider struct {
	s         *Service
	accountID int64
}

func (p taskLifeProvider) ReadLifeEvents(ctx context.Context, r driver.LifeRequest) (driver.LifeBatch, error) {
	return p.s.files.ReadLifeEvents(ctx, p.accountID, r)
}

func (s *Service) treeBaselineComplete(ctx context.Context, id int64, rootPath string, startedUnix int64) error {
	if s.eventStore == nil {
		return nil
	}
	state, err := s.eventStore.LoadEventSync(ctx, id)
	if err != nil {
		return err
	}
	state.SnapshotComplete = true
	state.NeedsReconcile = false
	state.RootPath = rootPath
	if state.SinceUnix == 0 || startedUnix < state.SinceUnix {
		state.SinceUnix = startedUnix
	}
	if state.Receipts == nil {
		state.Receipts = map[string]int64{}
	}
	for id, ts := range state.PendingReconcile {
		if ts <= startedUnix {
			state.Receipts[id] = ts
			delete(state.PendingReconcile, id)
		} else {
			state.NeedsReconcile = true
		}
	}
	return s.eventStore.SaveEventSync(ctx, id, state)
}
func (s *Service) monitorEvents(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tasks, err := s.repo.List(ctx)
			if err != nil {
				continue
			}
			s.pollEventTasks(ctx, tasks, func(ctx context.Context, accountID int64, r driver.LifeRequest) (driver.LifeBatch, error) {
				return s.files.ReadLifeEvents(ctx, accountID, r)
			})

		}
	}
}

type fetchedLifeBatch struct {
	batch driver.LifeBatch
	err   error
}

func (p fetchedLifeBatch) ReadLifeEvents(_ context.Context, r driver.LifeRequest) (driver.LifeBatch, error) {
	out := p.batch
	out.Events = nil
	for _, event := range p.batch.Events {
		if event.UpdateUnix >= r.SinceUnix {
			out.Events = append(out.Events, event)
		}
	}
	return out, p.err
}

type directoryRoundKey struct{}
type directoryRoundResult struct {
	entries []driver.LifeEvent
	err     error
}
type directoryRoundCache map[string]directoryRoundResult

func (s *Service) expandDirectoryForRound(ctx context.Context, accountID int64, fileID string) ([]driver.LifeEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cache, _ := ctx.Value(directoryRoundKey{}).(directoryRoundCache)
	if result, ok := cache[fileID]; ok {
		return result.entries, result.err
	}
	entries, err := s.files.ExpandDirectoryEvents(ctx, accountID, fileID)
	if cache != nil {
		cache[fileID] = directoryRoundResult{entries, err}
	}
	return entries, err
}

// pollEventTasks shares one reference monitor fetch per account and isolates
// each task's output, receipts and durable cursor under its own task lock.
func (s *Service) pollEventTasks(ctx context.Context, tasks []*domain.StrmTask, read func(context.Context, int64, driver.LifeRequest) (driver.LifeBatch, error)) {
	type target struct {
		task  *domain.StrmTask
		state EventSyncState
	}
	groups := map[int64][]target{}
	for _, task := range tasks {
		if !task.MonitoringEnabled || task.Status != domain.StrmStatusActive {
			continue
		}
		state, err := s.eventStore.LoadEventSync(ctx, task.ID)
		if err != nil || !state.SnapshotComplete {
			continue
		}
		if state.NeedsReconcile {
			_, _ = s.RunTaskNow(ctx, task.ID, domain.StrmRunModeFull)
			continue
		}
		groups[task.AccountID] = append(groups[task.AccountID], target{task, state})
	}
	for accountID, targets := range groups {
		since := targets[0].state.SinceUnix
		for _, t := range targets {
			if t.state.SinceUnix < since {
				since = t.state.SinceUnix
			}
		}
		since -= 30
		if since < 0 {
			since = 0
		}
		batch, fetchErr := read(ctx, accountID, driver.LifeRequest{SinceUnix: since})
		roundCtx := context.WithValue(ctx, directoryRoundKey{}, directoryRoundCache{})
		mutated := false
		for _, target := range targets {
			taskCopy := *target.task
			taskCopy.Path = target.state.RootPath
			task := &taskCopy
			syncer := EventSync{Store: s.eventStore, Provider: fetchedLifeBatch{batch, fetchErr}, Apply: func(ctx context.Context, event driver.LifeEvent) error {
				changed, err := s.applyLifeEventTracked(ctx, task, event)
				mutated = mutated || changed
				return err
			}}
			if err := syncer.Poll(roundCtx, task.ID); err != nil && ctx.Err() == nil {
				s.log.Warn("STRM life synchronization paused", "task_id", task.ID, "error", err)
			}
		}
		if mutated && s.files != nil {
			s.files.NotifyExternalMutation(accountID)
		}
	}
}

// applyLifeEventTracked distinguishes output changes from ignored events. The
// account scheduler coalesces invalidation after all task applications.
func (s *Service) applyLifeEventTracked(ctx context.Context, task *domain.StrmTask, event driver.LifeEvent) (bool, error) {
	output := TaskOutputDir(s.strmDir, TaskRelDir(task.GroupDir, task.OutputFolder))
	before, err := loadTreeInventory(output, task.ID)
	if err != nil {
		return false, err
	}
	err = s.applyLifeEvent(ctx, task, event)
	after, readErr := loadTreeInventory(output, task.ID)
	if readErr != nil {
		if err != nil {
			return false, err
		}
		return false, readErr
	}
	return !maps.Equal(before.Files, after.Files), err
}

func (s *Service) applyLifeEvent(ctx context.Context, task *domain.StrmTask, event driver.LifeEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	output := TaskOutputDir(s.strmDir, TaskRelDir(task.GroupDir, task.OutputFolder))
	inv, err := loadTreeInventory(output, task.ID)
	if err != nil {
		return err
	}
	confined, err := os.OpenRoot(output)
	if err != nil {
		return err
	}
	defer confined.Close()
	settings := s.scanSettingsForTask(task)
	rules := newScanRules(task, settings)
	relative := func(remote string) (string, bool) {
		base := strings.TrimRight(task.Path, "/")
		if base != "" && !strings.HasPrefix(remote, base+"/") {
			return "", false
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(remote, base), "/")
		if rel == "" || !task.Recursive && strings.Contains(rel, "/") {
			return "", false
		}
		for _, seg := range strings.Split(rel, "/") {
			if seg == ".." || seg == "." {
				return "", false
			}
		}
		return rel, true
	}
	mediaLocal := func(rel string) string {
		var dirs []string
		if path.Dir(rel) != "." {
			dirs = strings.Split(path.Dir(rel), "/")
		}
		return LocalRelPath("", dirs, path.Base(rel), settings.ISOFilenameEnabled)
	}
	preserve := ""
	remove := func(remote string) error {
		rel, ok := relative(remote)
		if !ok {
			return nil
		}
		media := mediaLocal(rel)
		for local, hash := range inv.Files {
			if local == preserve {
				continue
			}
			if local != media && local != rel && !(event.IsDir && strings.HasPrefix(local, rel+"/")) {
				continue
			}
			if !strings.EqualFold(filepath.Ext(local), ".strm") && settings.MetadataSyncMode != "cloud_primary" {
				continue
			}
			if _, e := safeOwnedPath(output, local); e != nil {
				return e
			}
			body, e := confined.ReadFile(local)
			if os.IsNotExist(e) {
				delete(inv.Files, local)
				continue
			}
			if e != nil {
				return e
			}
			if treeFingerprint(body) != hash {
				continue
			}
			if e = confined.Remove(local); e != nil {
				return e
			}
			delete(inv.Files, local)
			delete(inv.Pending, local)
		}
		return saveTreeInventory(output, inv)
	}
	if event.Kind == "delete" {
		return remove(event.Path)
	}
	rel, inScope := relative(event.Path)
	if inScope {
		if event.IsDir {
			entries, e := s.expandDirectoryForRound(ctx, task.AccountID, event.FileID)
			if e != nil {
				return e
			}
			// Each child is durably published before the old directory is retired.
			for _, child := range entries {
				child.Kind = "create"
				child.OldPath = ""
				if e = s.applyLifeEvent(ctx, task, child); e != nil {
					return e
				}
			}
			if event.Kind == "rename" || event.Kind == "move" {
				inv, e = loadTreeInventory(output, task.ID)
				if e != nil {
					return e
				}
				if event.OldPath != event.Path {
					return remove(event.OldPath)
				}
			}
			return nil
		}
		excluded := false
		for _, keyword := range rules.excludeFiles {
			if strings.Contains(path.Base(rel), keyword) {
				excluded = true
			}
		}
		for _, keyword := range rules.excludeDirs {
			if strings.Contains(path.Dir(rel), keyword) {
				excluded = true
			}
		}
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(rel)), ".")
		if _, media := rules.mediaExts[ext]; media && !excluded {
			local := mediaLocal(rel)
			remoteRoot := task.Path
			if remoteRoot == "" {
				remoteRoot = "/"
			}
			content := []byte(BuildPathPlayURL(s.scanBaseURL(), task.AccountID, remoteRoot, rel, path.Base(rel), s.settings.String("strm_token"), s.settings.Bool("strm_signature_enabled"), s.secret))
			if err = publishTreeOutput(output, confined, &inv, local, content); err != nil {
				return err
			}
			preserve = local
		} else if _, attachment := rules.metadataExts[ext]; attachment && task.SyncMetadata && !excluded {
			if _, err = safeOwnedPath(output, rel); err != nil {
				return err
			}
			existing, e := confined.ReadFile(rel)
			if e == nil {
				hash, owned := inv.Files[rel]
				if !owned || treeFingerprint(existing) != hash {
					return fmt.Errorf("unmanaged attachment output conflict")
				}
			}
			if e != nil {
				if !os.IsNotExist(e) {
					return e
				}
				var body []byte
				if event.Pickcode != "" {
					body, e = s.files.DownloadAttachmentPickcode(ctx, task.AccountID, event.Pickcode)
				} else {
					body, e = s.files.DownloadAttachment(ctx, task.AccountID, event.FileID)
				}
				if e != nil {
					return e
				}
				if e = publishTreeOutput(output, confined, &inv, rel, body); e != nil {
					return e
				}
			}
			preserve = rel
		}
	}
	if event.Kind == "rename" || event.Kind == "move" {
		return remove(event.OldPath)
	}
	return nil
}
